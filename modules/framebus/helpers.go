package framebus

// CalculateDropRate returns the drop rate as a percentage (0.0 to 1.0).
// Returns 0.0 if no frames have been sent or dropped.
func CalculateDropRate(stats BusStats) float64 {
	total := stats.TotalSent + stats.TotalDropped
	if total == 0 {
		return 0.0
	}
	return float64(stats.TotalDropped) / float64(total)
}

// CalculateSubscriberDropRate returns the drop rate for a specific subscriber.
// Returns 0.0 if subscriber not found or if no frames have been sent or dropped.
func CalculateSubscriberDropRate(stats BusStats, subscriberID string) float64 {
	sub, exists := stats.Subscribers[subscriberID]
	if !exists {
		return 0.0
	}

	total := sub.Sent + sub.Dropped
	if total == 0 {
		return 0.0
	}
	return float64(sub.Dropped) / float64(total)
}
