package streamcapture

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/stream-capture/internal/rtsp"
	"github.com/e7canasta/orion-care-sensor/modules/stream-capture/internal/warmup"
	"github.com/tinyzimmer/go-gst/gst"
	"github.com/tinyzimmer/go-gst/gst/app"
)

// RTSPStream implements StreamProvider using GStreamer for RTSP streaming
type RTSPStream struct {
	// Configuration
	rtspURL      string
	width        int
	height       int
	targetFPS    float64
	sourceStream string
	acceleration HardwareAccel

	// GStreamer pipeline elements (for hot-reload)
	elements *rtsp.PipelineElements

	// Frame output
	frames chan Frame
	mu     sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics (atomic for thread-safety)
	frameCount    uint64
	framesDropped uint64 // Counter for dropped frames
	bytesRead     uint64
	reconnects    uint32
	started       time.Time
	lastFrameAt   time.Time

	// Error telemetry (atomic for thread-safety)
	errorsNetwork uint64 // Network-related errors (connection, timeout)
	errorsCodec   uint64 // Codec/stream errors (decode failures)
	errorsAuth    uint64 // Authentication/authorization errors
	errorsUnknown uint64 // Unclassified errors

	// VAAPI telemetry
	usingVAAPI      bool                               // True if VAAPI pipeline is active
	decodeLatencies atomic.Pointer[rtsp.LatencyWindow] // Lock-free latency tracking

	// Reconnection state
	reconnectState *rtsp.ReconnectState
	reconnectCfg   rtsp.ReconnectConfig

	// Shutdown protection (atomic flag to prevent double-close panic)
	framesClosed atomic.Bool
}

// NewRTSPStream creates a new RTSP stream with fail-fast validation
//
// Validates configuration at construction time (fail-fast principle):
//   - RTSP URL must not be empty
//   - Target FPS must be between 0.1 and 30.0
//   - Resolution must be valid (512p, 720p, 1080p)
//
// Returns an error if validation fails or GStreamer is not available.
func NewRTSPStream(cfg RTSPConfig) (*RTSPStream, error) {
	// Fail-fast validation: RTSP URL
	if cfg.URL == "" {
		return nil, fmt.Errorf("stream-capture: RTSP URL is required")
	}

	// Fail-fast validation: Target FPS
	if cfg.TargetFPS < 0.1 || cfg.TargetFPS > 30 {
		return nil, fmt.Errorf(
			"stream-capture: invalid FPS %.2f (must be 0.1-30)",
			cfg.TargetFPS,
		)
	}

	// Fail-fast validation: Resolution
	width, height := cfg.Resolution.Dimensions()
	if width == 0 || height == 0 {
		return nil, fmt.Errorf(
			"stream-capture: invalid resolution %v",
			cfg.Resolution,
		)
	}

	// Fail-fast validation: GStreamer availability
	if err := checkGStreamerAvailable(); err != nil {
		return nil, fmt.Errorf("stream-capture: GStreamer not available: %w", err)
	}

	// Fail-fast validation: VAAPI availability (if forced)
	if cfg.Acceleration == AccelVAAPI {
		if err := checkVAAPIAvailable(); err != nil {
			return nil, fmt.Errorf("stream-capture: VAAPI not available: %w", err)
		}
	}

	// Build reconnect config from user settings (or defaults)
	reconnectCfg := rtsp.DefaultReconnectConfig()
	if cfg.MaxReconnectAttempts > 0 {
		reconnectCfg.MaxRetries = cfg.MaxReconnectAttempts
	}
	if cfg.ReconnectInitialDelay > 0 {
		reconnectCfg.RetryDelay = cfg.ReconnectInitialDelay
	}
	if cfg.ReconnectMaxDelay > 0 {
		reconnectCfg.MaxRetryDelay = cfg.ReconnectMaxDelay
	}

	s := &RTSPStream{
		rtspURL:      cfg.URL,
		width:        width,
		height:       height,
		targetFPS:    cfg.TargetFPS,
		sourceStream: cfg.SourceStream,
		acceleration: cfg.Acceleration,
		frames:       make(chan Frame, 10), // Buffer 10 frames
		reconnectCfg: reconnectCfg,
		reconnectState: &rtsp.ReconnectState{
			Reconnects: new(uint32),
		},
	}

	slog.Info("stream-capture: RTSP stream created",
		"url", cfg.URL,
		"resolution", fmt.Sprintf("%dx%d", width, height),
		"target_fps", cfg.TargetFPS,
		"source_stream", cfg.SourceStream,
		"acceleration", cfg.Acceleration.String(),
	)

	return s, nil
}

