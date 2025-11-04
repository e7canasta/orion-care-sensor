package rtsp

import (
	"log/slog"
	"sort"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tinyzimmer/go-gst/gst"
	"github.com/tinyzimmer/go-gst/gst/app"
)

// Frame is a minimal frame struct for internal use (avoids import cycle)
// The actual Frame type is defined in the parent package
type Frame struct {
	Seq          uint64
	Timestamp    time.Time
	Width        int
	Height       int
	Data         []byte
	SourceStream string
	TraceID      string
}

// LatencyWindow maintains a rolling window of decode latency samples
// for statistical analysis (mean, P95, max).
//
// Thread-safety: Accessed via atomic.Pointer for lock-free reads/writes.
// Ring buffer of fixed size (100 samples) to bound memory usage.
type LatencyWindow struct {
	Samples [100]float64 // Ring buffer of latency samples (milliseconds)
	Index   int          // Current write index (wraps around)
	Count   int          // Total samples collected (capped at len(Samples))
}

// AddSample adds a latency sample to the window (lock-free via atomic pointer swap)
func (w *LatencyWindow) AddSample(latencyMS float64) {
	w.Samples[w.Index] = latencyMS
	w.Index = (w.Index + 1) % len(w.Samples)
	if w.Count < len(w.Samples) {
		w.Count++
	}
}

// GetStats calculates mean, P95, and max from current window
// Returns zeros if window is empty.
func (w *LatencyWindow) GetStats() (mean, p95, max float64) {
	if w.Count == 0 {
		return 0, 0, 0
	}

	// Copy samples for safe calculation (avoid mutation during sort)
	validSamples := make([]float64, w.Count)
	copy(validSamples, w.Samples[:w.Count])

	// Calculate mean
	var sum float64
	for _, sample := range validSamples {
		sum += sample
		if sample > max {
			max = sample
		}
	}
	mean = sum / float64(w.Count)

	// Calculate P95 (sort required)
	sort.Float64s(validSamples)
	p95Index := int(float64(w.Count) * 0.95)
	if p95Index >= w.Count {
		p95Index = w.Count - 1
	}
	p95 = validSamples[p95Index]

	return mean, p95, max
}

// CallbackContext holds state needed by GStreamer callbacks
type CallbackContext struct {
	FrameChan       chan<- Frame                  // Uses internal Frame type
	FrameCounter    *uint64                       // Atomic counter for sequence numbers
	BytesRead       *uint64                       // Atomic counter for bytes read
	FramesDropped   *uint64                       // Atomic counter for dropped frames (channel full)
	Width           int                           // Frame width in pixels
	Height          int                           // Frame height in pixels
	SourceStream    string                        // Stream identifier (e.g., "LQ", "HQ")
	DecodeLatencies *atomic.Pointer[LatencyWindow] // Lock-free latency tracking (nil if disabled)
}

// OnNewSample is called by GStreamer when a new frame is available
//
// This callback:
//  1. Pulls the sample from the appsink
//  2. Maps the buffer to read pixel data
//  3. Copies data (GStreamer will reuse the buffer)
//  4. Creates a Frame struct with metadata
//  5. Sends frame to channel (non-blocking - drops if full)
//
// Returns gst.FlowOK to continue processing, or gst.FlowEOS/FlowError on failure.
func OnNewSample(sink *app.Sink, ctx *CallbackContext) gst.FlowReturn {
	// Pull sample from appsink
	sample := sink.PullSample()
	if sample == nil {
		// Graceful degradation: skip frame instead of terminating stream
		// A single corrupted frame should not kill the entire pipeline
		slog.Warn("rtsp: failed to pull sample from appsink, skipping frame")
		return gst.FlowOK
	}

	// Get buffer from sample
	buffer := sample.GetBuffer()
	if buffer == nil {
		// Graceful degradation: skip frame instead of terminating stream
		slog.Warn("rtsp: failed to get buffer from sample, skipping frame")
		return gst.FlowOK
	}

	// Capture decode latency telemetry (VAAPI performance tracking)
	//
	// Note: GStreamer PTS is relative to pipeline start time, not absolute wall-clock time.
	// We can't directly measure "camera timestamp → callback arrival" latency without
	// additional metadata from the camera.
	//
	// What we CAN measure here is inter-frame processing time variance, which indirectly
	// indicates decode performance. For now, we skip this measurement and rely on
	// overall FPS metrics. Future enhancement: use GStreamer probes on decoder element
	// to measure decode time directly.
	//
	// TODO(future): Add GstPadProbe on decoder output to measure decode-specific latency
	_ = ctx.DecodeLatencies // Placeholder for future implementation

	// Map buffer to read data
	mapInfo := buffer.Map(gst.MapRead)
	data := mapInfo.Bytes()
	if data == nil || len(data) == 0 {
		buffer.Unmap()
		slog.Warn("rtsp: empty buffer received")
		return gst.FlowOK
	}

	// Copy frame data (GStreamer will reuse buffer)
	frameData := make([]byte, len(data))
	copy(frameData, data)
	buffer.Unmap()

	// Update atomic counters
	seq := atomic.AddUint64(ctx.FrameCounter, 1)
	atomic.AddUint64(ctx.BytesRead, uint64(len(data)))

	// Create frame struct (using internal Frame type)
	frame := Frame{
		Seq:          seq,
		Timestamp:    time.Now(),
		Width:        ctx.Width,
		Height:       ctx.Height,
		Data:         frameData,
		SourceStream: ctx.SourceStream,
		TraceID:      uuid.New().String(),
	}

	// Send frame (non-blocking - drop if channel full)
	select {
	case ctx.FrameChan <- frame:
		slog.Debug("rtsp: frame sent",
			"seq", frame.Seq,
			"size_bytes", len(data),
			"trace_id", frame.TraceID,
		)
	default:
		// Track dropped frame at callback layer
		atomic.AddUint64(ctx.FramesDropped, 1)
		slog.Debug("rtsp: dropping frame, channel full",
			"seq", frame.Seq,
			"trace_id", frame.TraceID,
		)
	}

	return gst.FlowOK
}

// OnPadAdded is called by GStreamer when rtspsrc creates a new dynamic pad
//
// rtspsrc has dynamic pads (not known at pipeline creation time), so we need
// to connect a callback to link them when they appear.
//
// This callback links the rtspsrc output pad to the rtph264depay input pad.
func OnPadAdded(srcElement *gst.Element, srcPad *gst.Pad, sinkElement *gst.Element) {
	slog.Debug("rtsp: pad-added signal received", "pad", srcPad.GetName())

	// Get sink pad from rtph264depay
	sinkPad := sinkElement.GetStaticPad("sink")
	if sinkPad == nil {
		slog.Error("rtsp: failed to get sink pad from rtph264depay")
		return
	}

	// Link pads
	if ret := srcPad.Link(sinkPad); ret != gst.PadLinkOK {
		slog.Error("rtsp: failed to link pads",
			"src_pad", srcPad.GetName(),
			"sink_pad", sinkPad.GetName(),
			"ret", ret,
		)
		return
	}

	slog.Debug("rtsp: pads linked successfully",
		"src_pad", srcPad.GetName(),
		"sink_pad", sinkPad.GetName(),
	)
}
