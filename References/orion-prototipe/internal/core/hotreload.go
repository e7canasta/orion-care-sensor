package core

import (
	"fmt"
	"log/slog"
)

// updateConfig applies configuration changes without restarting
func (o *Orion) updateConfig(newConfig map[string]interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	slog.Info("applying config update", "changes", newConfig)

	changes := []string{}

	// Handle stream FPS update
	if streamCfg, ok := newConfig["stream"].(map[string]interface{}); ok {
		if fps, ok := streamCfg["fps"].(float64); ok {
			oldFPS := o.cfg.Stream.FPS
			newFPS := int(fps)

			if newFPS != oldFPS {
				o.cfg.Stream.FPS = newFPS
				changes = append(changes, fmt.Sprintf("stream.fps: %d → %d", oldFPS, newFPS))

				// Note: Stream mock doesn't support dynamic FPS change yet
				// In real implementation, we would recreate stream
				slog.Warn("stream FPS change requires restart (not implemented yet)",
					"old", oldFPS,
					"new", newFPS,
				)
			}
		}

		if resolution, ok := streamCfg["resolution"].(string); ok {
			oldRes := o.cfg.Stream.Resolution
			if resolution != oldRes {
				o.cfg.Stream.Resolution = resolution
				changes = append(changes, fmt.Sprintf("stream.resolution: %s → %s", oldRes, resolution))

				slog.Warn("stream resolution change requires restart (not implemented yet)",
					"old", oldRes,
					"new", resolution,
				)
			}
		}
	}

	// Handle models update
	// TODO: Implement hot-reload for new models config structure
	// For now, models hot-reload is disabled (requires worker restart)
	if _, ok := newConfig["models"].(map[string]interface{}); ok {
		slog.Warn("models config update detected, but hot-reload not implemented yet")
	}

	// Handle ROIs update
	if roisCfg, ok := newConfig["rois"].(map[string]interface{}); ok {
		if activeROIs, ok := roisCfg["active_rois"].([]interface{}); ok {
			rois := make([]string, 0)
			for _, r := range activeROIs {
				if roi, ok := r.(string); ok {
					rois = append(rois, roi)
				}
			}

			if len(rois) > 0 {
				oldROIs := o.cfg.ROIs.ActiveROIs
				o.cfg.ROIs.ActiveROIs = rois
				changes = append(changes, fmt.Sprintf("rois.active_rois: %v → %v", oldROIs, rois))

				slog.Info("rois config updated", "old", oldROIs, "new", rois)
			}
		}
	}

	if len(changes) == 0 {
		return fmt.Errorf("no valid configuration changes found")
	}

	slog.Info("config update applied", "changes_count", len(changes))

	for _, change := range changes {
		slog.Info("config changed", "change", change)
	}

	return nil
}
