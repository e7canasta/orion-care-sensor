package rtsp

import (
	"fmt"

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
	rtspsrc.SetProperty("latency", 200) // 200ms buffering

	// Create decoding elements
	rtph264depay, err := gst.NewElement("rtph264depay")
	if err != nil {
		return nil, fmt.Errorf("failed to create rtph264depay: %w", err)
	}

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
		// Force VAAPI - fail if not available
		decoder, err = gst.NewElement("vaapidecodebin")
		if err != nil {
			return nil, fmt.Errorf("failed to create vaapidecodebin (VAAPI required): %w", err)
		}

		vaapiPostproc, err = gst.NewElement("vaapipostproc")
		if err != nil {
			return nil, fmt.Errorf("failed to create vaapipostproc (VAAPI required): %w", err)
		}

		// VAAPI outputs NV12 (YUV), need videoconvert to RGB
		converter, err = gst.NewElement("videoconvert")
		if err != nil {
			return nil, fmt.Errorf("failed to create videoconvert: %w", err)
		}

		scaler, err = gst.NewElement("videoscale")
		if err != nil {
			return nil, fmt.Errorf("failed to create videoscale: %w", err)
		}

		usingVAAPI = true

	case AccelAuto:
		// Try VAAPI, fallback to software
		decoder, err = gst.NewElement("vaapidecodebin")
		if err == nil {
			vaapiPostproc, err = gst.NewElement("vaapipostproc")
			if err == nil {
				// VAAPI available
				converter, _ = gst.NewElement("videoconvert")
				scaler, _ = gst.NewElement("videoscale")
				usingVAAPI = true
			} else {
				// vaapipostproc failed, fallback to software
				vaapiPostproc = nil
				decoder, _ = gst.NewElement("avdec_h264")
				converter, _ = gst.NewElement("videoconvert")
				scaler, _ = gst.NewElement("videoscale")
			}
		} else {
			// vaapidecodebin failed, use software
			vaapiPostproc = nil
			decoder, _ = gst.NewElement("avdec_h264")
			converter, _ = gst.NewElement("videoconvert")
			scaler, _ = gst.NewElement("videoscale")
		}

	case AccelSoftware:
		// Force software decode
		decoder, err = gst.NewElement("avdec_h264")
		if err != nil {
			return nil, fmt.Errorf("failed to create avdec_h264: %w", err)
		}

		converter, err = gst.NewElement("videoconvert")
		if err != nil {
			return nil, fmt.Errorf("failed to create videoconvert: %w", err)
		}

		scaler, err = gst.NewElement("videoscale")
		if err != nil {
			return nil, fmt.Errorf("failed to create videoscale: %w", err)
		}

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

	// Add all elements to pipeline (conditionally based on VAAPI usage)
	if usingVAAPI {
		// VAAPI pipeline: rtspsrc → rtph264depay → vaapidecodebin → vaapipostproc → videoconvert → videoscale → videorate → capsfilter → appsink
		pipeline.AddMany(
			rtspsrc,
			rtph264depay,
			decoder,
			vaapiPostproc,
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
			vaapiPostproc,
			converter,
			scaler,
			videorate,
			capsfilter,
			appsink.Element,
		); err != nil {
			return nil, fmt.Errorf("failed to link VAAPI pipeline elements: %w", err)
		}
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
	}

	return &PipelineElements{
		Pipeline:   pipeline,
		AppSink:    appsink,
		VideoRate:  videorate,
		CapsFilter: capsfilter,
		RTSPSrc:    rtspsrc,
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
