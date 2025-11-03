package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/care/orion/internal/config"
	"github.com/care/orion/internal/control"
	"github.com/care/orion/internal/emitter"
	"github.com/care/orion/internal/framebus"
	"github.com/care/orion/internal/roiprocessor"
	"github.com/care/orion/internal/stream"
	"github.com/care/orion/internal/types"
	"github.com/care/orion/internal/worker"
)

// Orion is the main service orchestrator
type Orion struct {
	cfg *config.Config

	// Core components
	stream         StreamProvider
	frameBus       *framebus.Bus
	roiProcessor   *roiprocessor.Processor
	workers        []types.InferenceWorker
	emitter        *emitter.MQTTEmitter
	controlHandler *control.Handler

	// Lifecycle management
	started   time.Time
	mu        sync.RWMutex
	wg        sync.WaitGroup
	isRunning bool
	isPaused  bool
	runCtx    context.Context    // Run context for component restarts (stream, etc.)
	cancelCtx context.CancelFunc // For MQTT shutdown command
}

// NewOrion creates a new Orion service instance
func NewOrion(configPath string) (*Orion, error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	slog.Info("configuration loaded",
		"instance_id", cfg.InstanceID,
		"room_id", cfg.RoomID,
	)

	// Initialize ROI processor with thresholds from config
	roiProc := roiprocessor.NewProcessor(roiprocessor.Config{
		Threshold320: cfg.ROIs.Threshold320,
		Threshold640: cfg.ROIs.Threshold640,
	})

	o := &Orion{
		cfg:          cfg,
		frameBus:     framebus.New(),
		roiProcessor: roiProc,
		workers:      make([]types.InferenceWorker, 0),
		emitter:      emitter.NewMQTTEmitter(cfg),
	}

	// Initialize workers
	if err := o.initializeWorkers(); err != nil {
		return nil, fmt.Errorf("failed to initialize workers: %w", err)
	}

	return o, nil
}

// initializeWorkers creates and registers inference workers
func (o *Orion) initializeWorkers() error {
	// Person Detector: Python ONNX worker
	if o.cfg.Models.PersonDetector == nil || o.cfg.Models.PersonDetector.ModelPath == "" {
		return fmt.Errorf("person detector model not configured (models.person_detector.model_path required)")
	}

	pythonDetector, err := worker.NewPythonPersonDetector(worker.PythonPersonDetectorConfig{
		WorkerID:     "person-detector",
		ModelPath:    o.cfg.Models.PersonDetector.ModelPath,
		ModelPath320: o.cfg.Models.PersonDetector.ModelPath320,
		Confidence:   o.cfg.Models.PersonDetector.Confidence,
		InstanceID:   o.cfg.InstanceID,
		RoomID:       o.cfg.RoomID,
	})
	if err != nil {
		return fmt.Errorf("failed to create python person detector: %w", err)
	}

	o.frameBus.Register(pythonDetector)
	o.workers = append(o.workers, pythonDetector)

	slog.Info("python person detector configured",
		"model", o.cfg.Models.PersonDetector.ModelPath,
		"confidence", o.cfg.Models.PersonDetector.Confidence,
	)

	slog.Info("workers initialized", "count", len(o.workers))

	return nil
}

