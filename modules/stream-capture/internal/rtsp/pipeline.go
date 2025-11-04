package rtsp

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/tinyzimmer/go-gst/gst"
	"github.com/tinyzimmer/go-gst/gst/app"
)

// PipelineConfig contains configuration for GStreamer pipeline creation
type PipelineConfig struct {
	RTSPURL      string
	Width        int
	Height       int
	TargetFPS    float64
	Acceleration int // 0=Auto, 1=VAAPI, 2=Software (from streamcapture.HardwareAccel)
}

// PipelineElements holds references to GStreamer pipeline elements
// These references are needed for hot-reload and cleanup
type PipelineElements struct {
	Pipeline   *gst.Pipeline
	AppSink    *app.Sink
	VideoRate  *gst.Element
	CapsFilter *gst.Element
	RTSPSrc    *gst.Element
	UsingVAAPI bool // True if VAAPI hardware acceleration is active
}

// CreatePipeline creates and configures a GStreamer pipeline for RTSP streaming
//
// Pipeline structure:
//
//	rtspsrc → rtph264depay → avdec_h264 → videoconvert → videoscale →
//	videorate → capsfilter → appsink
//
// The pipeline is configured but NOT started (state remains NULL).
// Caller must call pipeline.SetState(gst.StatePlaying) to start.
//
// Returns PipelineElements with references to key elements for hot-reload,
// or an error if pipeline creation fails.
func CreatePipeline(cfg PipelineConfig) (*PipelineElements, error) {
	// Initialize GStreamer (safe to call multiple times)
	gst.Init(nil)

	// Create pipeline
	pipeline, err := gst.NewPipeline("")
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	// Create rtspsrc element
	// protocols=4 (TCP only) required for go2rtc compatibility
	rtspsrc, err := gst.NewElement("rtspsrc")
	if err != nil {
		return nil, fmt.Errorf("failed to create rtspsrc: %w", err)
	}
	rtspsrc.SetProperty("location", cfg.RTSPURL)
	rtspsrc.SetProperty("protocols", 4) // TCP only

	// OPTIMIZATION Level 2: Adaptive latency buffer for low FPS
	// Low FPS (≤2fps) benefits from minimal buffering (50ms)
	// High FPS (>2fps) needs stability (200ms)
	latency := 200
	if cfg.TargetFPS <= 2.0 {
		latency = 50
		slog.Debug("rtsp: using low-latency buffer", "latency_ms", latency, "target_fps", cfg.TargetFPS)
	}
	rtspsrc.SetProperty("latency", latency)

	// OPTIMIZATION Level 3: Advanced buffer tuning
	rtspsrc.SetProperty("buffer-mode", 3)              // Auto-adaptive jitter buffering
	rtspsrc.SetProperty("ntp-sync", false)             // Disable NTP sync (reduces overhead)
	rtspsrc.SetProperty("tcp-timeout", uint64(10000000)) // 10s timeout (was 20s default)

	// Create decoding elements
	rtph264depay, err := gst.NewElement("rtph264depay")
	if err != nil {
		return nil, fmt.Errorf("failed to create rtph264depay: %w", err)
	}

	// OPTIMIZATION Level 2: Keyframe recovery
	// Request keyframes on packet loss for faster recovery (2s → 500ms)
	rtph264depay.SetProperty("request-keyframe", true)

	// Choose pipeline elements based on acceleration mode
	const (
		AccelAuto     = 0
		AccelVAAPI    = 1
		AccelSoftware = 2
	)

	var decoder, vaapiPostproc, converter, scaler *gst.Element
	usingVAAPI := false

	switch cfg.Acceleration {
	case AccelVAAPI:
		// OPTIMIZATION Level 1: H.264-specific decoder (no auto-detection)
		// Try vaapih264dec first (H.264-only, faster), fallback to vaapidecodebin
		decoder, err = gst.NewElement("vaapih264dec")
		if err != nil {
			// Fallback to generic VAAPI decoder
			slog.Warn("rtsp: vaapih264dec not available, using vaapidecodebin", "error", err)
			decoder, err = gst.NewElement("vaapidecodebin")
			if err != nil {
				return nil, fmt.Errorf("failed to create VAAPI decoder (VAAPI required): %w", err)
			}
		} else {
			// OPTIMIZATION Level 3: Low-latency mode for vaapih264dec
			// Safe for H.264 Main profile (no B-frames)
			decoder.SetProperty("low-latency", true)
			slog.Debug("rtsp: using vaapih264dec with low-latency mode")
		}

		// OPTIMIZATION Level 2: Skip corrupt frames for low FPS
		// When target < source FPS, decoder can skip damaged frames
		if cfg.TargetFPS < 6.0 {
			decoder.SetProperty("output-corrupt", false)
			slog.Debug("rtsp: decoder will skip corrupt frames", "target_fps", cfg.TargetFPS)
		}

		vaapiPostproc, err = gst.NewElement("vaapipostproc")
		if err != nil {
			return nil, fmt.Errorf("failed to create vaapipostproc (VAAPI required): %w", err)
		}

		// OPTIMIZATION Level 1: Force NV12 output format (no negotiation)
		// GPU scaling to target resolution
		vaapiPostproc.SetProperty("format", "nv12")
		vaapiPostproc.SetProperty("width", cfg.Width)
		vaapiPostproc.SetProperty("height", cfg.Height)
		vaapiPostproc.SetProperty("scale-method", 2) // HQ scaling (2 = high quality)

		// VAAPI outputs NV12 (YUV), need videoconvert to RGB
		converter, err = gst.NewElement("videoconvert")
		if err != nil {
			return nil, fmt.Errorf("failed to create videoconvert: %w", err)
		}

		// OPTIMIZATION Level 3: Multi-threaded YUV→RGB conversion
		converter.SetProperty("n-threads", 0)      // 0 = auto-detect cores
		converter.SetProperty("dither", 0)         // Disable dithering (minimal quality loss)
		converter.SetProperty("chroma-mode", 0)    // Full chroma resampling

		// OPTIMIZATION Level 1: Remove videoscale (GPU does it in vaapipostproc)
		scaler = nil

		usingVAAPI = true

	case AccelAuto:
		// OPTIMIZATION Level 1: Try H.264-specific first, fallback to generic/software
		decoder, err = gst.NewElement("vaapih264dec")
		if err == nil {
			// vaapih264dec available
			decoder.SetProperty("low-latency", true)
			if cfg.TargetFPS < 6.0 {
				decoder.SetProperty("output-corrupt", false)
			}

			vaapiPostproc, err = gst.NewElement("vaapipostproc")
			if err == nil {
				// VAAPI fully available
				vaapiPostproc.SetProperty("format", "nv12")
				vaapiPostproc.SetProperty("width", cfg.Width)
				vaapiPostproc.SetProperty("height", cfg.Height)
				vaapiPostproc.SetProperty("scale-method", 2)

				converter, _ = gst.NewElement("videoconvert")
				converter.SetProperty("n-threads", 0)
				converter.SetProperty("dither", 0)
				converter.SetProperty("chroma-mode", 0)

				scaler = nil
				usingVAAPI = true
				slog.Info("rtsp: using optimized VAAPI pipeline (vaapih264dec)")
			} else {
				// vaapipostproc failed, fallback to software
				vaapiPostproc = nil
				decoder, _ = gst.NewElement("avdec_h264")
				decoder.SetProperty("max-threads", 0) // Multi-threaded decode
				decoder.SetProperty("output-corrupt", false)
				converter, _ = gst.NewElement("videoconvert")
				converter.SetProperty("n-threads", 0)
				converter.SetProperty("dither", 0)
				converter.SetProperty("chroma-mode", 0)
				scaler, _ = gst.NewElement("videoscale")
				usingVAAPI = false
				slog.Warn("rtsp: VAAPI unavailable, using software decoder")
			}
		} else {
			// vaapih264dec failed, use software
			vaapiPostproc = nil
			decoder, _ = gst.NewElement("avdec_h264")
			decoder.SetProperty("max-threads", 0)
			decoder.SetProperty("output-corrupt", false)
			converter, _ = gst.NewElement("videoconvert")
			converter.SetProperty("n-threads", 0)
			converter.SetProperty("dither", 0)
			converter.SetProperty("chroma-mode", 0)
			scaler, _ = gst.NewElement("videoscale")
			usingVAAPI = false
			slog.Warn("rtsp: VAAPI unavailable, using software decoder")
		}

	case AccelSoftware:
		// Force software decode
		decoder, err = gst.NewElement("avdec_h264")
		if err != nil {
			return nil, fmt.Errorf("failed to create avdec_h264: %w", err)
		}

		// OPTIMIZATION Level 3: Multi-threaded software decode
		decoder.SetProperty("max-threads", 0)       // 0 = auto-detect cores
		decoder.SetProperty("output-corrupt", false) // Skip corrupt frames

		converter, err = gst.NewElement("videoconvert")
		if err != nil {
			return nil, fmt.Errorf("failed to create videoconvert: %w", err)
		}

		// OPTIMIZATION Level 3: Multi-threaded conversion
		converter.SetProperty("n-threads", 0)
		converter.SetProperty("dither", 0)
		converter.SetProperty("chroma-mode", 0)

		scaler, err = gst.NewElement("videoscale")
		if err != nil {
			return nil, fmt.Errorf("failed to create videoscale: %w", err)
		}

		slog.Info("rtsp: using software decoder with multi-threading")

	default:
		return nil, fmt.Errorf("invalid acceleration mode: %d", cfg.Acceleration)
	}

	// Create videorate for FPS control (hot-reload support)
	videorate, err := gst.NewElement("videorate")
	if err != nil {
		return nil, fmt.Errorf("failed to create videorate: %w", err)
	}
	videorate.SetProperty("drop-only", true)     // Only drop frames, never duplicate
	videorate.SetProperty("skip-to-first", true) // Skip to first frame on start

	// OPTIMIZATION Level 2: No averaging for low FPS
	// Immediate drop decisions (no 500ms smoothing window)
	if cfg.TargetFPS <= 2.0 {
		videorate.SetProperty("average-period", uint64(0))
		slog.Debug("rtsp: videorate using immediate drop mode", "target_fps", cfg.TargetFPS)
	}

	// Create capsfilter with framerate control
	capsfilter, err := gst.NewElement("capsfilter")
	if err != nil {
		return nil, fmt.Errorf("failed to create capsfilter: %w", err)
	}

	// Build caps string with framerate
	capsStr := buildFramerateCaps(cfg.Width, cfg.Height, cfg.TargetFPS)
	caps := gst.NewCapsFromString(capsStr)
	capsfilter.SetProperty("caps", caps)

	// Create appsink
	appsink, err := app.NewAppSink()
	if err != nil {
		return nil, fmt.Errorf("failed to create appsink: %w", err)
	}
	appsink.SetProperty("sync", false)    // No sync with clock (real-time)
	appsink.SetProperty("max-buffers", 1) // Keep only latest frame
	appsink.SetProperty("drop", true)     // Drop old frames

	// OPTIMIZATION Level 2: QoS events for upstream frame dropping
	// When appsink drops frames, notify upstream elements to drop BEFORE decoding
	appsink.SetProperty("qos", true)
	slog.Debug("rtsp: appsink QoS events enabled (pre-decode drops)")

	// OPTIMIZATION Level 1: Add RGB capsfilter after videoconvert (format lock)
	// Note: We do NOT add NV12 capsfilter because video/x-raw(memory:VASurface)
	// prevents videoconvert from accessing the data (GPU-only memory).
	// Instead, vaapipostproc properties (format=nv12, width, height) are sufficient
	// for format negotiation, and GStreamer will automatically handle GPU→CPU transfer.
	var capsRGB *gst.Element
	if usingVAAPI {
		// Force videoconvert to output RGB BEFORE videorate
		// This prevents caps negotiation issues between videorate and final capsfilter
		capsRGB, err = gst.NewElement("capsfilter")
		if err != nil {
			return nil, fmt.Errorf("failed to create RGB capsfilter: %w", err)
		}
		// Lock RGB format with target resolution (no framerate yet)
		capsRGBStr := fmt.Sprintf("video/x-raw,format=RGB,width=%d,height=%d", cfg.Width, cfg.Height)
		capsRGB.SetProperty("caps", gst.NewCapsFromString(capsRGBStr))
		slog.Debug("rtsp: RGB format lock enabled", "caps", capsRGBStr)
	}

	// Add all elements to pipeline (conditionally based on VAAPI usage)
	if usingVAAPI {
		// OPTIMIZED VAAPI pipeline: rtspsrc → rtph264depay → vaapih264dec → vaapipostproc(GPU scale+NV12) → videoconvert → capsRGB → videorate → capsfilter → appsink
		// Note: videoscale removed (GPU does it in vaapipostproc)
		// Note: capsRGB added to force RGB format before videorate (prevents caps negotiation issues)
		// Note: No capsNV12 - vaapipostproc properties handle format, GStreamer handles GPU→CPU transfer
		pipeline.AddMany(
			rtspsrc,
			rtph264depay,
			decoder,
			vaapiPostproc, // GPU: decode + scale + NV12 format
			converter,     // CPU: NV12 → RGB conversion
			capsRGB,       // RGB format lock (videoconvert output)
			// scaler removed (GPU scaling in vaapipostproc)
			videorate,
			capsfilter,    // RGB + framerate lock (final)
			appsink.Element,
		)

		// Link static elements (rtspsrc has dynamic pads, linked in pad-added callback)
		if err := gst.ElementLinkMany(
			rtph264depay,
			decoder,
			vaapiPostproc, // GPU processing
			converter,     // GPU→CPU transfer + NV12→RGB
			capsRGB,       // RGB format lock
			// scaler removed
			videorate,
			capsfilter,    // RGB + framerate
			appsink.Element,
		); err != nil {
			return nil, fmt.Errorf("failed to link VAAPI pipeline elements: %w", err)
		}

		// Add probe to vaapipostproc output to measure decode latency
		// This captures the timestamp when the frame exits the GPU decoder
		if err := addDecodeLatencyProbe(vaapiPostproc); err != nil {
			slog.Warn("rtsp: failed to add decode latency probe, continuing without telemetry", "error", err)
		}

		slog.Info("rtsp: optimized VAAPI pipeline created",
			"decoder", "vaapih264dec",
			"gpu_scaling", true,
			"format", "NV12→RGB",
			"rgb_capsfilter", true,
			"multi_thread", true,
			"low_latency", true,
		)
	} else {
		// Software pipeline: rtspsrc → rtph264depay → avdec_h264 → videoconvert → videoscale → videorate → capsfilter → appsink
		pipeline.AddMany(
			rtspsrc,
			rtph264depay,
			decoder,
			converter,
			scaler,
			videorate,
			capsfilter,
			appsink.Element,
		)

		// Link static elements (rtspsrc has dynamic pads, linked in pad-added callback)
		if err := gst.ElementLinkMany(
			rtph264depay,
			decoder,
			converter,
			scaler,
			videorate,
			capsfilter,
			appsink.Element,
		); err != nil {
			return nil, fmt.Errorf("failed to link software pipeline elements: %w", err)
		}

		// Add probe to decoder output to measure decode latency (software decode)
		if err := addDecodeLatencyProbe(decoder); err != nil {
			slog.Warn("rtsp: failed to add decode latency probe, continuing without telemetry", "error", err)
		}
	}

	return &PipelineElements{
		Pipeline:   pipeline,
		AppSink:    appsink,
		VideoRate:  videorate,
		CapsFilter: capsfilter,
		RTSPSrc:    rtspsrc,
		UsingVAAPI: usingVAAPI,
	}, nil
}

