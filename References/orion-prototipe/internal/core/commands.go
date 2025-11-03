package core

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/care/orion/internal/roiprocessor"
	"github.com/care/orion/internal/types"
)

// getStatus returns the current service status
func (o *Orion) getStatus() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	streamStats := o.stream.Stats()
	busStats := o.frameBus.Stats()
	emitterStats := o.emitter.Stats()

	// Worker stats
	workerStats := make([]map[string]interface{}, 0)
	for _, worker := range o.workers {
		workerStats = append(workerStats, map[string]interface{}{
			"id": worker.ID(),
		})
	}

	// Build configuration metadata
	config := map[string]interface{}{
		"stream": map[string]interface{}{
			"resolution": o.cfg.Stream.Resolution,
			"rtsp_url":   o.cfg.Camera.RTSPURL,
		},
		"models": map[string]interface{}{},
		"mqtt": map[string]interface{}{
			"broker":          o.cfg.MQTT.Broker,
			"control_topic":   o.cfg.MQTT.Topics.Control,
			"inference_topic": o.cfg.MQTT.Topics.Inferences,
		},
	}

	// Add person detector config if available
	if o.cfg.Models.PersonDetector != nil {
		config["models"].(map[string]interface{})["person_detector"] = map[string]interface{}{
			"model_path":            o.cfg.Models.PersonDetector.ModelPath,
			"size":                  o.cfg.Models.PersonDetector.Size,
			"confidence":            o.cfg.Models.PersonDetector.Confidence,
			"max_inference_rate_hz": o.cfg.Models.PersonDetector.MaxInferenceRateHz,
		}
	}

	// Add ROI config
	if len(o.cfg.ROIs.ActiveROIs) > 0 {
		config["rois"] = map[string]interface{}{
			"active": o.cfg.ROIs.ActiveROIs,
			"count":  len(o.cfg.ROIs.Definitions),
		}
	}

	// Build status response
	status := map[string]interface{}{
		"instance_id": o.cfg.InstanceID,
		"room_id":     o.cfg.RoomID,
		"uptime_s":    time.Since(o.started).Seconds(),
		"running":     o.isRunning,
		"paused":      o.isPaused,
		"stream": map[string]interface{}{
			"connected":   streamStats.IsConnected,
			"fps_real":    streamStats.FPSReal,
			"fps_target":  streamStats.FPSTarget,
			"frame_count": streamStats.FrameCount,
			"latency_ms":  streamStats.LatencyMS,
			"reconnects":  streamStats.Reconnects,
		},
		"framebus": map[string]interface{}{
			"workers_count":      busStats.WorkersCount,
			"frames_distributed": busStats.FramesDistributed,
			"dropped_by_worker":  busStats.DroppedByWorker,
		},
		"workers": workerStats,
		"emitter": map[string]interface{}{
			"connected": emitterStats.Connected,
			"published": emitterStats.Published,
			"errors":    emitterStats.Errors,
		},
		"config": config, // ← Metadata de configuración actual
	}

	return status
}

// pauseInference pauses inference processing
func (o *Orion) pauseInference() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.isPaused {
		return fmt.Errorf("already paused")
	}

	o.isPaused = true
	return nil
}

// resumeInference resumes inference processing
func (o *Orion) resumeInference() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.isPaused {
		return fmt.Errorf("not paused")
	}

	o.isPaused = false
	return nil
}

// isPausedCheck returns whether inference is paused
func (o *Orion) isPausedCheck() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.isPaused
}

// shutdownViaControl initiates graceful shutdown via MQTT control command
func (o *Orion) shutdownViaControl() error {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.isRunning {
		return fmt.Errorf("service not running")
	}

	if o.cancelCtx == nil {
		return fmt.Errorf("shutdown not available (no cancel context)")
	}

	// Trigger context cancellation - this will cause Run() to exit
	// and main.go will handle the graceful shutdown sequence
	o.cancelCtx()
	return nil
}