// Run starts the Orion service and blocks until context is cancelled
func (o *Orion) Run(ctx context.Context) error {
	o.mu.Lock()
	if o.isRunning {
		o.mu.Unlock()
		return fmt.Errorf("service is already running")
	}
	o.isRunning = true
	o.started = time.Now()
	o.mu.Unlock()

	// Create cancellable context for MQTT shutdown command
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	o.mu.Lock()
	o.runCtx = ctx       // Store for component restarts (stream hot-reload)
	o.cancelCtx = cancel // Store for MQTT shutdown command
	o.mu.Unlock()

	slog.Info("orion service starting",
		"instance_id", o.cfg.InstanceID,
	)

	// Initialize stream (RTSP or Mock for testing)
	width, height := parseResolution(o.cfg.Stream.Resolution)

	// Use RTSP if configured, otherwise fallback to mock
	if o.cfg.Camera.RTSPURL != "" {
		// TODO: Fix ProbeRTSPStream - currently disabled due to mainloop issues
		// For now, skip probing and use config values directly
		slog.Info("skipping rtsp probe (disabled), using config values")

		if false { // Disabled probe
			metadata, err := stream.ProbeRTSPStream(o.cfg.Camera.RTSPURL, 10*time.Second)
			if err != nil {
				slog.Warn("failed to probe rtsp stream, using config values",
					"error", err,
					"url", o.cfg.Camera.RTSPURL,
				)
			} else {
			// Adjust config based on detected metadata
			processInterval := 10 // Default
			if o.cfg.Models.PersonDetector != nil {
				processInterval = o.cfg.Models.PersonDetector.ProcessInterval
			}

			adjustedWidth, adjustedHeight, adjustedInterval, warnings := stream.AdjustConfigFromMetadata(
				metadata,
				width,
				height,
				processInterval,
			)

			// Log warnings
			for _, warning := range warnings {
				slog.Warn(warning)
			}

			// Apply adjustments
			width, height = adjustedWidth, adjustedHeight

			// Update worker config with adjusted interval
			if o.cfg.Models.PersonDetector != nil && adjustedInterval != processInterval {
				slog.Info("adjusting process_interval based on stream FPS",
					"old", processInterval,
					"new", adjustedInterval,
					"stream_fps", metadata.FPS,
				)
				o.cfg.Models.PersonDetector.ProcessInterval = adjustedInterval
			}

				slog.Info("stream metadata detected",
					"detected_resolution", fmt.Sprintf("%dx%d", metadata.Width, metadata.Height),
					"detected_fps", metadata.FPS,
					"using_resolution", fmt.Sprintf("%dx%d", width, height),
				)
			}
		} // End of disabled probe block

		rtspStream, err := stream.NewRTSPStream(stream.RTSPConfig{
			RTSPURL:      o.cfg.Camera.RTSPURL,
			Width:        width,
			Height:       height,
			FPS:          o.cfg.Stream.FPS,
			SourceStream: "LQ",
		})
		if err != nil {
			return fmt.Errorf("failed to create rtsp stream: %w", err)
		}
		o.stream = rtspStream

		slog.Info("using rtsp stream", "url", o.cfg.Camera.RTSPURL)
	} else {
		o.stream = stream.NewMockStream(width, height, o.cfg.Stream.FPS, "LQ")
		slog.Info("using mock stream (no rtsp_url configured)")
	}

	// Start stream
	if err := o.stream.Start(ctx); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	// Warm-up phase: consume frames without inference to measure real FPS
	warmupDuration := time.Duration(o.cfg.Stream.WarmupDurationS) * time.Second
	if warmupDuration == 0 {
		warmupDuration = 5 * time.Second // Default 5s
	}

	// Convert frame channel to interface{} channel for warm-up
	frameChan := make(chan interface{}, 10)
	go func() {
		for frame := range o.stream.Frames() {
			select {
			case frameChan <- frame:
			case <-ctx.Done():
				close(frameChan)
				return
			}
		}
		close(frameChan)
	}()

	warmupStats, err := stream.WarmupStream(ctx, frameChan, warmupDuration)
	if err != nil {
		slog.Warn("stream warm-up failed, continuing without FPS stats",
			"error", err,
		)
	} else {
		// Adjust worker config based on warm-up stats
		if o.cfg.Models.PersonDetector != nil {
			maxRate := o.cfg.Models.PersonDetector.MaxInferenceRateHz
			if maxRate == 0 {
				maxRate = 1.0 // Default to 1 Hz
			}

			// Calculate optimal inference rate (never exceeds max_rate)
			optimalRate := stream.CalculateOptimalInferenceRate(warmupStats, maxRate)

			// Calculate process_interval from stream FPS and inference rate
			processInterval := stream.CalculateProcessInterval(warmupStats.FPSMean, optimalRate)

			// Update config
			o.cfg.Models.PersonDetector.ProcessInterval = processInterval

			slog.Info("inference rate configured from warm-up",
				"stream_fps_mean", fmt.Sprintf("%.2f", warmupStats.FPSMean),
				"max_rate_hz", maxRate,
				"optimal_rate_hz", fmt.Sprintf("%.2f", optimalRate),
				"process_interval", processInterval,
			)
		}
	}

	// Connect MQTT emitter
	if err := o.emitter.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect mqtt: %w", err)
	}

	// Setup control plane handler
	o.controlHandler = control.NewHandler(o.cfg, o.emitter.Client, control.CommandCallbacks{
		OnGetStatus:          o.getStatus,
		OnPause:              o.pauseInference,
		OnResume:             o.resumeInference,
		OnUpdateConfig:       o.updateConfig,
		OnShutdown:           o.shutdownViaControl,
		OnSetInferenceRate:   o.setInferenceRate,
		OnSetModelSize:       o.setModelSize,
		OnSetAttentionROIs:   o.setAttentionROIs,
		OnClearAttentionROIs: o.clearAttentionROIs,
		OnGetAttentionROIs:   o.getAttentionROIs,
		// Auto-focus strategy commands
		OnSetAutoFocusStrategy:  o.setAutoFocusStrategy,
		OnGetAutoFocusStrategy:  o.getAutoFocusStrategy,
		OnEnableAutoFocus:       o.enableAutoFocus,
		OnDisableAutoFocus:      o.disableAutoFocus,
		OnClearAutoFocusHistory: o.clearAutoFocusHistory,
		OnGetAutoFocusStats:     o.getAutoFocusStats,
	})

	if err := o.controlHandler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	// Start workers
	if err := o.frameBus.Start(ctx); err != nil {
		return fmt.Errorf("failed to start workers: %w", err)
	}

	// Start frame consumer (distributes to FrameBus)
	o.wg.Add(1)
	go o.consumeFrames(ctx)

	// Start inference consumer (logs inferences from workers)
	o.wg.Add(1)
	go o.consumeInferences(ctx)

	// Start periodic stats logging
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		o.frameBus.StartStatsLogger(ctx, 10*time.Second)
	}()

	// Start worker health watchdog (auto-recovery)
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		o.watchWorkers(ctx)
	}()

	slog.Info("orion service running",
		"workers", len(o.workers),
		"watchdog_enabled", true,
	)

	// Wait for context cancellation
	<-ctx.Done()

	slog.Info("orion service run loop exiting")
	return nil
}

