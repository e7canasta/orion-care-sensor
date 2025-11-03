/*
═══════════════════════════════════════════════════════════════════════════════
PYTHON PERSON DETECTOR WORKER - PSEUDOCODE OVERVIEW
═══════════════════════════════════════════════════════════════════════════════

PURPOSE:
  Manages a Python subprocess that runs ONNX inference for person detection.
  Bridges Go and Python: sends frames via stdin, receives results via stdout.

ARCHITECTURE:
  ┌─────────────┐  frames  ┌──────────────┐  stdin   ┌────────────────┐
  │  Orion Core │ ───────> │ Go Worker    │ ───────> │ Python Process │
  │   (Go)      │          │ (this file)  │          │  person_det.py │
  └─────────────┘          └──────────────┘          └────────────────┘
                                  ^                          │
                                  │ stdout (JSON)            │
                                  └──────────────────────────┘

DATA FLOW:
  1. Orion Core → SendFrame() → input channel (buffered, size 5)
  2. processFrames() goroutine → reads from input channel
  3. sendFrame() → encodes frame to base64 JSON → writes to Python stdin
  4. Python process → runs ONNX inference → outputs JSON to stdout
  5. readResults() goroutine → parses JSON → sends to results channel (size 10)
  6. Orion Core → reads from Results() channel → emits to MQTT

KEY GOROUTINES:
  [1] processFrames()   - Reads frames from input channel, sends to Python
  [2] readResults()     - Reads inference results from Python stdout
  [3] logStderr()       - Logs Python process stderr output
  [4] waitProcess()     - Waits for Python process exit (prevent zombies)

BACKPRESSURE HANDLING:
  - Input channel (5 slots): If full → drop frame (increment framesDropped)
  - Results channel (10 slots): If full → drop inference (log warning)
  - sendFrame() timeout: 2s to prevent blocking on hung Python process
  - No rate limiting here: GStreamer videorate controls upstream frame rate

MULTI-MODEL SUPPORT (ROI Attention):
  - Primary model (640x640): Used when frame.ROIProcessing.TargetSize = 640
  - Secondary model (320x320): Used when frame.ROIProcessing.TargetSize = 320
  - Python worker selects model based on "roi_processing.target_size" in metadata
  - Both models loaded at startup if modelPath320 configured

HOT-RELOAD:
  - SetModelSize(size string) → sends JSON command to Python stdin
  - Python worker reloads model without restarting process
  - Valid sizes: n/s/m/l/x (e.g., yolo11n, yolo11s, yolo11m, etc.)

ERROR RECOVERY:
  - Frame send failure → log error, continue processing (don't crash)
  - JSON parse error → log truncated output, continue reading
  - Python process exit → logged by waitProcess(), Orion monitors health
  - Stop timeout (2s) → force kill Python process

LIFECYCLE:
  CREATE:
    worker = NewPythonPersonDetector(config)
      → validates config (modelPath required)
      → creates input/results channels
      → stores model paths (640 and optional 320)

  START:
    worker.Start(ctx)
      → spawnPythonProcess()
          • exec.CommandContext("models/run_worker.sh", args...)
          • setup stdin/stdout/stderr pipes
          • cmd.Start()
      → launch goroutines: [1] processFrames, [2] readResults,
                           [3] logStderr, [4] waitProcess
      → set isActive = true

  RUNNING:
    [Core] → SendFrame(frame) → input channel
    [1] processFrames:
          LOOP:
            frame = <-input channel
            sendFrame(frame):
              • encode frame.Data to base64
              • build JSON: {frame_data, width, height, meta:{seq, roi_processing}}
              • write to stdin with 2s timeout

    [Python subprocess]:
          READ stdin → decode base64 → run ONNX inference → WRITE stdout JSON

    [2] readResults:
          LOOP:
            line = scanner.Scan(stdout)
            json.Unmarshal(line) → result
            inference = PersonDetectionInference{
              type: "person_detection",
              data: result["data"],      // detections, count, etc.
              timing: result["timing"]   // total_ms, inference_ms, etc.
            }
            results channel <- inference (non-blocking)
            update metrics (inferenceCount, totalLatencyMS, lastSeenAt)

    [3] logStderr:
          LOOP:
            line = scanner.Scan(stderr)
            map Python log level → slog:
              [ERROR]/[CRITICAL] → slog.Error
              [WARNING]/[WARN]   → slog.Warn
              [INFO]/[DEBUG]     → slog.Debug

    [4] waitProcess:
          cmd.Wait() → blocks until Python process exits
          IF err:
            IF ctx.Done() → expected shutdown (log debug)
            ELSE → unexpected crash (log error)

  METRICS:
    Metrics() → returns WorkerMetrics:
      • framesProcessed: atomic counter (total frames received)
      • framesDropped: input channel full drops
      • inferencesEmitted: successful results sent
      • avgLatencyMS: totalLatencyMS / inferencesEmitted
      • lastSeenAt: timestamp of last inference (for health checks)

  STOP:
    worker.Stop()
      → cancel context (triggers all goroutines to exit)
      → close stdin (signals Python to exit gracefully)
      → wait for goroutines with 2s timeout:
          IF timeout → force kill Python process (cmd.Process.Kill())
      → close input/results channels
      → set isActive = false

JSON PROTOCOL (stdin → Python):
  {
    "frame_data": "<base64 encoded jpeg>",
    "width": 1280,
    "height": 720,
    "meta": {
      "instance_id": "node_001",
      "room_id": "room_A",
      "seq": 12345,
      "timestamp": "2025-10-09T14:30:00.123456789Z",
      "roi_processing": {            // Optional, for multi-model selection
        "target_size": 320,          // Tells Python which model to use
        "crop_applied": false,
        "num_rois": 2
      }
    }
  }

JSON PROTOCOL (Python → stdout):
  {
    "data": {
      "detections": [...],           // YOLO detections
      "person_count": 2,
      "confidence_threshold": 0.5,
      "frame_seq": 12345
    },
    "timing": {
      "total_ms": 45.2,
      "inference_ms": 42.1,
      "preprocessing_ms": 1.5,
      "postprocessing_ms": 1.6
    }
  }

CONTROL COMMANDS (hot-reload):
  {
    "type": "command",
    "command": "set_model_size",
    "params": {
      "size": "m"                    // n/s/m/l/x
    }
  }

THREAD SAFETY:
  - atomic.Bool for isActive
  - atomic.Uint64 for counters (frameCount, inferenceCount, etc.)
  - atomic.Value for lastSeenAt timestamp
  - sync.WaitGroup for goroutine lifecycle
  - context.Context for cancellation propagation

PERFORMANCE CHARACTERISTICS:
  - Zero-copy: frames passed by reference through channels
  - Non-blocking sends: dropped frames/inferences if channels full
  - GStreamer controls rate: no rate limiting in Go worker
  - Concurrent: 4 goroutines + Python subprocess
  - Timeout protection: 2s for stdin writes (prevent hangs)

DEPENDENCIES:
  - Python subprocess: models/run_worker.sh (activates venv, runs person_detector.py)
  - ONNX models: yolo11{n,s,m,l,x}_fp32_{320,640}.onnx
  - Config: model paths, confidence threshold, worker ID

INTEGRATION POINTS:
  - types.InferenceWorker interface: ID(), SendFrame(), Start(), Stop(), Results(), Metrics()
  - types.Frame: contains Data (JPEG), Width, Height, Seq, Timestamp, ROIProcessing
  - types.Inference: implemented by PersonDetectionInference

FAILURE MODES:
  1. Python crash → waitProcess logs error, Orion health check fails
  2. stdin write timeout → log error, continue (frame dropped)
  3. JSON parse error → log error, skip result, continue
  4. Channel full → drop frame/inference, increment counters
  5. Stop timeout → force kill Python process

═══════════════════════════════════════════════════════════════════════════════
*/

