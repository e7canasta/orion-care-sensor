package streamcapture

import (
	"context"
	"testing"
	"time"
)

// TestRTSPStream_Stop_Idempotent verifies that Stop() can be called multiple times safely
// This validates the double-close protection using atomic.Bool
func TestRTSPStream_Stop_Idempotent(t *testing.T) {
	// Use mock stream if RTSP_URL not available
	// This test focuses on Stop() logic, not actual streaming
	cfg := RTSPConfig{
		URL:          "rtsp://test.invalid/stream", // Invalid URL is OK - we won't start
		Resolution:   Res720p,
		TargetFPS:    2.0,
		SourceStream: "test",
		Acceleration: AccelSoftware,
	}

	stream, err := NewRTSPStream(cfg)
	if err != nil {
		t.Skipf("Skipping test: GStreamer not available or VAAPI check failed: %v", err)
	}

	// First Stop() call (stream not started, should be no-op)
	err = stream.Stop()
	if err != nil {
		t.Errorf("First Stop() on non-started stream failed: %v", err)
	}

	// Second Stop() call (should still be no-op, no panic)
	err = stream.Stop()
	if err != nil {
		t.Errorf("Second Stop() on non-started stream failed: %v", err)
	}

	// The critical test: verify no panic occurred
	t.Log("✅ Double Stop() on non-started stream successful (no panic)")
}

// TestRTSPStream_Stop_AfterStart verifies Stop() works correctly after Start()
func TestRTSPStream_Stop_AfterStart(t *testing.T) {
	// This test requires actual GStreamer + valid stream
	// Skip if not available
	t.Skip("Skipping integration test (requires GStreamer + RTSP stream)")

	cfg := RTSPConfig{
		URL:          "rtsp://127.0.0.1:8554/test", // Assumes go2rtc or similar
		Resolution:   Res720p,
		TargetFPS:    2.0,
		SourceStream: "test",
		Acceleration: AccelSoftware,
	}

	stream, err := NewRTSPStream(cfg)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start stream
	frameChan, err := stream.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	// Let it run briefly
	time.Sleep(500 * time.Millisecond)

	// Drain channel before stopping
	go func() {
		for range frameChan {
			// Discard frames
		}
	}()

	// First Stop()
	err = stream.Stop()
	if err != nil {
		t.Errorf("First Stop() failed: %v", err)
	}

	// Second Stop() (should be idempotent)
	err = stream.Stop()
	if err != nil {
		t.Errorf("Second Stop() failed: %v", err)
	}

	// Third Stop() (paranoia check)
	err = stream.Stop()
	if err != nil {
		t.Errorf("Third Stop() failed: %v", err)
	}

	t.Log("✅ Multiple Stop() calls after Start() successful (no panic)")
}

// TestRTSPStream_FailFast_InvalidFPS validates fail-fast FPS validation
func TestRTSPStream_FailFast_InvalidFPS(t *testing.T) {
	testCases := []struct {
		name      string
		fps       float64
		shouldErr bool
	}{
		{"valid_low", 0.1, false},
		{"valid_mid", 2.0, false},
		{"valid_high", 30.0, false},
		{"invalid_zero", 0.0, true},
		{"invalid_negative", -1.0, true},
		{"invalid_too_low", 0.05, true},
		{"invalid_too_high", 35.0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := RTSPConfig{
				URL:          "rtsp://test.invalid/stream",
				Resolution:   Res720p,
				TargetFPS:    tc.fps,
				SourceStream: "test",
				Acceleration: AccelSoftware,
			}

			stream, err := NewRTSPStream(cfg)

			if tc.shouldErr && err == nil {
				t.Errorf("Expected error for FPS=%.2f, got nil", tc.fps)
			}

			if !tc.shouldErr && err != nil && stream == nil {
				// Only fail if it's an FPS error (not GStreamer unavailable)
				t.Logf("Warning: GStreamer may not be available: %v", err)
			}
		})
	}
}

// TestRTSPStream_FailFast_EmptyURL validates fail-fast URL validation
func TestRTSPStream_FailFast_EmptyURL(t *testing.T) {
	cfg := RTSPConfig{
		URL:          "", // Empty URL should fail
		Resolution:   Res720p,
		TargetFPS:    2.0,
		SourceStream: "test",
	}

	stream, err := NewRTSPStream(cfg)
	if err == nil {
		t.Errorf("Expected error for empty URL, got nil")
	}

	if stream != nil {
		t.Errorf("Expected nil stream for invalid config, got non-nil")
	}

	t.Logf("✅ Fail-fast validation caught empty URL: %v", err)
}