// Start initializes the stream and returns a read-only channel of frames
//
// This method:
//  1. Creates GStreamer pipeline
//  2. Starts pipeline in Playing state
//  3. Launches background goroutines for frame processing and monitoring
//  4. Returns frame channel immediately (non-blocking)
//
// IMPORTANT: This method returns immediately. Frames will start arriving
// asynchronously once the pipeline reaches PLAYING state (~3 seconds).
//
// For production use, call WarmupStream() separately after Start() to
// measure FPS stability before processing frames.
func (s *RTSPStream) Start(ctx context.Context) (<-chan Frame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return nil, fmt.Errorf("stream-capture: stream already started")
	}

	// Create cancellable context
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = time.Now()

	slog.Info("stream-capture: starting RTSP stream",
		"url", s.rtspURL,
		"resolution", fmt.Sprintf("%dx%d", s.width, s.height),
		"target_fps", s.targetFPS,
	)

	// Create GStreamer pipeline
	pipelineCfg := rtsp.PipelineConfig{
		RTSPURL:      s.rtspURL,
		Width:        s.width,
		Height:       s.height,
		TargetFPS:    s.targetFPS,
		Acceleration: int(s.acceleration), // Pass acceleration mode
	}

	elements, err := rtsp.CreatePipeline(pipelineCfg)
	if err != nil {
		return nil, fmt.Errorf("stream-capture: failed to create pipeline: %w", err)
	}
	s.elements = elements

	// Set VAAPI flag from pipeline detection
	s.usingVAAPI = elements.UsingVAAPI

	// Initialize latency tracking if VAAPI is active
	if s.usingVAAPI {
		initialWindow := &rtsp.LatencyWindow{}
		s.decodeLatencies.Store(initialWindow)
		slog.Info("stream-capture: VAAPI hardware acceleration active, latency tracking enabled")
	}

	// Create internal frame channel for callbacks
	// (avoids import cycle by using rtsp.Frame instead of streamcapture.Frame)
	internalFrames := make(chan rtsp.Frame, 10)

	// Set up callbacks
	callbackCtx := &rtsp.CallbackContext{
		FrameChan:     internalFrames,
		FrameCounter:  &s.frameCount,
		BytesRead:     &s.bytesRead,
		FramesDropped: &s.framesDropped,
		Width:         s.width,
		Height:        s.height,
		SourceStream:  s.sourceStream,
	}

	// Enable latency tracking if VAAPI is active
	if s.usingVAAPI {
		callbackCtx.DecodeLatencies = &s.decodeLatencies
	}

	// Launch goroutine to convert internal frames to public frames
	// Capture ctx locally to avoid nil dereference during shutdown
	localCtx := s.ctx
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(internalFrames) // Ensure internal channel is closed on exit

		for internalFrame := range internalFrames {
			// Convert rtsp.Frame to streamcapture.Frame
			publicFrame := Frame{
				Seq:          internalFrame.Seq,
				Timestamp:    internalFrame.Timestamp,
				Width:        internalFrame.Width,
				Height:       internalFrame.Height,
				Data:         internalFrame.Data,
				SourceStream: internalFrame.SourceStream,
				TraceID:      internalFrame.TraceID,
			}

			// Update lastFrameAt timestamp (for latency metric)
			s.mu.Lock()
			s.lastFrameAt = time.Now()
			s.mu.Unlock()

			// Send to public channel (non-blocking with drop tracking)
			select {
			case s.frames <- publicFrame:
				// Frame sent successfully
			case <-localCtx.Done():
				return
			default:
				// Channel full - drop frame and track metric
				atomic.AddUint64(&s.framesDropped, 1)
				slog.Debug("stream-capture: dropping frame, channel full",
					"seq", publicFrame.Seq,
					"trace_id", publicFrame.TraceID,
				)
			}
		}
	}()

	s.elements.AppSink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			return rtsp.OnNewSample(sink, callbackCtx)
		},
	})

	// Connect pad-added signal for rtspsrc dynamic pads
	// We need to find the rtph264depay element to link to
	var depayElement *gst.Element
	pipelineElements, _ := s.elements.Pipeline.GetElements()
	for _, elem := range pipelineElements {
		if elem.GetFactory() != nil && elem.GetFactory().GetName() == "rtph264depay" {
			depayElement = elem
			break
		}
	}

	if depayElement != nil {
		s.elements.RTSPSrc.Connect("pad-added", func(self *gst.Element, srcPad *gst.Pad) {
			rtsp.OnPadAdded(self, srcPad, depayElement)
		})
	} else {
		slog.Warn("stream-capture: rtph264depay element not found, pad-added callback not connected")
	}

	// Start pipeline
	if err := s.elements.Pipeline.SetState(gst.StatePlaying); err != nil {
		return nil, fmt.Errorf("stream-capture: failed to start pipeline: %w", err)
	}

	// Wait for pipeline to reach PLAYING state
	bus := s.elements.Pipeline.GetPipelineBus()
	msg := bus.TimedPop(5 * time.Second)
	if msg != nil && msg.Type() == gst.MessageStateChanged {
		_, newState := msg.ParseStateChanged()
		if newState == gst.StatePlaying {
			slog.Info("stream-capture: pipeline reached PLAYING state")
		}
	}

	// Launch background goroutine for pipeline bus monitoring
	s.wg.Add(1)
	go s.runPipeline()

	slog.Info("stream-capture: RTSP stream started",
		"url", s.rtspURL,
		"note", "frames will arrive asynchronously once pipeline reaches PLAYING state",
	)

	return s.frames, nil
}