package worker

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/care/orion/internal/types"
	"github.com/vmihailenco/msgpack/v5"
)

// PythonPersonDetector wraps a Python ONNX worker process
type PythonPersonDetector struct {
	id           string
	modelPath    string
	modelPath320 string // Secondary model for ROI attention
	confidence   float64

	// Python process
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	stderrBuf *bufio.Reader

	// Worker interface channels
	input   chan types.Frame
	results chan types.Inference

	// Lifecycle
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	isActive atomic.Bool

	// Stats
	frameCount     uint64
	inferenceCount uint64
	totalLatencyMS uint64
	framesDropped  uint64
	lastSeenAt     atomic.Value // time.Time

	// Metadata
	instanceID string
	roomID     string
}

// PythonPersonDetectorConfig contains configuration for Python worker
type PythonPersonDetectorConfig struct {
	WorkerID     string
	ModelPath    string
	ModelPath320 string // Secondary model for ROI attention (optional)
	Confidence   float64
	InstanceID   string
	RoomID       string
}

// NewPythonPersonDetector creates a new Python person detector worker
func NewPythonPersonDetector(cfg PythonPersonDetectorConfig) (*PythonPersonDetector, error) {
	if cfg.ModelPath == "" {
		return nil, fmt.Errorf("model_path is required")
	}

	if cfg.Confidence <= 0 {
		cfg.Confidence = 0.5 // Default threshold
	}

	w := &PythonPersonDetector{
		id:           cfg.WorkerID,
		modelPath:    cfg.ModelPath,
		modelPath320: cfg.ModelPath320,
		confidence:   cfg.Confidence,
		input:        make(chan types.Frame, 5),
		results:      make(chan types.Inference, 10),
		instanceID:   cfg.InstanceID,
		roomID:       cfg.RoomID,
	}

	multiModelMode := ""
	if cfg.ModelPath320 != "" {
		multiModelMode = " (multi-model ROI attention enabled)"
	}

	slog.Info("python person detector worker created"+multiModelMode,
		"worker_id", cfg.WorkerID,
		"model_640", cfg.ModelPath,
		"model_320", cfg.ModelPath320,
		"confidence", cfg.Confidence,
	)

	return w, nil
}

