package types

import (
	"context"
	"time"
)

// WorkerMetrics contains health metrics for a worker
type WorkerMetrics struct {
	FramesProcessed   uint64    `json:"frames_processed"`
	FramesDropped     uint64    `json:"frames_dropped"`
	InferencesEmitted uint64    `json:"inferences_emitted"`
	AvgLatencyMS      float64   `json:"avg_latency_ms"`
	LastSeenAt        time.Time `json:"last_seen_at"`
}

// InferenceWorker processes frames and generates inferences
type InferenceWorker interface {
	// ID returns the worker's unique identifier
	ID() string
	// Start begins the worker
	Start(ctx context.Context) error
	// SendFrame sends a frame to the worker for processing (non-blocking)
	SendFrame(frame Frame) error
	// Results returns a channel of inferences
	Results() <-chan Inference
	// Stop stops the worker
	Stop() error
	// Metrics returns current worker health metrics
	Metrics() WorkerMetrics
}
