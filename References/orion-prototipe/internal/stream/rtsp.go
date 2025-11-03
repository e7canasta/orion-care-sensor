package stream

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/care/orion/internal/types"
	"github.com/google/uuid"
	"github.com/tinyzimmer/go-gst/gst"
	"github.com/tinyzimmer/go-gst/gst/app"
)

// RTSPStream implements StreamProvider using GStreamer for RTSP streaming
type RTSPStream struct {
	// Configuration
	rtspURL      string
	width        int
	height       int
	targetFPS    float64 // Changed to float64 for hot-reload (e.g., 0.5, 1.0, 2.0 Hz)
	sourceStream string

	// GStreamer pipeline
	pipeline   *gst.Pipeline
	appsink    *app.Sink
	videorate  *gst.Element // Reference for hot-reload
	capsfilter *gst.Element // Reference for hot-reload

	// Frame output
	frames chan types.Frame
	mu     sync.RWMutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup

	// Stats
	frameCount   uint64
	started      time.Time
	lastFrameAt  time.Time
	reconnects   uint32
	bytesRead    uint64

	// Reconnection
	maxRetries     int
	retryDelay     time.Duration
	maxRetryDelay  time.Duration
	currentRetries int
}

// RTSPConfig contains RTSP stream configuration
type RTSPConfig struct {
	RTSPURL      string
	Width        int
	Height       int
	FPS          int
	SourceStream string // "LQ" or "HQ"
}

// NewRTSPStream creates a new RTSP stream
func NewRTSPStream(cfg RTSPConfig) (*RTSPStream, error) {
	if cfg.RTSPURL == "" {
		return nil, fmt.Errorf("rtsp_url is required")
	}

	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("invalid resolution: %dx%d", cfg.Width, cfg.Height)
	}

	s := &RTSPStream{
		rtspURL:       cfg.RTSPURL,
		width:         cfg.Width,
		height:        cfg.Height,
		targetFPS:     float64(cfg.FPS), // Convert to float64
		sourceStream:  cfg.SourceStream,
		frames:        make(chan types.Frame, 10),
		done:          make(chan struct{}),
		maxRetries:    5,
		retryDelay:    1 * time.Second,
		maxRetryDelay: 30 * time.Second,
	}

	return s, nil
}

// SetTargetFPS updates the target FPS dynamically (hot-reload)
// This adjusts the GStreamer videorate element caps to control frame rate at the source
func (s *RTSPStream) SetTargetFPS(fps float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if fps <= 0 || fps > 30 {
		return fmt.Errorf("invalid FPS: %.2f (must be between 0.1 and 30)", fps)
	}

	slog.Info("updating stream target FPS",
		"old_fps", s.targetFPS,
		"new_fps", fps,
	)

	s.targetFPS = fps

	// Update caps filter if pipeline is running
	if s.capsfilter != nil {
		// Build new caps with framerate
		// Format: "video/x-raw,format=RGB,width=640,height=480,framerate=1/1"
		// For fractional rates like 0.5 Hz, use framerate=1/2

		numerator := 1
		denominator := 1

		// Handle fractional framerates
		if fps < 1.0 {
			denominator = int(1.0 / fps)
		} else {
			numerator = int(fps)
		}

		capsStr := fmt.Sprintf(
			"video/x-raw,format=RGB,width=%d,height=%d,framerate=%d/%d",
			s.width, s.height, numerator, denominator,
		)

		newCaps := gst.NewCapsFromString(capsStr)
		s.capsfilter.SetProperty("caps", newCaps)

		slog.Info("stream FPS updated",
			"fps", fps,
			"framerate", fmt.Sprintf("%d/%d", numerator, denominator),
			"caps", capsStr,
		)
	} else {
		slog.Warn("capsfilter not available, FPS will apply on next reconnect")
	}

	return nil
}

// Start initializes GStreamer and starts the RTSP pipeline
func (s *RTSPStream) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		return fmt.Errorf("stream already started")
	}

	// Initialize GStreamer
	gst.Init(nil)

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = time.Now()

	// Start pipeline in goroutine
	s.wg.Add(1)
	go s.runPipeline()

	slog.Info("rtsp stream starting",
		"url", s.rtspURL,
		"resolution", fmt.Sprintf("%dx%d", s.width, s.height),
		"target_fps", s.targetFPS,
	)

	return nil
}