// ID returns the worker ID
func (w *PythonPersonDetector) ID() string {
	return w.id
}

// SendFrame sends a frame to the worker (non-blocking, implements types.InferenceWorker)
func (w *PythonPersonDetector) SendFrame(frame types.Frame) (err error) {
	// Protect against panic from sending to closed channel during worker restart
	defer func() {
		if r := recover(); r != nil {
			atomic.AddUint64(&w.framesDropped, 1)
			err = fmt.Errorf("worker channel closed (restart in progress)")
		}
	}()

	// Check if worker is active before attempting send
	if !w.isActive.Load() {
		atomic.AddUint64(&w.framesDropped, 1)
		return fmt.Errorf("worker not active")
	}

	select {
	case w.input <- frame:
		return nil
	default:
		// Channel full, frame dropped
		atomic.AddUint64(&w.framesDropped, 1)
		return fmt.Errorf("worker input buffer full")
	}
}

// Start starts the Python worker process and goroutines
func (w *PythonPersonDetector) Start(ctx context.Context) error {
	if w.isActive.Load() {
		return fmt.Errorf("worker already started")
	}

	// Recreate channels if they were closed by a previous Stop()
	// This is essential for auto-recovery restarts
	w.input = make(chan types.Frame, 5)
	w.results = make(chan types.Inference, 10)

	w.ctx, w.cancel = context.WithCancel(ctx)

	// Spawn Python process
	if err := w.spawnPythonProcess(); err != nil {
		return fmt.Errorf("failed to spawn python process: %w", err)
	}

	w.isActive.Store(true)

	// Initialize lastSeenAt
	w.lastSeenAt.Store(time.Now())

	// Start frame processing goroutine
	w.wg.Add(1)
	go w.processFrames()

	// Start stderr logger goroutine
	w.wg.Add(1)
	go w.logStderr()

	slog.Info("python person detector started",
		"worker_id", w.id,
		"model", w.modelPath,
		"confidence", w.confidence,
	)

	return nil
}

