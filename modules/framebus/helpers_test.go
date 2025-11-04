package framebus

import "testing"

func TestCalculateDropRate(t *testing.T) {
	tests := []struct {
		name     string
		stats    BusStats
		expected float64
	}{
		{
			name: "no frames",
			stats: BusStats{
				TotalSent:    0,
				TotalDropped: 0,
			},
			expected: 0.0,
		},
		{
			name: "no drops",
			stats: BusStats{
				TotalSent:    100,
				TotalDropped: 0,
			},
			expected: 0.0,
		},
		{
			name: "all dropped",
			stats: BusStats{
				TotalSent:    0,
				TotalDropped: 100,
			},
			expected: 1.0,
		},
		{
			name: "50% drop rate",
			stats: BusStats{
				TotalSent:    50,
				TotalDropped: 50,
			},
			expected: 0.5,
		},
		{
			name: "97% drop rate (typical inference)",
			stats: BusStats{
				TotalSent:    3,
				TotalDropped: 97,
			},
			expected: 0.97,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDropRate(tt.stats)
			if got != tt.expected {
				t.Errorf("CalculateDropRate() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCalculateSubscriberDropRate(t *testing.T) {
	stats := BusStats{
		Subscribers: map[string]SubscriberStats{
			"fast-worker": {
				Sent:    100,
				Dropped: 0,
			},
			"slow-worker": {
				Sent:    3,
				Dropped: 97,
			},
			"no-frames": {
				Sent:    0,
				Dropped: 0,
			},
		},
	}

	tests := []struct {
		name         string
		subscriberID string
		expected     float64
	}{
		{
			name:         "fast worker - no drops",
			subscriberID: "fast-worker",
			expected:     0.0,
		},
		{
			name:         "slow worker - 97% drops",
			subscriberID: "slow-worker",
			expected:     0.97,
		},
		{
			name:         "no frames yet",
			subscriberID: "no-frames",
			expected:     0.0,
		},
		{
			name:         "nonexistent subscriber",
			subscriberID: "unknown",
			expected:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateSubscriberDropRate(stats, tt.subscriberID)
			if got != tt.expected {
				t.Errorf("CalculateSubscriberDropRate() = %v, want %v", got, tt.expected)
			}
		})
	}
}