// runPipeline runs the GStreamer pipeline with reconnection logic
func (s *RTSPStream) runPipeline() {
	defer s.wg.Done()
	defer close(s.frames)
	defer close(s.done)

	for {
		select {
		case <-s.ctx.Done():
			slog.Info("rtsp pipeline context cancelled")
			return
		default:
		}

		err := s.connectAndStream()
		if err != nil {
			slog.Error("rtsp pipeline error", "error", err)
		}

		// Check if we should reconnect
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Reconnection logic
		s.currentRetries++
		atomic.AddUint32(&s.reconnects, 1)

		if s.currentRetries > s.maxRetries {
			slog.Error("max retries exceeded, stopping stream",
				"retries", s.currentRetries,
				"max_retries", s.maxRetries,
			)
			return
		}

		// Exponential backoff
		delay := s.retryDelay * time.Duration(1<<uint(s.currentRetries-1))
		if delay > s.maxRetryDelay {
			delay = s.maxRetryDelay
		}

		slog.Warn("reconnecting to rtsp stream",
			"retry", s.currentRetries,
			"delay", delay,
		)

		select {
		case <-time.After(delay):
			continue
		case <-s.ctx.Done():
			return
		}
	}
}

// connectAndStream establishes RTSP connection and streams frames
func (s *RTSPStream) connectAndStream() error {
	// Initialize GStreamer (safe to call multiple times)
	gst.Init(nil)

	// Create pipeline manually to use app.Sink
	pipeline, err := gst.NewPipeline("")
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}
	s.pipeline = pipeline

	// Create elements
	// protocols=4 (TCP) required for go2rtc compatibility
	rtspsrc, err := gst.NewElement("rtspsrc")
	if err != nil {
		return fmt.Errorf("failed to create rtspsrc: %w", err)
	}
	rtspsrc.SetProperty("location", s.rtspURL)
	rtspsrc.SetProperty("protocols", 4) // TCP
	rtspsrc.SetProperty("latency", 200)

	rtph264depay, _ := gst.NewElement("rtph264depay")
	avdec_h264, _ := gst.NewElement("avdec_h264")
	videoconvert, _ := gst.NewElement("videoconvert")
	videoscale, _ := gst.NewElement("videoscale")

	// Add videorate element for frame rate control (hot-reload support)
	videorate, _ := gst.NewElement("videorate")
	videorate.SetProperty("drop-only", true)  // Only drop frames, don't duplicate
	videorate.SetProperty("skip-to-first", true)
	s.videorate = videorate

	// Capsfilter with framerate control
	capsfilter, _ := gst.NewElement("capsfilter")

	// Calculate framerate fraction
	numerator := 1
	denominator := 1
	if s.targetFPS < 1.0 {
		denominator = int(1.0 / s.targetFPS)
	} else {
		numerator = int(s.targetFPS)
	}

	caps := gst.NewCapsFromString(fmt.Sprintf(
		"video/x-raw,format=RGB,width=%d,height=%d,framerate=%d/%d",
		s.width, s.height, numerator, denominator,
	))
	capsfilter.SetProperty("caps", caps)
	s.capsfilter = capsfilter

	// Create appsink using app package (correct API!)
	appsink, err := app.NewAppSink()
	if err != nil {
		return fmt.Errorf("failed to create appsink: %w", err)
	}
	s.appsink = appsink

	// Configure appsink
	appsink.SetProperty("sync", false)
	appsink.SetProperty("max-buffers", 1)
	appsink.SetProperty("drop", true)

	// Set callbacks for appsink - THIS IS THE CORRECT WAY!
	appsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			return s.onNewSample(sink)
		},
	})

	// Add elements to pipeline
	pipeline.AddMany(rtspsrc, rtph264depay, avdec_h264, videoconvert, videoscale, videorate, capsfilter, appsink.Element)

	// Link static elements (rtspsrc has dynamic pads, will link in pad-added callback)
	// Order: decode → convert → scale → videorate → capsfilter (with framerate) → sink
	// Rate limiting is NOW handled by GStreamer videorate, not Go worker
	gst.ElementLinkMany(rtph264depay, avdec_h264, videoconvert, videoscale, videorate, capsfilter, appsink.Element)

	// Connect pad-added signal for rtspsrc dynamic pads
	rtspsrc.Connect("pad-added", func(self *gst.Element, srcPad *gst.Pad) {
		slog.Debug("rtspsrc pad added", "pad", srcPad.GetName())
		sinkPad := rtph264depay.GetStaticPad("sink")
		if sinkPad != nil {
			srcPad.Link(sinkPad)
		}
	})

	// Start playing
	slog.Debug("setting pipeline to playing")
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return fmt.Errorf("failed to set pipeline to playing: %w", err)
	}

	// Process bus messages - THIS IS THE CORRECT WAY (not glib.MainLoop)!
	bus := pipeline.GetPipelineBus()
	for {
		select {
		case <-s.ctx.Done():
			slog.Debug("context cancelled, stopping pipeline")
			pipeline.SetState(gst.StateNull)
			return nil
		default:
		}

		// Poll for messages with short timeout for responsive shutdown
		msg := bus.TimedPop(50 * time.Millisecond)
		if msg == nil {
			continue
		}

		switch msg.Type() {
		case gst.MessageEOS:
			slog.Info("end of stream")
			return nil

		case gst.MessageError:
			gerr := msg.ParseError()
			slog.Error("pipeline error",
				"error", gerr.Error(),
				"debug", gerr.DebugString(),
			)
			return fmt.Errorf("pipeline error: %w", gerr)

		case gst.MessageStateChanged:
			if msg.Source() == pipeline.GetName() {
				old, new := msg.ParseStateChanged()
				slog.Debug("pipeline state changed", "from", old, "to", new)

				if new == gst.StatePlaying {
					s.currentRetries = 0
					slog.Info("rtsp stream connected successfully")
				}
			}
		}
	}
}