// spawnPythonProcess starts the Python worker subprocess
func (w *PythonPersonDetector) spawnPythonProcess() error {
	// Build command: use wrapper script to activate venv
	// models/run_worker.sh --model <path> --confidence <threshold> [--model-320 <path>]
	args := []string{
		"--model", w.modelPath,
		"--confidence", fmt.Sprintf("%.2f", w.confidence),
	}

	// Add secondary model if configured for multi-model ROI attention
	if w.modelPath320 != "" {
		args = append(args, "--model-320", w.modelPath320)
	}

	w.cmd = exec.CommandContext(
		w.ctx,
		"models/run_worker.sh",
		args...,
	)

	// Setup stdin pipe
	stdin, err := w.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	w.stdin = stdin

	// Setup stdout pipe
	stdout, err := w.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	w.stdout = stdout

	// Setup stderr pipe
	stderr, err := w.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	w.stderr = stderr
	w.stderrBuf = bufio.NewReader(stderr)

	// Start process
	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start python process: %w", err)
	}

	slog.Info("python process spawned",
		"worker_id", w.id,
		"pid", w.cmd.Process.Pid,
	)

	// Start stdout reader goroutine
	w.wg.Add(1)
	go w.readResults()

	// Start process waiter goroutine to prevent zombies
	w.wg.Add(1)
	go w.waitProcess()

	return nil
}

// processFrames reads frames from input channel and sends to Python worker
// SIMPLIFIED: No rate limiting here - GStreamer controls frame rate at the source
func (w *PythonPersonDetector) processFrames() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return

		case frame, ok := <-w.input:
			if !ok {
				return
			}

			atomic.AddUint64(&w.frameCount, 1)

			// Check context before processing (allows faster shutdown)
			select {
			case <-w.ctx.Done():
				slog.Debug("dropping frame due to context cancellation",
					"worker_id", w.id,
					"frame_seq", frame.Seq,
					"trace_id", frame.TraceID,
				)
				return
			default:
				// Context still active, proceed with frame processing
			}

			// Send every frame to Python worker
			// Rate limiting is done upstream by GStreamer videorate
			if err := w.sendFrame(frame); err != nil {
				slog.Error("failed to send frame to python worker",
					"worker_id", w.id,
					"frame_seq", frame.Seq,
					"trace_id", frame.TraceID,
					"error", err,
					"frames_in_flight", len(w.input),
					"action", "worker may be hung, check health metrics")
				// Continue processing (don't crash on single frame failure)
			}
		}
	}
}

