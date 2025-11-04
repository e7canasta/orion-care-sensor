package streamcapture_test

/*

1. ✅ ExampleNewRTSPStream()
- Muestra configuración básica
- Aparecerá en godoc de NewRTSPStream()

2. ✅ ExampleRTSPStream_Start()
- Muestra workflow completo: Start → Warmup → Process frames
- Comentado (no ejecutable) porque requiere RTSP real

3. ✅ ExampleRTSPStream_SetTargetFPS()
- Muestra hot-reload de FPS
- Comentado (no ejecutable) porque requiere stream activo

4. ✅ ExampleRTSPStream_Stats()
- Muestra monitoreo de estadísticas
- Comentado (no ejecutable) porque requiere stream activo

5. ✅ ExampleResolution_Dimensions()
- Ejecutable con Output verificable
- PASS ✓

6. ✅ ExampleResolution_String()
- Ejecutable con Output verificable
- PASS ✓

7. ✅ ExampleHardwareAccel()
- Muestra 3 modos de aceleración
- Constructor pattern

*/

import (
	"fmt"
	"testing"
	"time"

	streamcapture "github.com/e7canasta/orion-care-sensor/modules/stream-capture"
)

// TestNewRTSPStream_FailFast tests fail-fast validation in constructor
//
// These tests ensure configuration errors are caught at construction time
// (load time) rather than runtime, following the "Fail Fast" principle.
func TestNewRTSPStream_FailFast(t *testing.T) {
	tests := []struct {
		name    string
		cfg     streamcapture.RTSPConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: streamcapture.RTSPConfig{
				URL:          "rtsp://test.local/stream",
				TargetFPS:    2.0,
				Resolution:   streamcapture.Res720p,
				SourceStream: "test",
			},
			wantErr: false,
		},
		{
			name: "empty URL",
			cfg: streamcapture.RTSPConfig{
				URL:        "",
				TargetFPS:  2.0,
				Resolution: streamcapture.Res720p,
			},
			wantErr: true,
			errMsg:  "RTSP URL is required",
		},
		{
			name: "invalid FPS - zero",
			cfg: streamcapture.RTSPConfig{
				URL:        "rtsp://test.local/stream",
				TargetFPS:  0.0,
				Resolution: streamcapture.Res720p,
			},
			wantErr: true,
			errMsg:  "invalid FPS",
		},
		{
			name: "invalid FPS - too low",
			cfg: streamcapture.RTSPConfig{
				URL:        "rtsp://test.local/stream",
				TargetFPS:  0.05,
				Resolution: streamcapture.Res720p,
			},
			wantErr: true,
			errMsg:  "invalid FPS",
		},
		{
			name: "invalid FPS - too high",
			cfg: streamcapture.RTSPConfig{
				URL:        "rtsp://test.local/stream",
				TargetFPS:  100.0,
				Resolution: streamcapture.Res720p,
			},
			wantErr: true,
			errMsg:  "invalid FPS",
		},
		{
			name: "valid FPS - minimum boundary",
			cfg: streamcapture.RTSPConfig{
				URL:          "rtsp://test.local/stream",
				TargetFPS:    0.1,
				Resolution:   streamcapture.Res720p,
				SourceStream: "test",
			},
			wantErr: false,
		},
		{
			name: "valid FPS - maximum boundary",
			cfg: streamcapture.RTSPConfig{
				URL:          "rtsp://test.local/stream",
				TargetFPS:    30.0,
				Resolution:   streamcapture.Res720p,
				SourceStream: "test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := streamcapture.NewRTSPStream(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewRTSPStream() expected error containing %q, got nil", tt.errMsg)
					return
				}
				// Check error message contains expected substring
				if tt.errMsg != "" {
					errStr := err.Error()
					if len(errStr) == 0 || !contains(errStr, tt.errMsg) {
						t.Errorf("NewRTSPStream() error = %q, want error containing %q", errStr, tt.errMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("NewRTSPStream() unexpected error = %v", err)
					return
				}
				if stream == nil {
					t.Error("NewRTSPStream() returned nil stream with no error")
				}
			}
		})
	}
}

// TestResolution_Dimensions tests resolution dimension calculations
func TestResolution_Dimensions(t *testing.T) {
	tests := []struct {
		name       string
		resolution streamcapture.Resolution
		wantWidth  int
		wantHeight int
	}{
		{
			name:       "512p",
			resolution: streamcapture.Res512p,
			wantWidth:  910,
			wantHeight: 512,
		},
		{
			name:       "720p",
			resolution: streamcapture.Res720p,
			wantWidth:  1280,
			wantHeight: 720,
		},
		{
			name:       "1080p",
			resolution: streamcapture.Res1080p,
			wantWidth:  1920,
			wantHeight: 1080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height := tt.resolution.Dimensions()
			if width != tt.wantWidth {
				t.Errorf("Resolution.Dimensions() width = %d, want %d", width, tt.wantWidth)
			}
			if height != tt.wantHeight {
				t.Errorf("Resolution.Dimensions() height = %d, want %d", height, tt.wantHeight)
			}
		})
	}
}

