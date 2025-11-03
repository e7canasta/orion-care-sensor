package core

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// WorkerHealthMetrics contains health metrics for a worker with drop rate
type WorkerHealthMetrics struct {
	FramesProcessed   uint64    `json:"frames_processed"`
	FramesDropped     uint64    `json:"frames_dropped"`
	InferencesEmitted uint64    `json:"inferences_emitted"`
	DropRate          float64   `json:"drop_rate"`
	AvgLatencyMS      float64   `json:"avg_latency_ms"`
	LastSeenAt        time.Time `json:"last_seen_at"`
}

// HealthStatus represents the health state of the Orion service
type HealthStatus struct {
	Status          string                          `json:"status"`           // "healthy", "degraded", "unhealthy"
	UptimeSeconds   int64                           `json:"uptime_seconds"`
	WorkersUp       int                             `json:"workers_up"`
	WorkersTotal    int                             `json:"workers_total"`
	StreamConnected bool                            `json:"stream_connected"`
	MQTTConnected   bool                            `json:"mqtt_connected"`
	Workers         map[string]WorkerHealthMetrics  `json:"workers,omitempty"`
}

// HealthCheck returns the current health status of the service
func (o *Orion) HealthCheck() HealthStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()

	status := HealthStatus{
		Status:        "healthy",
		UptimeSeconds: int64(time.Since(o.started).Seconds()),
		WorkersTotal:  len(o.workers),
		Workers:       make(map[string]WorkerHealthMetrics),
	}

	// Check stream connection
	if o.stream != nil && o.isRunning {
		status.StreamConnected = true
	}

	// Check MQTT connection
	if o.emitter != nil && o.emitter.Client != nil && o.emitter.Client.IsConnected() {
		status.MQTTConnected = true
	}

	// Collect worker metrics
	if o.isRunning {
		status.WorkersUp = len(o.workers)

		for _, worker := range o.workers {
			metrics := worker.Metrics()

			// Calculate drop rate
			var dropRate float64
			totalFrames := metrics.FramesProcessed + metrics.FramesDropped
			if totalFrames > 0 {
				dropRate = float64(metrics.FramesDropped) / float64(totalFrames)
			}

			status.Workers[worker.ID()] = WorkerHealthMetrics{
				FramesProcessed:   metrics.FramesProcessed,
				FramesDropped:     metrics.FramesDropped,
				InferencesEmitted: metrics.InferencesEmitted,
				DropRate:          dropRate,
				AvgLatencyMS:      metrics.AvgLatencyMS,
				LastSeenAt:        metrics.LastSeenAt,
			}
		}
	} else {
		status.WorkersUp = 0
	}

	// Determine overall health status
	if !o.isRunning {
		status.Status = "unhealthy"
	} else if !status.StreamConnected || !status.MQTTConnected {
		status.Status = "degraded"
	}

	return status
}

// LivenessHandler handles /health endpoint (simple liveness check)
// Returns 200 if the service process is alive
func (o *Orion) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Simple check: if we can execute this code, we're alive
	response := map[string]interface{}{
		"status": "alive",
		"uptime": int64(time.Since(o.started).Seconds()),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ReadinessHandler handles /readiness endpoint (detailed readiness check)
// Returns 200 only if the service is ready to handle requests
func (o *Orion) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := o.HealthCheck()

	// Determine HTTP status based on health
	statusCode := http.StatusOK
	if health.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if health.Status == "degraded" {
		statusCode = http.StatusOK // Still ready, but degraded
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(health)
}

// MetricsHandler handles /metrics endpoint (stub for future Prometheus)
// Currently returns a simple placeholder
func (o *Orion) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	// Stub: Will be replaced with Prometheus metrics in future
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# Prometheus metrics endpoint (future implementation)\n"))
	w.Write([]byte("# orion_uptime_seconds{instance=\"" + o.cfg.InstanceID + "\"} " +
		time.Since(o.started).String() + "\n"))
}

// StartHealthServer starts the HTTP health check server on the given port
// This runs in a separate goroutine and does not block
func (o *Orion) StartHealthServer(port string) error {
	mux := http.NewServeMux()

	// Register health check endpoints
	mux.HandleFunc("/health", o.LivenessHandler)
	mux.HandleFunc("/readiness", o.ReadinessHandler)
	mux.HandleFunc("/metrics", o.MetricsHandler)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("starting health check server",
		"port", port,
		"endpoints", []string{"/health", "/readiness", "/metrics"},
	)

	// Start server in goroutine (non-blocking)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health check server failed", "error", err)
		}
	}()

	return nil
}