// runPipeline monitors the GStreamer pipeline bus for messages with reconnection
//
// This goroutine runs in the background and:
//  1. Monitors pipeline bus for errors
//  2. On error: triggers reconnection with exponential backoff
//  3. On success: resets retry counter and continues
//
// Runs until context is cancelled or max retries exceeded.
func (s *RTSPStream) runPipeline() {
	defer s.wg.Done()

	// Use RunWithReconnect for automatic reconnection with exponential backoff
	connectFn := func(ctx context.Context) error {
		return s.monitorPipeline(ctx)
	}

	err := rtsp.RunWithReconnect(
		s.ctx,
		connectFn,
		s.reconnectCfg,
		s.reconnectState,
	)

	if err != nil {
		slog.Error("stream-capture: pipeline stopped after reconnection failure",
			"error", err,
			"rtsp_url", s.rtspURL,
			"resolution", fmt.Sprintf("%dx%d", s.width, s.height),
			"uptime", time.Since(s.started),
			"frames_processed", atomic.LoadUint64(&s.frameCount),
			"reconnects", atomic.LoadUint32(s.reconnectState.Reconnects),
		)
	}
}

// monitorPipeline monitors the GStreamer pipeline bus for messages
//
// Returns an error if the pipeline encounters an error (triggers reconnection).
// Returns nil if context is cancelled (graceful shutdown).
func (s *RTSPStream) monitorPipeline(ctx context.Context) error {
	if s.elements == nil || s.elements.Pipeline == nil {
		return fmt.Errorf("pipeline not initialized")
	}

	// Prepare error counters
	errorCounters := &rtsp.ErrorCounters{
		Network: &s.errorsNetwork,
		Codec:   &s.errorsCodec,
		Auth:    &s.errorsAuth,
		Unknown: &s.errorsUnknown,
	}

	// Prepare metrics
	metrics := &rtsp.MonitorMetrics{
		RTSPURL:        s.rtspURL,
		Resolution:     fmt.Sprintf("%dx%d", s.width, s.height),
		FrameCount:     &s.frameCount,
		ReconnectCount: s.reconnectState.Reconnects,
		StartedAt:      s.started,
	}

	// Delegate to monitor module
	return rtsp.MonitorPipelineBus(
		ctx,
		s.elements.Pipeline,
		errorCounters,
		s.reconnectState,
		metrics,
	)
}