// onNewSample is called by GStreamer when a new frame is available
// Uses app.Sink.PullSample() - the correct API from go-gst examples
func (s *RTSPStream) onNewSample(sink *app.Sink) gst.FlowReturn {
	// Pull the sample - THIS IS THE CORRECT WAY per go-gst examples!
	sample := sink.PullSample()
	if sample == nil {
		return gst.FlowEOS
	}

	// Get buffer from sample
	buffer := sample.GetBuffer()
	if buffer == nil {
		return gst.FlowError
	}

	// Map buffer to read data
	mapInfo := buffer.Map(gst.MapRead)
	data := mapInfo.Bytes()
	defer buffer.Unmap()

	if data == nil || len(data) == 0 {
		return gst.FlowOK
	}

	// Create frame copy
	frameData := make([]byte, len(data))
	copy(frameData, data)

	frame := types.Frame{
		Data:         frameData,
		Width:        s.width,
		Height:       s.height,
		Timestamp:    time.Now(),
		Seq:          atomic.AddUint64(&s.frameCount, 1),
		SourceStream: s.sourceStream,
		TraceID:      uuid.New().String(),
	}

	s.lastFrameAt = time.Now()
	atomic.AddUint64(&s.bytesRead, uint64(len(data)))

	// Send frame (non-blocking)
	select {
	case s.frames <- frame:
		slog.Debug("frame sent",
			"seq", frame.Seq,
			"size_bytes", len(data),
			"trace_id", frame.TraceID)
	default:
		slog.Debug("dropping frame, channel full",
			"seq", frame.Seq,
			"trace_id", frame.TraceID)
	}

	return gst.FlowOK
}

// Frames returns the channel of frames
func (s *RTSPStream) Frames() <-chan types.Frame {
	return s.frames
}

// Stop stops the RTSP stream
func (s *RTSPStream) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel == nil {
		return fmt.Errorf("stream not started")
	}

	slog.Info("stopping rtsp stream")

	// Cancel context to signal shutdown
	s.cancel()

	// Wait for pipeline goroutine to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean shutdown
		slog.Info("rtsp stream stopped",
			"frames_received", atomic.LoadUint64(&s.frameCount),
			"reconnects", atomic.LoadUint32(&s.reconnects),
			"uptime", time.Since(s.started),
		)
	case <-time.After(3 * time.Second):
		// Timeout - pipeline didn't stop cleanly
		slog.Warn("rtsp stream stop timeout, pipeline may still be running",
			"frames_received", atomic.LoadUint64(&s.frameCount),
			"uptime", time.Since(s.started),
		)
	}

	// Reset state to allow restart (critical for hot-reload)
	s.cancel = nil
	s.ctx = nil
	s.pipeline = nil
	s.appsink = nil
	s.videorate = nil
	s.capsfilter = nil

	// Recreate channels for restart (frames closed by runPipeline defer)
	s.frames = make(chan types.Frame, 10)
	s.done = make(chan struct{})

	slog.Debug("rtsp stream state reset, ready for restart")

	return nil
}

// Stats returns current stream statistics
func (s *RTSPStream) Stats() types.StreamStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	frameCount := atomic.LoadUint64(&s.frameCount)
	uptime := time.Since(s.started).Seconds()

	var fpsReal float64
	if uptime > 0 {
		fpsReal = float64(frameCount) / uptime
	}

	var latencyMS int64
	if !s.lastFrameAt.IsZero() {
		latencyMS = time.Since(s.lastFrameAt).Milliseconds()
	}

	return types.StreamStats{
		FPSTarget:    int(s.targetFPS), // Convert back to int for stats
		FPSReal:      fpsReal,
		FrameCount:   frameCount,
		LatencyMS:    latencyMS,
		SourceStream: s.sourceStream,
		Resolution:   fmt.Sprintf("%dx%d", s.width, s.height),
		Reconnects:   atomic.LoadUint32(&s.reconnects),
		BytesRead:    atomic.LoadUint64(&s.bytesRead),
	}
}
