package types

import "time"

// Frame represents a single video frame
type Frame struct {
	// Seq is the monotonic sequence number
	Seq uint64
	// Timestamp is when the frame was captured/decoded
	Timestamp time.Time
	// Width in pixels
	Width int
	// Height in pixels
	Height int
	// Data contains the frame data (BGR24 format by default)
	Data []byte
	// SourceStream identifies the stream (LQ, HQ)
	SourceStream string
	// TraceID is a unique identifier for distributed tracing across the pipeline
	TraceID string
	// ROIProcessing contains ROI metadata for intelligent cropping (optional)
	ROIProcessing *ROIProcessingMetadata
}

// ROIProcessingMetadata contains information about how to process the frame with ROIs
type ROIProcessingMetadata struct {
	// AttentionROIs are the regions of interest commanded by the expert
	// These are normalized coordinates (0.0-1.0) to be resolution-agnostic
	AttentionROIs []NormalizedRect
	// MergedROI is the computed bounding box that encompasses all attention ROIs
	// This is what will actually be cropped from the frame
	MergedROI *NormalizedRect
	// TargetSize is the recommended model input size (320 or 640)
	// Selected based on the merged ROI dimensions
	TargetSize int
	// CropApplied indicates if the frame has already been cropped to the ROI
	CropApplied bool

	// === AUTO-FOCUS METADATA ===
	// Source indicates where the ROIs came from
	// Values: "external" (MQTT command), "auto" (auto-focus), "hybrid" (both)
	Source string `json:"source,omitempty"`
	// AutoFocusEnabled indicates if auto-focus tracking is active
	AutoFocusEnabled bool `json:"auto_focus_enabled"`
	// HistoryFrames is the number of past frames used for auto-focus
	HistoryFrames int `json:"history_frames,omitempty"`
	// DetectionCount is the total number of detections in the auto-focus history
	DetectionCount int `json:"detection_count,omitempty"`
	// ExpansionPct is the percentage by which the auto-focus ROI was expanded
	ExpansionPct float64 `json:"expansion_pct,omitempty"`
}

// NormalizedRect represents a rectangle with normalized coordinates (0.0 - 1.0)
// This allows ROIs to be resolution-agnostic and work across different stream qualities
type NormalizedRect struct {
	X      float64 `json:"x"`       // Top-left X (0.0 = left edge, 1.0 = right edge)
	Y      float64 `json:"y"`       // Top-left Y (0.0 = top edge, 1.0 = bottom edge)
	Width  float64 `json:"width"`   // Width as fraction of frame width
	Height float64 `json:"height"`  // Height as fraction of frame height
}

// ToPixels converts normalized coordinates to pixel coordinates for a given frame size
func (r *NormalizedRect) ToPixels(frameWidth, frameHeight int) PixelRect {
	return PixelRect{
		X:      int(r.X * float64(frameWidth)),
		Y:      int(r.Y * float64(frameHeight)),
		Width:  int(r.Width * float64(frameWidth)),
		Height: int(r.Height * float64(frameHeight)),
	}
}

// PixelRect represents a rectangle in pixel coordinates
type PixelRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Area returns the pixel area of the rectangle
func (r *PixelRect) Area() int {
	return r.Width * r.Height
}

// Clamp ensures the rectangle is within the given frame dimensions
func (r *PixelRect) Clamp(frameWidth, frameHeight int) {
	if r.X < 0 {
		r.X = 0
	}
	if r.Y < 0 {
		r.Y = 0
	}
	if r.X+r.Width > frameWidth {
		r.Width = frameWidth - r.X
	}
	if r.Y+r.Height > frameHeight {
		r.Height = frameHeight - r.Y
	}
}

// FrameMeta contains frame metadata without the raw data
type FrameMeta struct {
	Seq          uint64
	Timestamp    time.Time
	Width        int
	Height       int
	Format       string // "BGR24", "JPEG", etc.
	SourceStream string
}

// StreamStats contains stream statistics
type StreamStats struct {
	FrameCount   uint64
	FPSTarget    int
	FPSReal      float64
	LatencyMS    int64
	SourceStream string
	Resolution   string
	Reconnects   uint32
	BytesRead    uint64
	IsConnected  bool
	Errors       uint64
}

// ═══════════════════════════════════════════════════════════════════════════════
// AUTO-FOCUS TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Detection represents a single object detection with normalized coordinates
type Detection struct {
	// BBox is the bounding box in normalized coordinates [0.0, 1.0]
	BBox NormalizedRect `json:"bbox"`
	// Confidence is the detection confidence score [0.0, 1.0]
	Confidence float64 `json:"confidence"`
	// Class is the detected object class (e.g., "person")
	Class string `json:"class,omitempty"`
}

// DetectionHistory stores detections from a specific frame for auto-focus tracking
type DetectionHistory struct {
	// Timestamp when the frame was processed
	Timestamp time.Time
	// FrameSeq is the frame sequence number
	FrameSeq uint64
	// Detections are all objects detected in this frame
	Detections []Detection
}

// ToNormalizedRect converts a Detection to a NormalizedRect
func (d *Detection) ToNormalizedRect() NormalizedRect {
	return d.BBox
}