// sendFrame sends a frame to the Python worker via stdin with timeout
// Uses MsgPack for efficient binary serialization (no base64 overhead)
func (w *PythonPersonDetector) sendFrame(frame types.Frame) error {
	// Build metadata
	meta := map[string]interface{}{
		"instance_id": w.instanceID,
		"room_id":     w.roomID,
		"seq":         frame.Seq,
		"timestamp":   frame.Timestamp.Format(time.RFC3339Nano),
	}

	// Include ROI processing metadata if available (for multi-model selection)
	// Python will echo this back so we can attach it to the inference
	if frame.ROIProcessing != nil {
		meta["roi_processing"] = frame.ROIProcessing
	}

	// Build request with raw bytes (NO base64 encoding!)
	request := map[string]interface{}{
		"frame_data": frame.Data, // Raw []byte, MsgPack handles this natively
		"width":      frame.Width,
		"height":     frame.Height,
		"meta":       meta,
	}

	// Marshal to MsgPack (5x faster than JSON + base64)
	msgpackBytes, err := msgpack.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal msgpack request: %w", err)
	}

	// Write to stdin with length-prefix framing (4 bytes big-endian + msgpack data)
	// This ensures Python can detect message boundaries in the stream
	writeErr := make(chan error, 1)
	go func() {
		// Write length prefix (4 bytes, big endian)
		lengthPrefix := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthPrefix, uint32(len(msgpackBytes)))

		if _, err := w.stdin.Write(lengthPrefix); err != nil {
			writeErr <- fmt.Errorf("failed to write length prefix: %w", err)
			return
		}

		// Write msgpack data
		if _, err := w.stdin.Write(msgpackBytes); err != nil {
			writeErr <- fmt.Errorf("failed to write msgpack data: %w", err)
			return
		}

		writeErr <- nil
	}()

	// Wait for write completion or timeout
	select {
	case err := <-writeErr:
		if err != nil {
			return fmt.Errorf("failed to write to stdin: %w", err)
		}
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("stdin write timeout (python worker may be hung)")
	case <-w.ctx.Done():
		return fmt.Errorf("worker context cancelled during write")
	}
}

// readResults reads inference results from Python worker stdout
// Uses MsgPack with length-prefix framing for message boundaries
func (w *PythonPersonDetector) readResults() {
	defer w.wg.Done()

	lengthBuf := make([]byte, 4)

	for {
		// Read length prefix (4 bytes, big-endian)
		if _, err := io.ReadFull(w.stdout, lengthBuf); err != nil {
			if err == io.EOF {
				// Stream closed gracefully (Python worker exited)
				slog.Debug("python worker stdout closed (EOF)",
					"worker_id", w.id,
				)
				return
			}

			slog.Error("failed to read length prefix from python worker",
				"worker_id", w.id,
				"error", err,
			)
			return
		}

		// Decode length
		msgLength := binary.BigEndian.Uint32(lengthBuf)

		// Read exact msgpack message
		msgpackData := make([]byte, msgLength)
		if _, err := io.ReadFull(w.stdout, msgpackData); err != nil {
			slog.Error("failed to read msgpack data from python worker",
				"worker_id", w.id,
				"error", err,
				"expected_length", msgLength,
			)
			return
		}

		// Unmarshal msgpack
		var result map[string]interface{}
		if err := msgpack.Unmarshal(msgpackData, &result); err != nil {
			slog.Error("failed to unmarshal msgpack inference result",
				"worker_id", w.id,
				"error", err,
				"data_length", len(msgpackData),
				"action", "check python worker logs in stderr")
			continue
		}

		// Extract suggested ROI from Python (hybrid auto-focus)
		var suggestedROI *types.NormalizedRect
		if suggestedData, ok := result["suggested_roi"].(map[string]interface{}); ok && suggestedData != nil {
			// Convert Python dict to NormalizedRect
			suggestedROI = &types.NormalizedRect{
				X:      suggestedData["x"].(float64),
				Y:      suggestedData["y"].(float64),
				Width:  suggestedData["width"].(float64),
				Height: suggestedData["height"].(float64),
			}
			slog.Debug("python suggested ROI for next frame",
				"worker_id", w.id,
				"roi", suggestedROI,
			)
		}

		// Extract ROI metadata that was sent with the frame (Python echoes it back)
		var roiMetadata *types.ROIProcessingMetadata
		if data, ok := result["data"].(map[string]interface{}); ok {
			if metadata, ok := data["metadata"].(map[string]interface{}); ok {
				if roiProcessing, ok := metadata["roi_processing"].(map[string]interface{}); ok {
					// Reconstruct ROI metadata from echoed data
					roiMetadata = &types.ROIProcessingMetadata{
						TargetSize:       int(roiProcessing["target_size"].(float64)),
						CropApplied:      roiProcessing["crop_applied"].(bool),
						Source:           roiProcessing["source"].(string),
						AutoFocusEnabled: roiProcessing["auto_focus_enabled"].(bool),
					}

					// Optional fields
					if historyFrames, ok := roiProcessing["history_frames"].(float64); ok {
						roiMetadata.HistoryFrames = int(historyFrames)
					}
					if detectionCount, ok := roiProcessing["detection_count"].(float64); ok {
						roiMetadata.DetectionCount = int(detectionCount)
					}
					if expansionPct, ok := roiProcessing["expansion_pct"].(float64); ok {
						roiMetadata.ExpansionPct = expansionPct
					}

					// Reconstruct ROIs if present
					if attentionROIs, ok := roiProcessing["attention_rois"].([]interface{}); ok {
						for _, roiData := range attentionROIs {
							if roi, ok := roiData.(map[string]interface{}); ok {
								roiMetadata.AttentionROIs = append(roiMetadata.AttentionROIs, types.NormalizedRect{
									X:      roi["x"].(float64),
									Y:      roi["y"].(float64),
									Width:  roi["width"].(float64),
									Height: roi["height"].(float64),
								})
							}
						}
					}

					if mergedROIData, ok := roiProcessing["merged_roi"].(map[string]interface{}); ok {
						roiMetadata.MergedROI = &types.NormalizedRect{
							X:      mergedROIData["x"].(float64),
							Y:      mergedROIData["y"].(float64),
							Width:  mergedROIData["width"].(float64),
							Height: mergedROIData["height"].(float64),
						}
					}
				}
			}
		}

		// Create Inference object
		inference := &PersonDetectionInference{
			InferenceType: "person_detection",
			InstanceID:    w.instanceID,
			RoomID:        w.roomID,
			timestamp:     time.Now(), // Use current time (Python worker also provides timestamp)
			Data:          result["data"].(map[string]interface{}),
			Timing:        result["timing"].(map[string]interface{}),
			suggestedROI:  suggestedROI, // Hybrid auto-focus suggestion
			ROIMetadata:   roiMetadata,  // ROI metadata from frame processing
		}

		// Send to results channel (non-blocking)
		select {
		case w.results <- inference:
			atomic.AddUint64(&w.inferenceCount, 1)

			// Update last seen timestamp
			w.lastSeenAt.Store(time.Now())

			// Track latency
			if timing, ok := result["timing"].(map[string]interface{}); ok {
				if totalMS, ok := timing["total_ms"].(float64); ok {
					atomic.AddUint64(&w.totalLatencyMS, uint64(totalMS))
				}
			}

		default:
			// Results channel full, drop inference
			slog.Warn("dropping inference, results channel full", "worker_id", w.id)
		}
	}
}