// Stop gracefully shuts down the stream
//
// This method:
//  1. Cancels context to signal shutdown
//  2. Waits for goroutines to finish (timeout 3s)
//  3. Stops GStreamer pipeline
//  4. Closes frame channel
//  5. Resets state for potential restart
//
// Idempotent - safe to call multiple times.
func (s *RTSPStream) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel == nil {
		slog.Debug("stream-capture: stream not started, nothing to stop")
		return nil
	}

	slog.Info("stream-capture: stopping RTSP stream")

	// Cancel context to signal shutdown
	s.cancel()

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Debug("stream-capture: goroutines stopped cleanly")
	case <-time.After(3 * time.Second):
		slog.Warn("stream-capture: stop timeout exceeded, some goroutines may still be running")
	}

	// Destroy GStreamer pipeline
	if s.elements != nil {
		if err := rtsp.DestroyPipeline(s.elements); err != nil {
			slog.Error("stream-capture: failed to destroy pipeline", "error", err)
		}
		s.elements = nil
	}

	// Close frame channel (protected against double-close)
	// Use atomic CompareAndSwap to ensure channel is closed exactly once
	if s.framesClosed.CompareAndSwap(false, true) {
		close(s.frames)
		slog.Debug("stream-capture: frame channel closed")
	} else {
		slog.Debug("stream-capture: frame channel already closed, skipping")
	}

	// Log statistics
	frameCount := atomic.LoadUint64(&s.frameCount)
	reconnects := atomic.LoadUint32(s.reconnectState.Reconnects)
	uptime := time.Since(s.started)

	slog.Info("stream-capture: RTSP stream stopped",
		"frames_captured", frameCount,
		"reconnects", reconnects,
		"uptime", uptime,
	)

	// Reset state for potential restart
	s.cancel = nil
	s.ctx = nil
	s.frames = make(chan Frame, 10)
	s.framesClosed.Store(false) // Reset flag for restart

	return nil
}

// Stats returns current stream statistics
//
// Thread-safe - uses atomic operations for counters.
func (s *RTSPStream) Stats() StreamStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	frameCount := atomic.LoadUint64(&s.frameCount)
	framesDropped := atomic.LoadUint64(&s.framesDropped)
	bytesRead := atomic.LoadUint64(&s.bytesRead)
	reconnects := atomic.LoadUint32(s.reconnectState.Reconnects)

	// Calculate real FPS
	var fpsReal float64
	if !s.started.IsZero() {
		uptime := time.Since(s.started).Seconds()
		if uptime > 0 {
			fpsReal = float64(frameCount) / uptime
		}
	}

	// Calculate drop rate
	var dropRate float64
	totalAttempts := frameCount + framesDropped
	if totalAttempts > 0 {
		dropRate = (float64(framesDropped) / float64(totalAttempts)) * 100.0
	}

	// Calculate latency (time since last frame)
	var latencyMS int64
	if !s.lastFrameAt.IsZero() {
		latencyMS = time.Since(s.lastFrameAt).Milliseconds()
	}

	// Determine connection status
	isConnected := s.elements != nil && s.cancel != nil

	// Load error counters
	errorsNetwork := atomic.LoadUint64(&s.errorsNetwork)
	errorsCodec := atomic.LoadUint64(&s.errorsCodec)
	errorsAuth := atomic.LoadUint64(&s.errorsAuth)
	errorsUnknown := atomic.LoadUint64(&s.errorsUnknown)

	// Calculate VAAPI decode latency stats (lock-free read)
	var decodeMean, decodeP95, decodeMax float64
	if s.usingVAAPI {
		window := s.decodeLatencies.Load()
		if window != nil {
			decodeMean, decodeP95, decodeMax = window.GetStats()
		}
	}

	return StreamStats{
		FrameCount:    frameCount,
		FramesDropped: framesDropped,
		DropRate:      dropRate,
		FPSTarget:     s.targetFPS,
		FPSReal:       fpsReal,
		LatencyMS:     latencyMS,
		SourceStream:  s.sourceStream,
		Resolution:    fmt.Sprintf("%dx%d", s.width, s.height),
		Reconnects:    reconnects,
		BytesRead:     bytesRead,
		IsConnected:   isConnected,
		ErrorsNetwork:       errorsNetwork,
		ErrorsCodec:         errorsCodec,
		ErrorsAuth:          errorsAuth,
		ErrorsUnknown:       errorsUnknown,
		DecodeLatencyMeanMS: decodeMean,
		DecodeLatencyP95MS:  decodeP95,
		DecodeLatencyMaxMS:  decodeMax,
		UsingVAAPI:          s.usingVAAPI,
	}
}

