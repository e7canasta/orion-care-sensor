package stream

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/tinyzimmer/go-gst/gst"
)

// StreamMetadata contains detected stream information
type StreamMetadata struct {
	Width      int
	Height     int
	FPS        int
	FrameRate  string // "6/1", "30000/1001", etc.
	Format     string // "H264", "JPEG", etc.
	DetectedAt time.Time
}

// ProbeRTSPStream connects to RTSP stream and detects metadata without starting full pipeline
func ProbeRTSPStream(rtspURL string, timeout time.Duration) (*StreamMetadata, error) {
	slog.Info("probing rtsp stream", "url", rtspURL)

	// Initialize GStreamer (safe to call multiple times)
	gst.Init(nil)

	// Create simple probe pipeline: rtspsrc → H264 decode → fakesink
	// This connects and negotiates caps without consuming frames
	// protocols=4: force TCP transport (4=TCP flag, required for go2rtc)
	// latency=200ms: for RTSP stability
	pipelineStr := fmt.Sprintf(
		"rtspsrc location=%s protocols=4 latency=200 ! "+
			"rtph264depay ! "+
			"h264parse ! "+
			"avdec_h264 ! "+
			"videoconvert ! "+
			"fakesink",
		rtspURL,
	)

	slog.Debug("creating probe pipeline", "pipeline", pipelineStr)

	pipeline, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create probe pipeline: %w", err)
	}
	defer pipeline.SetState(gst.StateNull)

	// Metadata to collect
	metadata := &StreamMetadata{
		DetectedAt: time.Now(),
	}

	// Channel to signal metadata detection
	metadataDetected := make(chan struct{})

	// Set up bus watch to detect stream info
	bus := pipeline.GetPipelineBus()
	bus.AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {
		case gst.MessageError:
			gerr := msg.ParseError()
			slog.Error("probe pipeline error",
				"error", gerr.Error(),
				"debug", gerr.DebugString(),
			)
			close(metadataDetected)
			return false

		case gst.MessageStateChanged:
			// When pipeline reaches PAUSED, caps are negotiated
			if msg.Source() == pipeline.GetName() {
				old, new := msg.ParseStateChanged()
				slog.Debug("probe state changed", "from", old, "to", new)

				if new == gst.StatePaused {
					// Extract metadata from pipeline elements
					if err := extractMetadata(pipeline, metadata); err != nil {
						slog.Error("failed to extract metadata", "error", err)
					} else {
						close(metadataDetected)
						return false // Stop watching
					}
				}
			}

		case gst.MessageStreamStart:
			slog.Debug("stream started")

		case gst.MessageAsyncDone:
			slog.Debug("async done, caps should be negotiated")
		}

		return true
	})

	// Set pipeline to PAUSED (this triggers caps negotiation without playing)
	if err := pipeline.SetState(gst.StatePaused); err != nil {
		return nil, fmt.Errorf("failed to pause pipeline: %w", err)
	}

	// Wait for metadata or timeout
	select {
	case <-metadataDetected:
		slog.Info("stream metadata detected",
			"width", metadata.Width,
			"height", metadata.Height,
			"fps", metadata.FPS,
			"framerate", metadata.FrameRate,
			"format", metadata.Format,
		)
		return metadata, nil

	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for stream metadata after %v", timeout)
	}
}