// logStderr logs Python worker stderr output
// Maps Python log levels to Go slog levels appropriately
func (w *PythonPersonDetector) logStderr() {
	defer w.wg.Done()

	scanner := bufio.NewScanner(w.stderr)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse Python log level from format: "timestamp [LEVEL] message"
		// Map: [ERROR] → Error, [WARNING] → Warn, [INFO]/[DEBUG] → Debug
		if containsAny(line, "[ERROR]", "[CRITICAL]") {
			slog.Error("python worker error",
				"worker_id", w.id,
				"log", line,
			)
		} else if containsAny(line, "[WARNING]", "[WARN]") {
			slog.Warn("python worker warning",
				"worker_id", w.id,
				"log", line,
			)
		} else {
			// [INFO], [DEBUG], or unformatted logs → Debug
			slog.Debug("python worker log",
				"worker_id", w.id,
				"log", line,
			)
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("error reading stderr",
			"worker_id", w.id,
			"error", err,
		)
	}
}

// waitProcess waits for Python process to exit and prevents zombie processes
func (w *PythonPersonDetector) waitProcess() {
	defer w.wg.Done()

	if w.cmd == nil || w.cmd.Process == nil {
		return
	}

	// Block until process exits
	err := w.cmd.Wait()

	if err != nil {
		// Check if it's a normal exit (context cancelled)
		select {
		case <-w.ctx.Done():
			// Expected exit due to shutdown
			slog.Debug("python process exited (shutdown)",
				"worker_id", w.id,
				"pid", w.cmd.Process.Pid,
			)
		default:
			// Unexpected exit (crash or error)
			slog.Error("python process exited unexpectedly",
				"worker_id", w.id,
				"pid", w.cmd.Process.Pid,
				"error", err,
			)
		}
	} else {
		// Clean exit (exit code 0)
		slog.Info("python process exited cleanly",
			"worker_id", w.id,
			"pid", w.cmd.Process.Pid,
		)
	}
}