// SetTargetFPS updates the target FPS dynamically without restarting the stream
//
// This triggers a hot-reload of the GStreamer capsfilter, causing approximately
// 2 seconds of interruption. If the update fails, the previous FPS is restored.
func (s *RTSPStream) SetTargetFPS(fps float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate FPS range
	if fps < 0.1 || fps > 30 {
		return fmt.Errorf(
			"stream-capture: invalid FPS %.2f (must be 0.1-30)",
			fps,
		)
	}

	// Check if stream is running
	if s.elements == nil || s.elements.CapsFilter == nil {
		return fmt.Errorf("stream-capture: stream not running")
	}

	oldFPS := s.targetFPS

	slog.Info("stream-capture: updating target FPS",
		"old_fps", oldFPS,
		"new_fps", fps,
	)

	// Update capsfilter (hot-reload) with timeout protection
	// Hot-reload should complete in ~2 seconds, timeout at 5s for safety
	updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- rtsp.UpdateFramerateCaps(s.elements.CapsFilter, fps, s.width, s.height)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			// Explicit rollback to previous FPS
			slog.Warn("stream-capture: FPS update failed, attempting rollback",
				"error", err,
				"old_fps", oldFPS,
				"failed_fps", fps,
			)

			rollbackErr := rtsp.UpdateFramerateCaps(s.elements.CapsFilter, oldFPS, s.width, s.height)
			if rollbackErr != nil {
				slog.Error("stream-capture: rollback failed, pipeline may be in inconsistent state",
					"rollback_error", rollbackErr,
					"original_error", err,
				)
			}

			return fmt.Errorf("stream-capture: failed to update FPS: %w", err)
		}

	case <-updateCtx.Done():
		// Timeout exceeded - attempt rollback
		slog.Error("stream-capture: FPS update timeout (>5s), attempting rollback",
			"old_fps", oldFPS,
			"failed_fps", fps,
		)

		rollbackErr := rtsp.UpdateFramerateCaps(s.elements.CapsFilter, oldFPS, s.width, s.height)
		if rollbackErr != nil {
			slog.Error("stream-capture: rollback failed after timeout",
				"rollback_error", rollbackErr,
			)
		}

		return fmt.Errorf("stream-capture: SetTargetFPS timeout after 5 seconds")
	}

	// Update internal state
	s.targetFPS = fps

	slog.Info("stream-capture: target FPS updated successfully",
		"new_fps", fps,
	)

	return nil
}