// extractMetadata extracts width, height, fps from pipeline caps
func extractMetadata(pipeline *gst.Pipeline, metadata *StreamMetadata) error {
	// Try to get elements from pipeline
	elements, err := pipeline.GetElements()
	if err != nil {
		return fmt.Errorf("failed to get pipeline elements: %w", err)
	}

	// Iterate through elements to find one with video caps
	for _, elem := range elements {
		elemName := elem.GetName()

		// Look for videoconvert or other video processing elements
		if elemName == "" {
			continue
		}

		// Try to get sink pads
		pads, err := elem.GetSinkPads()
		if err != nil || len(pads) == 0 {
			continue
		}

		pad := pads[0]
		caps := pad.GetCurrentCaps()

		if caps == nil || caps.GetSize() == 0 {
			continue
		}

		structure := caps.GetStructureAt(0)

		// Check if this is a video structure
		structName := structure.Name()
		if structName != "video/x-raw" && structName != "video/x-h264" {
			continue
		}

		// Extract width
		if val, err := structure.GetValue("width"); err == nil {
			if width, ok := val.(int); ok {
				metadata.Width = width
			}
		}

		// Extract height
		if val, err := structure.GetValue("height"); err == nil {
			if height, ok := val.(int); ok {
				metadata.Height = height
			}
		}

		// Extract framerate
		if val, err := structure.GetValue("framerate"); err == nil {
			// Framerate is a Gst.Fraction, extract as string for now
			metadata.FrameRate = fmt.Sprintf("%v", val)

			// Try to parse FPS as integer
			if fpsInt := parseFPS(metadata.FrameRate); fpsInt > 0 {
				metadata.FPS = fpsInt
			}
		}

		// Extract format (if video/x-raw)
		if val, err := structure.GetValue("format"); err == nil {
			if format, ok := val.(string); ok {
				metadata.Format = format
			}
		}

		slog.Debug("extracted metadata from caps",
			"element", elemName,
			"caps", caps.String(),
			"width", metadata.Width,
			"height", metadata.Height,
			"framerate", metadata.FrameRate,
		)

		// If we found width and height, we're good
		if metadata.Width > 0 && metadata.Height > 0 {
			return nil
		}
	}

	return fmt.Errorf("could not find video caps in pipeline")
}

// parseFPS converts framerate string to integer FPS
// Examples: "30/1" → 30, "6/1" → 6, "30000/1001" → 29
func parseFPS(framerateStr string) int {
	var numerator, denominator int

	// Try to parse as fraction "N/D"
	if _, err := fmt.Sscanf(framerateStr, "%d/%d", &numerator, &denominator); err == nil {
		if denominator > 0 {
			return numerator / denominator
		}
	}

	// Try to parse as integer
	var fps int
	if _, err := fmt.Sscanf(framerateStr, "%d", &fps); err == nil {
		return fps
	}

	return 0
}

// AdjustConfigFromMetadata adjusts configuration based on detected stream metadata
func AdjustConfigFromMetadata(metadata *StreamMetadata, targetWidth, targetHeight int, processInterval int) (adjustedWidth, adjustedHeight, adjustedInterval int, warnings []string) {
	adjustedWidth = targetWidth
	adjustedHeight = targetHeight
	adjustedInterval = processInterval
	warnings = make([]string, 0)

	// Adjust resolution if significantly different
	if metadata.Width > 0 && metadata.Height > 0 {
		if metadata.Width != targetWidth || metadata.Height != targetHeight {
			warnings = append(warnings, fmt.Sprintf(
				"Stream resolution (%dx%d) differs from config (%dx%d). Using stream resolution.",
				metadata.Width, metadata.Height, targetWidth, targetHeight,
			))
			adjustedWidth = metadata.Width
			adjustedHeight = metadata.Height
		}
	}

	// Adjust process_interval based on stream FPS
	if metadata.FPS > 0 {
		// If process_interval is higher than stream FPS, cap it
		if processInterval > metadata.FPS {
			warnings = append(warnings, fmt.Sprintf(
				"Process interval (%d frames) exceeds stream FPS (%d). Adjusting to %d frames.",
				processInterval, metadata.FPS, metadata.FPS,
			))
			adjustedInterval = metadata.FPS
		}

		// If stream is low FPS (6 FPS), ensure we process at least once per second
		if metadata.FPS <= 6 {
			// Process every frame if FPS is very low
			if processInterval > metadata.FPS {
				adjustedInterval = metadata.FPS
				warnings = append(warnings, fmt.Sprintf(
					"Low FPS stream (%d FPS). Processing every %d frames (1 Hz).",
					metadata.FPS, adjustedInterval,
				))
			}
		}
	}

	return adjustedWidth, adjustedHeight, adjustedInterval, warnings
}