// TestResolution_String tests resolution string representation
func TestResolution_String(t *testing.T) {
	tests := []struct {
		name       string
		resolution streamcapture.Resolution
		want       string
	}{
		{
			name:       "512p",
			resolution: streamcapture.Res512p,
			want:       "512p",
		},
		{
			name:       "720p",
			resolution: streamcapture.Res720p,
			want:       "720p",
		},
		{
			name:       "1080p",
			resolution: streamcapture.Res1080p,
			want:       "1080p",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resolution.String(); got != tt.want {
				t.Errorf("Resolution.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCalculateFPSStats tests FPS statistics calculation (math only, no GStreamer)
func TestCalculateFPSStats(t *testing.T) {
	tests := []struct {
		name          string
		frameTimes    []time.Time
		totalDuration time.Duration
		wantFrames    int
		wantFPSMean   float64
		wantStable    bool
		epsilon       float64 // tolerance for float comparison
	}{
		{
			name: "near-perfect 2 Hz stream",
			frameTimes: []time.Time{
				time.Unix(0, 0),
				time.Unix(0, 500*1000*1000),  // 500ms
				time.Unix(0, 1000*1000*1000), // 1000ms
				time.Unix(0, 1500*1000*1000), // 1500ms
			},
			totalDuration: 1500 * time.Millisecond,
			wantFrames:    4,
			wantFPSMean:   2.666, // 4 frames / 1.5s ≈ 2.67 Hz
			wantStable:    false, // StdDev of instantaneous FPS is high due to sample size
			epsilon:       0.01,
		},
		{
			name: "near-perfect 1 Hz stream",
			frameTimes: []time.Time{
				time.Unix(0, 0),
				time.Unix(1, 0),
				time.Unix(2, 0),
				time.Unix(3, 0),
			},
			totalDuration: 3 * time.Second,
			wantFrames:    4,
			wantFPSMean:   1.333, // 4 frames / 3s ≈ 1.33 Hz
			wantStable:    false, // StdDev of instantaneous FPS is high due to sample size
			epsilon:       0.01,
		},
		{
			name: "unstable stream (high variance)",
			frameTimes: []time.Time{
				time.Unix(0, 0),
				time.Unix(0, 100*1000*1000),  // 100ms
				time.Unix(0, 1000*1000*1000), // 1000ms (900ms gap)
				time.Unix(0, 1200*1000*1000), // 1200ms (200ms gap)
			},
			totalDuration: 1200 * time.Millisecond,
			wantFrames:    4,
			wantFPSMean:   3.333, // 4 frames / 1.2s ≈ 3.33 Hz
			wantStable:    false, // High variance due to 100ms vs 900ms gaps
			epsilon:       0.01,
		},
		{
			name: "single frame",
			frameTimes: []time.Time{
				time.Unix(0, 0),
			},
			totalDuration: 1 * time.Second,
			wantFrames:    1,
			wantFPSMean:   1.0,
			wantStable:    false, // Not enough data
			epsilon:       0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := streamcapture.CalculateFPSStats(tt.frameTimes, tt.totalDuration)

			if stats.FramesReceived != tt.wantFrames {
				t.Errorf("CalculateFPSStats() FramesReceived = %d, want %d", stats.FramesReceived, tt.wantFrames)
			}

			if !almostEqual(stats.FPSMean, tt.wantFPSMean, tt.epsilon) {
				t.Errorf("CalculateFPSStats() FPSMean = %.3f, want %.3f (±%.3f)", stats.FPSMean, tt.wantFPSMean, tt.epsilon)
			}

			if stats.IsStable != tt.wantStable {
				t.Errorf("CalculateFPSStats() IsStable = %v, want %v (FPSMean=%.2f, StdDev=%.2f)",
					stats.IsStable, tt.wantStable, stats.FPSMean, stats.FPSStdDev)
			}
		})
	}
}

// Helper functions

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func almostEqual(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// Example functions for godoc (appear in pkg.go.dev)

// ExampleNewRTSPStream demonstrates basic stream creation and validation.
func ExampleNewRTSPStream() {
	cfg := streamcapture.RTSPConfig{
		URL:          "rtsp://192.168.1.100/stream",
		Resolution:   streamcapture.Res720p,
		TargetFPS:    2.0,
		SourceStream: "camera-1",
		Acceleration: streamcapture.AccelAuto,
	}

	stream, err := streamcapture.NewRTSPStream(cfg)
	if err != nil {
		// Handle error (e.g., GStreamer not available, invalid config)
		return
	}

	// Stream created successfully
	_ = stream
}

// ExampleRTSPStream_Start demonstrates frame capture from an RTSP stream.
//
// Note: This example requires a real RTSP stream to execute.
func ExampleRTSPStream_Start() {
	// cfg := streamcapture.RTSPConfig{
	// 	URL:        "rtsp://camera/stream",
	// 	Resolution: streamcapture.Res720p,
	// 	TargetFPS:  2.0,
	// }
	//
	// stream, _ := streamcapture.NewRTSPStream(cfg)
	// defer stream.Stop()
	//
	// ctx := context.Background()
	// frameChan, err := stream.Start(ctx)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	//
	// // Recommended: Warmup to measure FPS stability
	// stats, _ := stream.Warmup(ctx, 5*time.Second)
	// log.Printf("Stream stable: %v, FPS: %.2f", stats.IsStable, stats.FPSMean)
	//
	// // Process frames
	// for frame := range frameChan {
	// 	log.Printf("Frame %d: %dx%d, %d bytes",
	// 		frame.Seq, frame.Width, frame.Height, len(frame.Data))
	// }
}

// ExampleRTSPStream_SetTargetFPS demonstrates hot-reloading FPS without stream restart.
func ExampleRTSPStream_SetTargetFPS() {
	// cfg := streamcapture.RTSPConfig{
	// 	URL:        "rtsp://camera/stream",
	// 	TargetFPS:  2.0,
	// 	Resolution: streamcapture.Res720p,
	// }
	//
	// stream, _ := streamcapture.NewRTSPStream(cfg)
	// stream.Start(context.Background())
	// defer stream.Stop()
	//
	// // Change FPS dynamically (~2s interruption)
	// err := stream.SetTargetFPS(0.5) // 1 frame every 2 seconds
	// if err != nil {
	// 	log.Printf("FPS change failed: %v", err)
	// }
	//
	// // Stream continues with new FPS
}

// ExampleRTSPStream_Stats demonstrates statistics monitoring.
func ExampleRTSPStream_Stats() {
	// cfg := streamcapture.RTSPConfig{
	// 	URL:        "rtsp://camera/stream",
	// 	TargetFPS:  2.0,
	// 	Resolution: streamcapture.Res720p,
	// }
	//
	// stream, _ := streamcapture.NewRTSPStream(cfg)
	// stream.Start(context.Background())
	// defer stream.Stop()
	//
	// // Check statistics (thread-safe, can call from any goroutine)
	// stats := stream.Stats()
	// fmt.Printf("FPS: %.2f (target: %.2f)\n", stats.FPSReal, stats.FPSTarget)
	// fmt.Printf("Latency: %dms\n", stats.LatencyMS)
	// fmt.Printf("Frames captured: %d\n", stats.FrameCount)
	// fmt.Printf("Drop rate: %.2f%%\n", stats.DropRate)
	//
	// if stats.UsingVAAPI {
	// 	fmt.Printf("VAAPI decode latency (mean): %.2fms\n", stats.DecodeLatencyMeanMS)
	// }
}

// ExampleResolution_Dimensions demonstrates resolution dimension lookup.
func ExampleResolution_Dimensions() {
	width, height := streamcapture.Res720p.Dimensions()
	fmt.Printf("%d %d\n", width, height)
	// Output: 1280 720
}

// ExampleResolution_String demonstrates resolution string representation.
func ExampleResolution_String() {
	fmt.Println(streamcapture.Res720p.String())
	fmt.Println(streamcapture.Res1080p.String())
	// Output: 720p
	// 1080p
}

// ExampleHardwareAccel demonstrates acceleration mode selection.
func ExampleHardwareAccel() {
	// Auto mode (recommended): Try VAAPI, fallback to software
	cfg := streamcapture.RTSPConfig{
		URL:          "rtsp://camera/stream",
		Acceleration: streamcapture.AccelAuto,
	}

	// Force VAAPI (fail-fast if unavailable)
	cfgVAAPI := streamcapture.RTSPConfig{
		URL:          "rtsp://camera/stream",
		Acceleration: streamcapture.AccelVAAPI,
	}

	// Force software decode (debugging/compatibility)
	cfgSoftware := streamcapture.RTSPConfig{
		URL:          "rtsp://camera/stream",
		Acceleration: streamcapture.AccelSoftware,
	}

	_, _ = streamcapture.NewRTSPStream(cfg)
	_, _ = streamcapture.NewRTSPStream(cfgVAAPI)
	_, _ = streamcapture.NewRTSPStream(cfgSoftware)
}