// setInferenceRate updates the target inference rate (Hz)
// This adjusts the GStreamer pipeline framerate by restarting the stream
//
// Implementation: Restarts the stream to avoid GStreamer caps renegotiation issues
// Trade-off: ~2s interruption for reliable operation (KISS approach)
func (o *Orion) setInferenceRate(rateHz float64) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if rateHz <= 0 || rateHz > 30 {
		return fmt.Errorf("invalid rate: %.2f (must be between 0.1 and 30 Hz)", rateHz)
	}

	oldRate := o.cfg.Models.PersonDetector.MaxInferenceRateHz
	slog.Info("updating inference rate with stream restart",
		"old_rate_hz", oldRate,
		"new_rate_hz", rateHz,
		"reason", "GStreamer caps renegotiation unreliable",
	)

	// Update config
	o.cfg.Models.PersonDetector.MaxInferenceRateHz = rateHz

	// Restart stream with new FPS (KISS approach)
	if streamWithFPS, ok := o.stream.(interface{ SetTargetFPS(float64) error }); ok {
		// Stop current stream (this kills consumeFrames goroutine when channel closes)
		slog.Info("stopping stream for FPS change")
		if err := o.stream.Stop(); err != nil {
			slog.Warn("failed to stop stream cleanly", "error", err)
		}

		// Update FPS configuration
		if err := streamWithFPS.SetTargetFPS(rateHz); err != nil {
			slog.Error("failed to update stream FPS config", "error", err)
			return fmt.Errorf("failed to update stream FPS: %w", err)
		}

		// Restart stream with new FPS
		slog.Info("restarting stream with new FPS", "rate_hz", rateHz)
		if err := o.stream.Start(o.runCtx); err != nil {
			slog.Error("failed to restart stream", "error", err)
			return fmt.Errorf("failed to restart stream: %w", err)
		}

		// Re-launch consumeFrames goroutine (killed when old stream channel closed)
		o.wg.Add(1)
		go o.consumeFrames(o.runCtx)
		slog.Info("consumeFrames goroutine restarted")

		slog.Info("stream restarted successfully with new FPS",
			"rate_hz", rateHz,
			"interruption_ms", "~2000",
		)
	} else {
		slog.Warn("stream does not support SetTargetFPS (mock stream?)")
	}

	slog.Info("inference rate updated successfully",
		"rate_hz", rateHz,
		"method", "stream_restart",
	)
	return nil
}

// setModelSize updates the YOLO model size (n/s/m/l/x)
func (o *Orion) setModelSize(size string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Validate model size
	validSizes := map[string]bool{"n": true, "s": true, "m": true, "l": true, "x": true}
	if !validSizes[size] {
		return fmt.Errorf("invalid model size: %s (must be n/s/m/l/x)", size)
	}

	// Get old config values
	oldSize := "unknown"
	oldPath := ""
	if o.cfg.Models.PersonDetector != nil {
		oldSize = o.cfg.Models.PersonDetector.Size
		oldPath = o.cfg.Models.PersonDetector.ModelPath
	}

	slog.Info("updating model size",
		"old_size", oldSize,
		"new_size", size,
	)

	// Find new model path (match Python worker logic)
	newModelPath, err := findModelPath(size)
	if err != nil {
		return fmt.Errorf("failed to find model for size %s: %w", size, err)
	}

	// Update config FIRST (so get_status reflects the change immediately)
	if o.cfg.Models.PersonDetector != nil {
		o.cfg.Models.PersonDetector.Size = size
		o.cfg.Models.PersonDetector.ModelPath = newModelPath
	}

	// Update Python worker model (hot-swap)
	for _, worker := range o.workers {
		if pythonWorker, ok := worker.(interface{ SetModelSize(string) error }); ok {
			if err := pythonWorker.SetModelSize(size); err != nil {
				slog.Error("failed to update model size", "error", err)
				// Rollback config changes
				if o.cfg.Models.PersonDetector != nil {
					o.cfg.Models.PersonDetector.Size = oldSize
					o.cfg.Models.PersonDetector.ModelPath = oldPath
				}
				return err
			}
		}
	}

	slog.Info("model size updated successfully",
		"size", size,
		"model_path", newModelPath,
	)
	return nil
}

// findModelPath finds the ONNX model file for a given size
// Matches Python worker logic with glob patterns
func findModelPath(size string) (string, error) {
	patterns := []string{
		fmt.Sprintf("models/yolo11%s.onnx", size),
		fmt.Sprintf("models/yolo11%s_*.onnx", size),
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			return matches[0], nil
		}
	}

	return "", fmt.Errorf("no model found for size '%s' (tried patterns: %v)", size, patterns)
}

// setAttentionROIs updates the ROI attention regions for intelligent model selection
func (o *Orion) setAttentionROIs(rois []map[string]interface{}) error {
	// Convert map to NormalizedRect
	normalizedROIs := make([]types.NormalizedRect, 0, len(rois))

	for _, roiMap := range rois {
		x, okX := roiMap["x"].(float64)
		y, okY := roiMap["y"].(float64)
		width, okW := roiMap["width"].(float64)
		height, okH := roiMap["height"].(float64)

		if !okX || !okY || !okW || !okH {
			return fmt.Errorf("invalid ROI format: expected {x, y, width, height} as floats")
		}

		normalizedROIs = append(normalizedROIs, types.NormalizedRect{
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		})
	}

	// Validate ROIs
	if err := roiprocessor.ValidateROIs(normalizedROIs); err != nil {
		return fmt.Errorf("invalid ROIs: %w", err)
	}

	// Update ROI processor
	o.roiProcessor.SetAttentionROIs(normalizedROIs)

	slog.Info("attention ROIs updated",
		"num_rois", len(normalizedROIs),
		"rois", normalizedROIs,
	)

	return nil
}

// clearAttentionROIs removes all attention ROIs (full frame processing)
func (o *Orion) clearAttentionROIs() error {
	o.roiProcessor.ClearAttentionROIs()
	slog.Info("attention ROIs cleared - full frame processing enabled")
	return nil
}

