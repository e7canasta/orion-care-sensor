package framebus

import "time"

// Example: CriticalWorkerMonitor demonstrates proactive monitoring
// of Critical subscribers using the CriticalDropped metric.
//
// This pattern should be implemented in the Orion Core module's
// observability/health monitoring system.
//
// Usage in Orion Core:
//   monitor := NewCriticalWorkerMonitor(frameBus, alertService)
//   monitor.Start(ctx)
func ExampleCriticalWorkerMonitor() {
	// See CriticalWorkerMonitor implementation below
}

// CriticalWorkerMonitor proactively checks for Critical subscriber saturation.
type CriticalWorkerMonitor struct {
	bus           Bus
	alertService  AlertService // Your alerting system (e.g., PagerDuty, Slack)
	checkInterval time.Duration
}

// AlertService is a placeholder interface for your alerting system.
// Implement this based on your infrastructure (e.g., MQTT, HTTP, logs).
type AlertService interface {
	// Critical sends a critical alert (page on-call engineer)
	Critical(message string, labels map[string]interface{})

	// Warning sends a warning alert (log + monitor)
	Warning(message string, labels map[string]interface{})
}

// NewCriticalWorkerMonitor creates a monitor for Critical subscribers.
func NewCriticalWorkerMonitor(bus Bus, alertService AlertService) *CriticalWorkerMonitor {
	return &CriticalWorkerMonitor{
		bus:           bus,
		alertService:  alertService,
		checkInterval: 10 * time.Second, // Configurable
	}
}

// Start begins monitoring Critical subscribers.
// Blocks until context is cancelled.
func (m *CriticalWorkerMonitor) Start(stopCh <-chan struct{}) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkCriticalWorkers()
		case <-stopCh:
			return
		}
	}
}

// checkCriticalWorkers inspects all subscribers for Critical saturation.
func (m *CriticalWorkerMonitor) checkCriticalWorkers() {
	stats := m.bus.Stats()

	for id, sub := range stats.Subscribers {
		// Only monitor Critical priority subscribers
		if sub.Priority != PriorityCritical {
			continue
		}

		// Calculate drop rate
		total := sub.Sent + sub.Dropped
		if total == 0 {
			continue // No activity yet
		}
		dropRate := float64(sub.Dropped) / float64(total)

		// Check CriticalDropped metric (🚨 Most important)
		if sub.CriticalDropped > 0 {
			// CRITICAL ALERT: Critical worker dropping frames despite retry
			m.alertService.Critical("framebus_critical_worker_saturated", map[string]interface{}{
				"worker_id":        id,
				"critical_dropped": sub.CriticalDropped,
				"total_dropped":    sub.Dropped,
				"drop_rate":        dropRate,
				"priority":         "CRITICAL",
				"action":           "IMMEDIATE - Worker cannot keep up even with retry protection",
			})

			// Automatic remediation (optional):
			// - Restart worker
			// - Scale up resources
			// - Reduce inference rate
			// Example:
			//   m.restartWorker(id)
			//   m.scaleUpResources(id)

			continue
		}

		// Secondary check: High drop rate (but no CriticalDropped yet)
		if dropRate > 0.9 {
			// WARNING: Critical worker at risk of saturation
			m.alertService.Warning("framebus_critical_worker_high_drops", map[string]interface{}{
				"worker_id":     id,
				"drop_rate":     dropRate,
				"total_dropped": sub.Dropped,
				"priority":      "CRITICAL",
				"action":        "MONITOR - Worker falling behind but retry still succeeding",
			})
		}
	}

	// Additional check: Unhealthy subscribers across all priorities
	unhealthy := m.bus.GetUnhealthySubscribers()
	if len(unhealthy) > 0 {
		// Log for visibility (not necessarily an alert)
		// Example logging:
		// log.Info("unhealthy subscribers detected", "count", len(unhealthy), "ids", unhealthy)
	}
}

// Example: Health dashboard query
//
// This example shows how to expose FrameBus metrics for a monitoring dashboard
// (e.g., Prometheus, Grafana, custom HTTP endpoint).
func ExampleHealthDashboard(bus Bus) map[string]interface{} {
	stats := bus.Stats()

	// Aggregate metrics by priority
	priorityMetrics := make(map[SubscriberPriority]struct {
		Count        int
		TotalDropped uint64
		TotalSent    uint64
	})

	for _, sub := range stats.Subscribers {
		metric := priorityMetrics[sub.Priority]
		metric.Count++
		metric.TotalDropped += sub.Dropped
		metric.TotalSent += sub.Sent
		priorityMetrics[sub.Priority] = metric
	}

	// Build dashboard payload
	dashboard := map[string]interface{}{
		"total_published": stats.TotalPublished,
		"total_sent":      stats.TotalSent,
		"total_dropped":   stats.TotalDropped,
		"global_drop_rate": CalculateDropRate(stats),
		"by_priority":      make(map[string]interface{}),
	}

	// Per-priority breakdown
	for priority, metric := range priorityMetrics {
		priorityName := ""
		switch priority {
		case PriorityCritical:
			priorityName = "critical"
		case PriorityHigh:
			priorityName = "high"
		case PriorityNormal:
			priorityName = "normal"
		case PriorityBestEffort:
			priorityName = "best_effort"
		}

		total := metric.TotalSent + metric.TotalDropped
		dropRate := 0.0
		if total > 0 {
			dropRate = float64(metric.TotalDropped) / float64(total)
		}

		dashboard["by_priority"].(map[string]interface{})[priorityName] = map[string]interface{}{
			"subscriber_count": metric.Count,
			"total_sent":       metric.TotalSent,
			"total_dropped":    metric.TotalDropped,
			"drop_rate":        dropRate,
		}
	}

	return dashboard
}

// Example: Alert on Critical worker saturation with auto-restart
//
// This pattern implements automatic remediation when a Critical worker
// becomes saturated (CriticalDropped > 0).
func ExampleAutoRestartCriticalWorker(bus Bus, workerManager WorkerManager) {
	stats := bus.Stats()

	for _, sub := range stats.Subscribers {
		// Only apply to Critical subscribers
		if sub.Priority != PriorityCritical {
			continue
		}

		// Check CriticalDropped metric
		if sub.CriticalDropped > 0 {
			// Log incident
			// log.Error("Critical worker saturated, restarting",
			//     "worker_id", id,
			//     "critical_dropped", sub.CriticalDropped,
			//     "drop_rate", float64(sub.Dropped)/float64(sub.Sent+sub.Dropped))

			// Restart worker (implementation depends on WorkerManager)
			// err := workerManager.Restart(id)
			// if err != nil {
			//     log.Error("Failed to restart critical worker", "error", err)
			//     // Escalate alert
			// }
		}
	}
}

// WorkerManager is a placeholder interface for worker lifecycle management.
// This would be implemented in the worker-lifecycle module.
type WorkerManager interface {
	Restart(workerID string) error
	Stop(workerID string) error
	GetStatus(workerID string) (string, error)
}