// containsAny checks if string contains any of the given substrings
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// Results returns the inference results channel
func (w *PythonPersonDetector) Results() <-chan types.Inference {
	return w.results
}

// Metrics returns current worker health metrics
func (w *PythonPersonDetector) Metrics() types.WorkerMetrics {
	framesProcessed := atomic.LoadUint64(&w.frameCount)
	framesDropped := atomic.LoadUint64(&w.framesDropped)
	inferencesEmitted := atomic.LoadUint64(&w.inferenceCount)
	totalLatencyMS := atomic.LoadUint64(&w.totalLatencyMS)

	// Calculate average latency
	var avgLatencyMS float64
	if inferencesEmitted > 0 {
		avgLatencyMS = float64(totalLatencyMS) / float64(inferencesEmitted)
	}

	// Get last seen timestamp
	var lastSeen time.Time
	if val := w.lastSeenAt.Load(); val != nil {
		lastSeen = val.(time.Time)
	}

	return types.WorkerMetrics{
		FramesProcessed:   framesProcessed,
		FramesDropped:     framesDropped,
		InferencesEmitted: inferencesEmitted,
		AvgLatencyMS:      avgLatencyMS,
		LastSeenAt:        lastSeen,
	}
}

// Stop stops the worker and kills the Python process
func (w *PythonPersonDetector) Stop() error {
	if !w.isActive.Load() {
		return nil
	}

	// Set inactive IMMEDIATELY to prevent concurrent Stop() calls
	// This must happen before any channel operations to prevent double-close panic
	w.isActive.Store(false)

	slog.Info("stopping python person detector", "worker_id", w.id)

	// Cancel context
	if w.cancel != nil {
		w.cancel()
	}

	// Close stdin to signal Python process to exit gracefully
	if w.stdin != nil {
		w.stdin.Close()
	}

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean shutdown
		slog.Info("python worker goroutines stopped cleanly", "worker_id", w.id)
	case <-time.After(2 * time.Second):
		// Timeout - force kill Python process
		slog.Warn("python worker stop timeout, force killing process", "worker_id", w.id)
		if w.cmd != nil && w.cmd.Process != nil {
			if err := w.cmd.Process.Kill(); err != nil {
				slog.Error("failed to kill python process",
					"worker_id", w.id,
					"error", err,
				)
			}
		}
	}

	// Close channels safely (only if not already closed)
	// Note: SendFrame() has panic recovery for sends to closed channels
	safeClose := func(ch chan types.Frame) {
		defer func() {
			if r := recover(); r != nil {
				slog.Debug("channel already closed during Stop()", "worker_id", w.id)
			}
		}()
		close(ch)
	}

	safeCloseResults := func(ch chan types.Inference) {
		defer func() {
			if r := recover(); r != nil {
				slog.Debug("results channel already closed during Stop()", "worker_id", w.id)
			}
		}()
		close(ch)
	}

	safeClose(w.input)
	safeCloseResults(w.results)

	slog.Info("python person detector stopped",
		"worker_id", w.id,
		"frames_processed", atomic.LoadUint64(&w.frameCount),
		"inferences", atomic.LoadUint64(&w.inferenceCount),
	)

	return nil
}