// Shutdown performs graceful shutdown of all components
func (o *Orion) Shutdown(ctx context.Context) error {
	o.mu.Lock()
	if !o.isRunning {
		o.mu.Unlock()
		return nil
	}
	o.mu.Unlock()

	slog.Info("shutting down orion service")

	// Shutdown sequence (order is important!):
	// 1. Stop workers FIRST (they depend on stream frames)
	if o.frameBus != nil {
		slog.Info("stopping workers and framebus")
		if err := o.frameBus.Stop(); err != nil {
			slog.Error("failed to stop framebus", "error", err)
		}
	}

	// 2. Stop stream (no more frames needed)
	if o.stream != nil {
		slog.Info("stopping stream")
		if err := o.stream.Stop(); err != nil {
			slog.Error("failed to stop stream", "error", err)
		}
	}

	// 3. Stop control plane
	if o.controlHandler != nil {
		slog.Info("stopping control handler")
		if err := o.controlHandler.Stop(); err != nil {
			slog.Error("failed to stop control handler", "error", err)
		}
	}

	// 4. Wait for goroutines to finish (without holding the lock)
	slog.Info("waiting for goroutines to finish")
	o.wg.Wait()
	slog.Info("all goroutines finished")

	// 5. Disconnect MQTT
	if o.emitter != nil {
		if err := o.emitter.Disconnect(); err != nil {
			slog.Error("failed to disconnect mqtt", "error", err)
		}
	}

	o.mu.Lock()
	uptime := time.Since(o.started)
	o.isRunning = false
	o.mu.Unlock()

	slog.Info("orion service shutdown complete",
		"uptime", uptime,
	)

	return nil
}

// watchWorkers monitors worker health and attempts auto-recovery
func (o *Orion) watchWorkers(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.mu.RLock()
			workers := o.workers
			inferenceRate := o.cfg.Models.PersonDetector.MaxInferenceRateHz
			o.mu.RUnlock()

			// Calculate adaptive timeout based on inference rate
			// Formula: max(30s, 3 × inference_period)
			// This gives the worker 3 full cycles before declaring it hung
			minTimeout := 30 * time.Second
			if inferenceRate > 0 {
				inferencePeriod := time.Duration(float64(time.Second) / inferenceRate)
				adaptiveTimeout := 3 * inferencePeriod
				if adaptiveTimeout > minTimeout {
					minTimeout = adaptiveTimeout
				}
			}

			for _, worker := range workers {
				metrics := worker.Metrics()

				// Check if worker is silent (no activity for > adaptive timeout)
				if !metrics.LastSeenAt.IsZero() && time.Since(metrics.LastSeenAt) > minTimeout {
					slog.Warn("worker appears hung, attempting restart",
						"worker_id", worker.ID(),
						"last_seen_ago_s", int(time.Since(metrics.LastSeenAt).Seconds()),
						"frames_processed", metrics.FramesProcessed,
						"watchdog_timeout_s", int(minTimeout.Seconds()),
					)

					// Attempt restart (ONCE - KISS approach)
					if err := worker.Stop(); err != nil {
						slog.Error("failed to stop hung worker",
							"worker_id", worker.ID(),
							"error", err,
							"action", "manual intervention required")
						continue
					}

					if err := worker.Start(ctx); err != nil {
						slog.Error("failed to restart worker",
							"worker_id", worker.ID(),
							"error", err,
							"action", "manual intervention required")
						continue
					}

					slog.Info("worker restarted successfully",
						"worker_id", worker.ID(),
					)
				}
			}
		}
	}
}

// GetStatus returns the current status of the service
func (o *Orion) GetStatus() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return map[string]interface{}{
		"instance_id": o.cfg.InstanceID,
		"room_id":     o.cfg.RoomID,
		"uptime_s":    time.Since(o.started).Seconds(),
		"running":     o.isRunning,
		// TODO: Add component statuses
	}
}

// ShutdownTimeout returns the configured graceful shutdown timeout
// Returns default of 5 seconds if not configured
func (o *Orion) ShutdownTimeout() time.Duration {
	timeout := time.Duration(o.cfg.ShutdownTimeoutS) * time.Second
	if timeout == 0 {
		return 5 * time.Second // Default
	}
	return timeout
}

// parseResolution converts resolution string to width/height
func parseResolution(res string) (width, height int) {
	switch res {
	case "320p":
		return 426, 320
	case "480p":
		return 640, 480
	case "512p":
		return 640, 512
	case "720p":
		return 1280, 720
	case "1080p":
		return 1920, 1080
	default:
		// Default to 640x480
		slog.Warn("unknown resolution, using default", "resolution", res, "default", "640x480")
		return 640, 480
	}
}