// getAttentionROIs returns current attention ROIs and processor stats
func (o *Orion) getAttentionROIs() map[string]interface{} {
	rois := o.roiProcessor.GetAttentionROIs()
	stats := o.roiProcessor.Stats()

	return map[string]interface{}{
		"attention_rois":    rois,
		"num_rois":          len(rois),
		"stats": map[string]interface{}{
			"rois_processed":     stats.ROIsProcessed,
			"model_320_selected": stats.Model320Selected,
			"model_640_selected": stats.Model640Selected,
			"active_rois":        stats.ActiveROIs,
		},
	}
}

// setAutoFocusStrategy changes the auto-focus strategy at runtime
// Valid strategies: "simple", "smoothing", "velocity"
func (o *Orion) setAutoFocusStrategy(params map[string]interface{}) error {
	// Extract strategy
	strategyStr, ok := params["strategy"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid 'strategy' parameter (expected string)")
	}

	// Convert string to AutoFocusStrategy type
	var strategy roiprocessor.AutoFocusStrategy
	switch strategyStr {
	case "simple":
		strategy = roiprocessor.StrategySimple
	case "smoothing":
		strategy = roiprocessor.StrategySmoothing
	case "velocity":
		strategy = roiprocessor.StrategyVelocity
	default:
		return fmt.Errorf("invalid strategy: %s (valid: simple, smoothing, velocity)", strategyStr)
	}

	// Extract strategy-specific parameters
	strategyParams := make(map[string]float64)

	if alpha, ok := params["alpha"].(float64); ok {
		strategyParams["alpha"] = alpha
	}
	if threshold, ok := params["velocity_threshold"].(float64); ok {
		strategyParams["velocity_threshold"] = threshold
	}
	if factor, ok := params["velocity_factor"].(float64); ok {
		strategyParams["velocity_factor"] = factor
	}
	if maxExp, ok := params["max_expansion"].(float64); ok {
		strategyParams["max_expansion"] = maxExp
	}

	// Update ROI processor strategy
	if err := o.roiProcessor.SetAutoFocusStrategy(strategy, strategyParams); err != nil {
		return fmt.Errorf("failed to set auto-focus strategy: %w", err)
	}

	slog.Info("auto-focus strategy updated via MQTT",
		"strategy", strategyStr,
		"params", strategyParams,
	)

	return nil
}

// getAutoFocusStrategy returns current auto-focus strategy and configuration
func (o *Orion) getAutoFocusStrategy() map[string]interface{} {
	strategy := o.roiProcessor.GetAutoFocusStrategy()
	stats := o.roiProcessor.Stats()

	// Build strategy-specific params
	strategyParams := map[string]interface{}{
		"smoothing_alpha":    stats.SmoothingAlpha,
		"velocity_threshold": stats.VelocityParams.Threshold,
		"velocity_factor":    stats.VelocityParams.Factor,
		"max_expansion":      stats.VelocityParams.MaxExpansion,
	}

	// Build strategy applied counts
	appliedCounts := make(map[string]uint64)
	for strat, count := range stats.StrategyApplied {
		appliedCounts[string(strat)] = count
	}

	return map[string]interface{}{
		"strategy":         string(strategy),
		"params":           strategyParams,
		"strategy_applied": appliedCounts,
		"auto_focus_enabled": o.roiProcessor.AutoFocusEnabled(),
	}
}

// enableAutoFocus enables the auto-focus tracking system
func (o *Orion) enableAutoFocus() error {
	o.roiProcessor.EnableAutoFocus()
	slog.Info("auto-focus enabled via MQTT")
	return nil
}

// disableAutoFocus disables the auto-focus tracking system
func (o *Orion) disableAutoFocus() error {
	o.roiProcessor.DisableAutoFocus()
	slog.Info("auto-focus disabled via MQTT")
	return nil
}

// clearAutoFocusHistory clears the auto-focus detection history
func (o *Orion) clearAutoFocusHistory() error {
	o.roiProcessor.ClearAutoFocusHistory()
	slog.Info("auto-focus history cleared via MQTT")
	return nil
}

// getAutoFocusStats returns detailed auto-focus statistics
func (o *Orion) getAutoFocusStats() map[string]interface{} {
	stats := o.roiProcessor.AutoFocusStats()
	processorStats := o.roiProcessor.Stats()
	suggestedROI := o.roiProcessor.GetSuggestedROI()

	response := map[string]interface{}{
		"enabled": stats.Enabled,
		"stats": map[string]interface{}{
			"auto_focus_frames":   stats.AutoFocusFrames,
			"history_hits":        stats.HistoryHits,
			"history_misses":      stats.HistoryMisses,
			"total_detections":    stats.TotalDetections,
			"current_detections":  stats.CurrentDetections,
			"history_frames":      stats.HistoryFrames,
			"history_size":        stats.HistorySize,
			"expansion_pct":       stats.ExpansionPct,
		},
		"strategy": map[string]interface{}{
			"current": string(processorStats.CurrentStrategy),
			"applied": processorStats.StrategyApplied,
		},
	}

	// Add suggested ROI if available
	if suggestedROI != nil {
		response["suggested_roi"] = map[string]interface{}{
			"x":      suggestedROI.X,
			"y":      suggestedROI.Y,
			"width":  suggestedROI.Width,
			"height": suggestedROI.Height,
		}
	}

	return response
}