// Warmup measures stream FPS stability over a specified duration
//
// This method should be called after Start() to measure the real FPS and
// verify stream stability before processing frames. It consumes frames from
// the stream for the specified duration and returns statistics.
//
// The method blocks for the entire duration while collecting statistics.
//
// Returns WarmupStats with FPS measurements, or an error if:
//   - Stream is not running
//   - Not enough frames received (< 2)
//   - Context is cancelled
//
// Example:
//
//	stream, _ := streamcapture.NewRTSPStream(cfg)
//	frameChan, _ := stream.Start(ctx)
//
//	stats, err := stream.Warmup(ctx, 5*time.Second)
//	if err != nil {
//	    log.Fatal("warmup failed:", err)
//	}
//	log.Printf("Stream stable: %v, FPS: %.2f", stats.IsStable, stats.FPSMean)
//
//	// Now consume frames normally
//	for frame := range frameChan {
//	    // Process frame...
//	}
func (s *RTSPStream) Warmup(ctx context.Context, duration time.Duration) (*WarmupStats, error) {
	s.mu.RLock()
	if s.cancel == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("stream-capture: stream not started")
	}
	s.mu.RUnlock()

	// Create adapter channel for warmup helper (converts Frame → warmup.Frame)
	// This avoids import cycle and allows warmup package to be independent
	warmupFrames := make(chan warmup.Frame, 10)

	// Launch goroutine to convert frames
	go func() {
		defer close(warmupFrames)

		// Create timeout context for adapter goroutine
		warmupCtx, cancel := context.WithTimeout(ctx, duration+1*time.Second) // +1s buffer
		defer cancel()

		for {
			select {
			case <-warmupCtx.Done():
				return
			case frame, ok := <-s.frames:
				if !ok {
					return
				}
				// Convert streamcapture.Frame → warmup.Frame (minimal subset)
				warmupFrame := warmup.Frame{
					Seq:       frame.Seq,
					Timestamp: frame.Timestamp,
				}
				select {
				case warmupFrames <- warmupFrame:
					// Sent successfully
				case <-warmupCtx.Done():
					return
				}
			}
		}
	}()

	// Delegate to warmup helper (includes fail-fast validation)
	internalStats, err := warmup.WarmupStream(ctx, warmupFrames, duration)
	if err != nil {
		// Wrap error with module prefix
		return nil, fmt.Errorf("stream-capture: warmup failed: %w", err)
	}

	// Convert internal stats to public stats (identical structure)
	return &WarmupStats{
		FramesReceived: internalStats.FramesReceived,
		Duration:       internalStats.Duration,
		FPSMean:        internalStats.FPSMean,
		FPSStdDev:      internalStats.FPSStdDev,
		FPSMin:         internalStats.FPSMin,
		FPSMax:         internalStats.FPSMax,
		IsStable:       internalStats.IsStable,
		JitterMean:     internalStats.JitterMean,
		JitterStdDev:   internalStats.JitterStdDev,
		JitterMax:      internalStats.JitterMax,
	}, nil
}

// checkGStreamerAvailable checks if GStreamer is available
//
// This is a fail-fast validation that runs at construction time.
func checkGStreamerAvailable() error {
	// Initialize GStreamer (safe to call multiple times)
	gst.Init(nil)

	// Try to create a simple element to verify GStreamer is working
	elem, err := gst.NewElement("fakesrc")
	if err != nil {
		return fmt.Errorf("GStreamer not available or not properly installed: %w", err)
	}

	// Clean up test element
	elem.SetState(gst.StateNull)

	return nil
}

// checkVAAPIAvailable checks if VAAPI hardware acceleration is available
//
// This is a fail-fast validation that runs at construction time when
// Acceleration is set to AccelVAAPI.
//
// Returns an error if VAAPI elements are not available (missing gstreamer1.0-vaapi
// package or incompatible hardware).
func checkVAAPIAvailable() error {
	// Initialize GStreamer (safe to call multiple times)
	gst.Init(nil)

	// Try to create vaapidecodebin element
	decoder, err := gst.NewElement("vaapidecodebin")
	if err != nil {
		return fmt.Errorf("vaapidecodebin not available (install gstreamer1.0-vaapi): %w", err)
	}
	decoder.SetState(gst.StateNull)

	// Try to create vaapipostproc element
	postproc, err := gst.NewElement("vaapipostproc")
	if err != nil {
		return fmt.Errorf("vaapipostproc not available (install gstreamer1.0-vaapi): %w", err)
	}
	postproc.SetState(gst.StateNull)

	slog.Debug("stream-capture: VAAPI hardware acceleration available")

	return nil
}
