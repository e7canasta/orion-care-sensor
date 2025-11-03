package types

import (
	"encoding/json"
	"time"
)

// Inference is the interface that all inference types must implement
type Inference interface {
	// Type returns the inference type (person_detection, pose_keypoints, etc.)
	Type() string
	// Timestamp returns when the inference was generated
	Timestamp() time.Time
	// ToJSON converts the inference to JSON bytes
	ToJSON() ([]byte, error)
	// SuggestedROI returns Python's suggested ROI for next frame (hybrid auto-focus)
	// Returns nil if inference doesn't support auto-focus suggestions
	SuggestedROI() *NormalizedRect
}

// BBox represents a bounding box
type BBox struct {
	X1         int     `json:"x1"`
	Y1         int     `json:"y1"`
	X2         int     `json:"x2"`
	Y2         int     `json:"y2"`
	CenterX    int     `json:"center_x"`
	CenterY    int     `json:"center_y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Confidence float64 `json:"confidence"`
}

// PersonDetection represents a person detection inference
type PersonDetection struct {
	InstanceID   string             `json:"instance_id"`
	RoomID       string             `json:"room_id"`
	InferenceTyp string             `json:"inference_type"`
	SourceStream string             `json:"source_stream"`
	Model        string             `json:"model"`
	BBox         BBox               `json:"bbox"`
	ROIOverlap   map[string]float64 `json:"roi_overlap"`
	Metadata     InferenceMetadata  `json:"metadata"`
	TimestampStr string             `json:"timestamp"`
	ts           time.Time
}

// Type implements Inference interface
func (p *PersonDetection) Type() string {
	return "person_detection"
}

// Timestamp implements Inference interface
func (p *PersonDetection) Timestamp() time.Time {
	return p.ts
}

// ToJSON implements Inference interface
func (p *PersonDetection) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// SuggestedROI implements Inference interface (legacy type, no support)
func (p *PersonDetection) SuggestedROI() *NormalizedRect {
	return nil
}

// Keypoint represents a single pose keypoint
type Keypoint struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Confidence float64 `json:"confidence"`
}

// PoseKeypoints represents a pose estimation inference
type PoseKeypoints struct {
	InstanceID    string                 `json:"instance_id"`
	RoomID        string                 `json:"room_id"`
	InferenceTyp  string                 `json:"inference_type"`
	SourceStream  string                 `json:"source_stream"`
	Model         string                 `json:"model"`
	ROIID         string                 `json:"roi_id"`
	Keypoints     map[string]Keypoint    `json:"keypoints"`
	DerivedMetric map[string]interface{} `json:"derived_metrics"`
	Metadata      InferenceMetadata      `json:"metadata"`
	TimestampStr  string                 `json:"timestamp"`
	ts            time.Time
}

// Type implements Inference interface
func (p *PoseKeypoints) Type() string {
	return "pose_keypoints"
}

// Timestamp implements Inference interface
func (p *PoseKeypoints) Timestamp() time.Time {
	return p.ts
}

// ToJSON implements Inference interface
func (p *PoseKeypoints) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// SuggestedROI implements Inference interface (no support for pose estimation)
func (p *PoseKeypoints) SuggestedROI() *NormalizedRect {
	return nil
}

// InferenceMetadata contains common metadata for all inferences
type InferenceMetadata struct {
	ProcessingTimeMs float64 `json:"processing_time_ms"`
	FrameWidth       int     `json:"frame_width"`
	FrameHeight      int     `json:"frame_height"`
	// Model inference metadata (Python → Go → MQTT)
	ImgSize    string `json:"img_size"`     // Size of the frame that was inferenced (e.g., "640x640")
	ModelSize  string `json:"model_size"`   // Model size used: n/s/m/l/x
	ImgSizeInt int    `json:"imgsz"`        // Integer size for compiled weights (320 or 640)
}