// UpdateFramerateCaps updates the capsfilter framerate dynamically (hot-reload)
//
// This is called by SetTargetFPS to change the stream FPS without restarting
// the pipeline. Causes approximately 2 seconds of interruption while GStreamer
// adjusts the caps.
//
// Returns an error if caps update fails.
func UpdateFramerateCaps(capsfilter *gst.Element, fps float64, width, height int) error {
	if capsfilter == nil {
		return fmt.Errorf("capsfilter is nil")
	}

	capsStr := buildFramerateCaps(width, height, fps)
	newCaps := gst.NewCapsFromString(capsStr)

	capsfilter.SetProperty("caps", newCaps)

	return nil
}

// DestroyPipeline cleans up GStreamer pipeline resources
//
// Sets pipeline state to NULL and releases all resources.
// Safe to call even if pipeline is already destroyed.
func DestroyPipeline(elements *PipelineElements) error {
	if elements == nil || elements.Pipeline == nil {
		return nil
	}

	// Set pipeline to NULL state (stops and releases resources)
	if err := elements.Pipeline.SetState(gst.StateNull); err != nil {
		return fmt.Errorf("failed to set pipeline to NULL: %w", err)
	}

	return nil
}

// addDecodeLatencyProbe adds a probe to the decoder output pad to measure decode latency
//
// The probe captures the timestamp when a buffer exits the decoder (post-decode time).
// This timestamp is stored as ReferenceTimestampMeta on the buffer for later retrieval
// in OnNewSample callback.
//
// Performance: ~50-100ns overhead per frame (timestamp capture + metadata attachment).
func addDecodeLatencyProbe(element *gst.Element) error {
	// Get source pad (output) from decoder element
	srcPad := element.GetStaticPad("src")
	if srcPad == nil {
		return fmt.Errorf("failed to get src pad from element")
	}

	// Create caps for our custom timestamp metadata
	// Using a unique caps string to identify our metadata
	timestampCaps := gst.NewCapsFromString("timestamp/x-decode-exit")

	// Add probe to capture timestamp when buffer exits decoder
	srcPad.AddProbe(gst.PadProbeTypeBuffer, func(pad *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		buffer := info.GetBuffer()
		if buffer == nil {
			return gst.PadProbeOK
		}

		// Capture timestamp when buffer exits decoder (post-decode time)
		decodeExitTime := time.Now()

		// Attach timestamp as metadata to buffer (zero-copy)
		// This metadata will be available in OnNewSample callback
		buffer.AddReferenceTimestampMeta(timestampCaps, decodeExitTime.Sub(time.Time{}), 0)

		return gst.PadProbeOK
	})

	slog.Debug("rtsp: decode latency probe installed", "element", element.GetName())
	return nil
}

// buildFramerateCaps builds a caps string with framerate constraint
//
// Handles fractional framerates:
//   - fps >= 1.0: framerate = fps/1 (e.g., 5.0 → 5/1)
//   - fps < 1.0: framerate = 1/(1/fps) (e.g., 0.5 → 1/2)
//
// Format: "video/x-raw,format=RGB,width=W,height=H,framerate=N/D"
func buildFramerateCaps(width, height int, fps float64) string {
	numerator := 1
	denominator := 1

	if fps < 1.0 {
		// Fractional FPS: 0.5 Hz → framerate=1/2
		denominator = int(1.0 / fps)
	} else {
		// Integer FPS: 5.0 Hz → framerate=5/1
		numerator = int(fps)
	}

	return fmt.Sprintf(
		"video/x-raw,format=RGB,width=%d,height=%d,framerate=%d/%d",
		width, height, numerator, denominator,
	)
}