// SetModelSize sends a command to the Python worker to reload the model (hot-reload)
func (w *PythonPersonDetector) SetModelSize(size string) error {
	validSizes := map[string]bool{"n": true, "s": true, "m": true, "l": true, "x": true}
	if !validSizes[size] {
		return fmt.Errorf("invalid model size: %s (must be n/s/m/l/x)", size)
	}

	slog.Info("sending set_model_size command to Python worker",
		"worker_id", w.id,
		"model_size", size,
	)

	// Build control command JSON
	command := map[string]interface{}{
		"type":    "command",
		"command": "set_model_size",
		"params": map[string]interface{}{
			"size": size,
		},
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	// Send command to Python worker via stdin
	if w.stdin == nil {
		return fmt.Errorf("stdin not available (worker not started)")
	}

	if _, err := w.stdin.Write(append(jsonBytes, '\n')); err != nil {
		return fmt.Errorf("failed to write command to stdin: %w", err)
	}

	slog.Info("set_model_size command sent to Python worker",
		"worker_id", w.id,
		"model_size", size,
	)

	return nil
}

// PersonDetectionInference implements types.Inference
type PersonDetectionInference struct {
	InferenceType string
	InstanceID    string
	RoomID        string
	timestamp     time.Time // private field
	Data          map[string]interface{}
	Timing        map[string]interface{}
	// ROI metadata from frame processing (for MQTT feedback)
	ROIMetadata *types.ROIProcessingMetadata
	// suggestedROI is Python's suggestion for the NEXT frame (hybrid auto-focus)
	suggestedROI *types.NormalizedRect
}

// Type returns the inference type
func (i *PersonDetectionInference) Type() string {
	return i.InferenceType
}

// Timestamp returns the inference timestamp (implements types.Inference)
func (i *PersonDetectionInference) Timestamp() time.Time {
	return i.timestamp
}

// SuggestedROI implements types.Inference interface (hybrid auto-focus)
func (i *PersonDetectionInference) SuggestedROI() *types.NormalizedRect {
	return i.suggestedROI
}

// ToJSON converts inference to JSON (implements types.Inference)
func (i *PersonDetectionInference) ToJSON() ([]byte, error) {
	payload := map[string]interface{}{
		"type":        i.InferenceType,
		"instance_id": i.InstanceID,
		"room_id":     i.RoomID,
		"timestamp":   i.timestamp.Format(time.RFC3339Nano),
		"data":        i.Data,
		"timing":      i.Timing,
	}

	// Include complete ROI metadata for MQTT consumers (dashboards, experts, analytics)
	if i.ROIMetadata != nil {
		roiMeta := map[string]interface{}{
			"source":             i.ROIMetadata.Source,
			"auto_focus_enabled": i.ROIMetadata.AutoFocusEnabled,
			"target_size":        i.ROIMetadata.TargetSize,
			"crop_applied":       i.ROIMetadata.CropApplied,
		}

		// Include attention ROIs (regions being processed)
		if len(i.ROIMetadata.AttentionROIs) > 0 {
			roiMeta["attention_rois"] = i.ROIMetadata.AttentionROIs
		}

		// Include merged ROI (final bounding box)
		if i.ROIMetadata.MergedROI != nil {
			roiMeta["merged_roi"] = i.ROIMetadata.MergedROI
		}

		// Include auto-focus metadata (if applicable)
		if i.ROIMetadata.AutoFocusEnabled {
			roiMeta["history_frames"] = i.ROIMetadata.HistoryFrames
			roiMeta["detection_count"] = i.ROIMetadata.DetectionCount
			roiMeta["expansion_pct"] = i.ROIMetadata.ExpansionPct
		}

		payload["roi_metadata"] = roiMeta
	}

	// Include suggested ROI for next frame (hybrid auto-focus)
	if i.suggestedROI != nil {
		payload["suggested_roi"] = i.suggestedROI
	}

	return json.Marshal(payload)
}
