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
	frameCount  uint64
	bytesRead   uint64
	reconnects  uint32
	started     time.Time
	lastFrameAt time.Time

	// Reconnection state
	reconnectState *rtsp.ReconnectState
	reconnectCfg   rtsp.ReconnectConfig
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
	if cfg.TargetFPS <= 0 || cfg.TargetFPS > 30 {
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

	s := &RTSPStream{
		rtspURL:      cfg.URL,
		width:        width,
		height:       height,
		targetFPS:    cfg.TargetFPS,
		sourceStream: cfg.SourceStream,
		frames:       make(chan Frame, 10), // Buffer 10 frames
		reconnectCfg: rtsp.DefaultReconnectConfig(),
		reconnectState: &rtsp.ReconnectState{
			Reconnects: new(uint32),
		},
	}

	slog.Info("stream-capture: RTSP stream created",
		"url", cfg.URL,
		"resolution", fmt.Sprintf("%dx%d", width, height),
		"target_fps", cfg.TargetFPS,
		"source_stream", cfg.SourceStream,
	)

	return s, nil
}

// Start initializes the stream and returns a read-only channel of frames
//
// This method:
//  1. Creates GStreamer pipeline
//  2. Starts pipeline in Playing state
//  3. Runs warm-up for 5 seconds to measure FPS stability
//  4. Launches background goroutine for frame processing
//  5. Returns frame channel (already stable)
//
// Blocks for approximately 5 seconds during warm-up.
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
		RTSPURL:   s.rtspURL,
		Width:     s.width,
		Height:    s.height,
		TargetFPS: s.targetFPS,
	}

	elements, err := rtsp.CreatePipeline(pipelineCfg)
	if err != nil {
		return nil, fmt.Errorf("stream-capture: failed to create pipeline: %w", err)
	}
	s.elements = elements

	// Create internal frame channel for callbacks
	// (avoids import cycle by using rtsp.Frame instead of streamcapture.Frame)
	internalFrames := make(chan rtsp.Frame, 10)

	// Set up callbacks
	callbackCtx := &rtsp.CallbackContext{
		FrameChan:    internalFrames,
		FrameCounter: &s.frameCount,
		BytesRead:    &s.bytesRead,
		Width:        s.width,
		Height:       s.height,
		SourceStream: s.sourceStream,
	}

	// Launch goroutine to convert internal frames to public frames
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

			// Send to public channel (non-blocking)
			select {
			case s.frames <- publicFrame:
			case <-s.ctx.Done():
				return
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

	// Warm-up: measure FPS stability (5 seconds)
	// Create a channel adapter to convert Frame to warmup.Frame
	warmupFrames := make(chan warmup.Frame, 10)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(warmupFrames)

		timeout := time.After(5 * time.Second)
		for {
			select {
			case <-timeout:
				return
			case <-s.ctx.Done():
				return
			case frame, ok := <-s.frames:
				if !ok {
					return
				}
				// Convert to warmup.Frame (only needs Seq and Timestamp)
				warmupFrames <- warmup.Frame{
					Seq:       frame.Seq,
					Timestamp: frame.Timestamp,
				}
			}
		}
	}()

	slog.Info("stream-capture: starting warm-up phase", "duration", "5s")
	warmupStats, err := warmup.WarmupStream(s.ctx, warmupFrames, 5*time.Second)
	if err != nil {
		s.Stop() // Clean up on warm-up failure
		return nil, fmt.Errorf("stream-capture: warm-up failed: %w", err)
	}

	// Log warm-up results
	if !warmupStats.IsStable {
		slog.Warn("stream-capture: stream FPS unstable",
			"fps_mean", warmupStats.FPSMean,
			"fps_stddev", warmupStats.FPSStdDev,
		)
	}

	slog.Info("stream-capture: RTSP stream started successfully",
		"warmup_frames", warmupStats.FramesReceived,
		"fps_mean", fmt.Sprintf("%.2f", warmupStats.FPSMean),
		"stable", warmupStats.IsStable,
	)

	return s.frames, nil
}

// runPipeline monitors the GStreamer pipeline bus for messages with reconnection
//
// This goroutine runs in the background and:
//   1. Monitors pipeline bus for errors
//   2. On error: triggers reconnection with exponential backoff
//   3. On success: resets retry counter and continues
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

	bus := s.elements.Pipeline.GetPipelineBus()

	for {
		select {
		case <-ctx.Done():
			slog.Debug("stream-capture: context cancelled, stopping pipeline monitor")
			return nil

		default:
			// Poll for messages with short timeout for responsive shutdown
			msg := bus.TimedPop(50 * time.Millisecond)
			if msg == nil {
				continue
			}

			switch msg.Type() {
			case gst.MessageEOS:
				slog.Info("stream-capture: end of stream received")
				return fmt.Errorf("end of stream")

			case gst.MessageError:
				gerr := msg.ParseError()
				slog.Error("stream-capture: pipeline error",
					"error", gerr.Error(),
					"debug", gerr.DebugString(),
				)
				// Return error to trigger reconnection
				return fmt.Errorf("pipeline error: %s", gerr.Error())

			case gst.MessageStateChanged:
				if msg.Source() == s.elements.Pipeline.GetName() {
					old, new := msg.ParseStateChanged()
					slog.Debug("stream-capture: pipeline state changed",
						"from", old,
						"to", new,
					)

					// Reset reconnection state when reaching PLAYING state
					if new == gst.StatePlaying {
						rtsp.ResetReconnectState(s.reconnectState)
						slog.Info("stream-capture: pipeline playing, reconnect state reset")
					}
				}
			}
		}
	}
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

	// Note: internal frame channel is closed by GStreamer cleanup
	// Public frame channel is closed here
	close(s.frames)

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

	return nil
}

// Stats returns current stream statistics
//
// Thread-safe - uses atomic operations for counters.
func (s *RTSPStream) Stats() StreamStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	frameCount := atomic.LoadUint64(&s.frameCount)
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

	// Calculate latency (time since last frame)
	var latencyMS int64
	if !s.lastFrameAt.IsZero() {
		latencyMS = time.Since(s.lastFrameAt).Milliseconds()
	}

	// Determine connection status
	isConnected := s.elements != nil && s.cancel != nil

	return StreamStats{
		FrameCount:   frameCount,
		FPSTarget:    s.targetFPS,
		FPSReal:      fpsReal,
		LatencyMS:    latencyMS,
		SourceStream: s.sourceStream,
		Resolution:   fmt.Sprintf("%dx%d", s.width, s.height),
		Reconnects:   reconnects,
		BytesRead:    bytesRead,
		IsConnected:  isConnected,
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
	if fps <= 0 || fps > 30 {
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

	// Update capsfilter (hot-reload)
	if err := rtsp.UpdateFramerateCaps(s.elements.CapsFilter, fps, s.width, s.height); err != nil {
		// Rollback on error
		slog.Error("stream-capture: failed to update FPS, rolling back",
			"error", err,
			"old_fps", oldFPS,
		)
		return fmt.Errorf("stream-capture: failed to update FPS: %w", err)
	}

	// Update internal state
	s.targetFPS = fps

	slog.Info("stream-capture: target FPS updated successfully",
		"new_fps", fps,
	)

	return nil
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
