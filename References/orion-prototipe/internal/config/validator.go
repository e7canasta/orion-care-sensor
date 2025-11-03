package config

import (
	"fmt"
	"regexp"
)

var instanceIDPattern = regexp.MustCompile(`^[a-z0-9\-]+$`)

// Validate checks if the configuration is valid
func Validate(cfg *Config) error {
	// Validate instance_id
	if cfg.InstanceID == "" {
		return fmt.Errorf("instance_id is required")
	}
	if !instanceIDPattern.MatchString(cfg.InstanceID) {
		return fmt.Errorf("instance_id must match pattern [a-z0-9-]+")
	}

	// Validate room_id
	if cfg.RoomID == "" {
		return fmt.Errorf("room_id is required")
	}

	// Validate stream config
	if cfg.Stream.FPS <= 0 {
		return fmt.Errorf("stream.fps must be > 0")
	}
	if cfg.Stream.BufferFrames <= 0 {
		cfg.Stream.BufferFrames = 30 // default
	}

	// Validate MQTT broker
	if cfg.MQTT.Broker == "" {
		return fmt.Errorf("mqtt.broker is required")
	}

	// Set default topics if not provided
	if cfg.MQTT.Topics.Control == "" {
		cfg.MQTT.Topics.Control = fmt.Sprintf("care/control/%s", cfg.InstanceID)
	}
	if cfg.MQTT.Topics.Inferences == "" {
		cfg.MQTT.Topics.Inferences = fmt.Sprintf("care/inferences/%s", cfg.InstanceID)
	}
	if cfg.MQTT.Topics.Health == "" {
		cfg.MQTT.Topics.Health = fmt.Sprintf("care/health/%s", cfg.InstanceID)
	}

	// Set default QoS if not provided
	if cfg.MQTT.QoS == nil {
		cfg.MQTT.QoS = map[string]byte{
			"control":          1,
			"person_detection": 1,
			"pose_keypoints":   1,
			"flow_roi":         0,
			"health":           0,
		}
	}

	// Validate ROIs
	if err := ValidateROIs(cfg.ROIs); err != nil {
		return fmt.Errorf("roi validation failed: %w", err)
	}

	return nil
}

// ValidateROIs validates ROI configuration for correctness
func ValidateROIs(rois ROIsConfig) error {
	// Validate each ROI definition
	for name, roi := range rois.Definitions {
		switch roi.Type {
		case "polygon":
			if len(roi.Coords) < 3 {
				return fmt.Errorf("ROI '%s': polygon must have at least 3 points, got %d",
					name, len(roi.Coords))
			}

			// Validate each coordinate is [x, y]
			for i, coord := range roi.Coords {
				if len(coord) != 2 {
					return fmt.Errorf("ROI '%s': point %d must be [x,y], got %v",
						name, i, coord)
				}
			}

		case "rect":
			if len(roi.Coords) != 2 {
				return fmt.Errorf("ROI '%s': rect must have 2 points [top-left, bottom-right], got %d",
					name, len(roi.Coords))
			}

			// Validate each coordinate is [x, y]
			for i, coord := range roi.Coords {
				if len(coord) != 2 {
					return fmt.Errorf("ROI '%s': point %d must be [x,y], got %v",
						name, i, coord)
				}
			}

		case "":
			// Empty type is allowed (ROI exists but not configured yet)
			// This allows gradual configuration without breaking startup
			continue

		default:
			return fmt.Errorf("ROI '%s': unknown type '%s' (must be 'polygon' or 'rect')",
				name, roi.Type)
		}
	}

	// Validate active ROIs reference existing definitions
	for _, activeROI := range rois.ActiveROIs {
		if _, exists := rois.Definitions[activeROI]; !exists {
			return fmt.Errorf("active ROI '%s' not found in definitions", activeROI)
		}
	}

	return nil
}
