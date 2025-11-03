package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the complete Orion configuration
type Config struct {
	InstanceID         string       `yaml:"instance_id"`
	RoomID             string       `yaml:"room_id"`
	ShutdownTimeoutS   int          `yaml:"shutdown_timeout_s"` // Graceful shutdown timeout in seconds (default: 5)
	Camera             CameraConfig `yaml:"camera"`
	Stream             StreamConfig `yaml:"stream"`
	Models             ModelsConfig `yaml:"models"`
	ROIs               ROIsConfig   `yaml:"rois"`
	MQTT               MQTTConfig   `yaml:"mqtt"`
}

// CameraConfig contains camera settings
type CameraConfig struct {
	RTSPURL     string `yaml:"rtsp_url"`
	PointOfView string `yaml:"point_of_view"` // overhead, bedside, door
}

// StreamConfig contains stream processing settings
type StreamConfig struct {
	Resolution       string `yaml:"resolution"`         // 512p, 720p, 1080p
	FPS              int    `yaml:"fps"`                // target fps
	BufferFrames     int    `yaml:"buffer_frames"`      // circular buffer size
	WarmupDurationS  int    `yaml:"warmup_duration_s"`  // warm-up duration in seconds
}

// ModelsConfig contains AI model settings
type ModelsConfig struct {
	PersonDetector *ModelConfig `yaml:"person_detector,omitempty"`
	PoseEstimator  *ModelConfig `yaml:"pose_estimator,omitempty"`
}

// ModelConfig defines a single model configuration
type ModelConfig struct {
	ModelPath           string  `yaml:"model_path"`
	ModelPath320        string  `yaml:"model_path_320"`         // Secondary model for ROI attention (optional)
	Size                string  `yaml:"size"`                    // Model size: n/s/m/l/x (for YOLO models)
	Confidence          float64 `yaml:"confidence"`
	MaxInferenceRateHz  float64 `yaml:"max_inference_rate_hz"`  // Maximum inferences per second (e.g., 1.0)
	ProcessInterval     int     `yaml:"process_interval"`        // Calculated automatically or manual override
	MultiModelEnabled   bool    `yaml:"multi_model_enabled"`     // Enable multi-model ROI attention (auto-detect if model_path_320 is set)
}

// ROIsConfig contains ROI definitions
type ROIsConfig struct {
	ActiveROIs  []string               `yaml:"active_rois"`
	Definitions map[string]ROIDefinition `yaml:"definitions"`
	// ROI Attention thresholds for multi-model selection
	Threshold320 int `yaml:"threshold_320"` // Max area (pixels) for 320 model (default: 102400)
	Threshold640 int `yaml:"threshold_640"` // Min area (pixels) for 640 model (default: 409600)
}

// ROIDefinition defines a single ROI
type ROIDefinition struct {
	Type   string      `yaml:"type"`   // polygon, rect
	Coords [][]int     `yaml:"coords"` // [[x1,y1], [x2,y2], ...]
	Parent string      `yaml:"parent,omitempty"`
}

// MQTTConfig contains MQTT broker settings
type MQTTConfig struct {
	Broker   string          `yaml:"broker"`
	Topics   MQTTTopics      `yaml:"topics"`
	QoS      map[string]byte `yaml:"qos"`
}

// MQTTTopics contains topic templates
type MQTTTopics struct {
	Control    string `yaml:"control"`
	Inferences string `yaml:"inferences"`
	Health     string `yaml:"health"`
}

// Load reads and parses a YAML configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}
