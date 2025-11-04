package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	streamcapture "github.com/e7canasta/orion-care-sensor/modules/stream-capture"
)

// Version information
const version = "v0.1.0"

func main() {
	// Parse command-line flags
	rtspURL := flag.String("url", "", "RTSP stream URL (required)")
	resolution := flag.String("resolution", "720p", "Resolution: 512p, 720p, 1080p")
	fps := flag.Float64("fps", 2.0, "Target FPS (0.1-30)")
	sourceStream := flag.String("source", "test", "Source stream identifier")
	outputDir := flag.String("output", "", "Directory to save captured frames (optional)")
	outputFormat := flag.String("format", "png", "Output format: png, jpeg")
	jpegQuality := flag.Int("jpeg-quality", 90, "JPEG quality (1-100, only for jpeg format)")
	maxFrames := flag.Int("max-frames", 0, "Maximum frames to capture (0 = unlimited)")
	statsInterval := flag.Int("stats-interval", 10, "Seconds between stats reports")
	accel := flag.String("accel", "auto", "Acceleration mode: auto, vaapi, software")
	skipWarmup := flag.Bool("skip-warmup", false, "Skip FPS stability warmup")
	debug := flag.Bool("debug", false, "Enable debug logging")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	// Show version
	if *showVersion {
		fmt.Printf("test-capture %s\n", version)
		os.Exit(0)
	}

	// Validate required flags
	if *rtspURL == "" {
		fmt.Fprintf(os.Stderr, "Error: --url flag is required\n\n")
		fmt.Fprintf(os.Stderr, "Usage example:\n")
		fmt.Fprintf(os.Stderr, "  test-capture --url rtsp://192.168.1.100/stream\n")
		fmt.Fprintf(os.Stderr, "  test-capture --url rtsp://192.168.1.100/stream --output ./frames --fps 1.0\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Set up logging
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Parse resolution
	var res streamcapture.Resolution
	switch *resolution {
	case "512p":
		res = streamcapture.Res512p
	case "720p":
		res = streamcapture.Res720p
	case "1080p":
		res = streamcapture.Res1080p
	default:
		log.Fatalf("Invalid resolution: %s (must be 512p, 720p, or 1080p)", *resolution)
	}

	// Validate output format
	if *outputFormat != "png" && *outputFormat != "jpeg" {
		log.Fatalf("Invalid output format: %s (must be png or jpeg)", *outputFormat)
	}

	// Create output directory if specified
	if *outputDir != "" {
		if err := os.MkdirAll(*outputDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
		slog.Info("Frame saving enabled",
			"directory", *outputDir,
			"format", *outputFormat,
			"jpeg_quality", *jpegQuality,
		)
	}

	// Print banner
	fmt.Printf("\n")
	fmt.Printf("╔═══════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║         Stream Capture Test - Orion 2.0 Module           ║\n")
	fmt.Printf("║                      Version %s                        ║\n", version)
	fmt.Printf("╚═══════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  RTSP URL:      %s\n", *rtspURL)
	fmt.Printf("  Resolution:    %s\n", *resolution)
	fmt.Printf("  Target FPS:    %.2f\n", *fps)
	fmt.Printf("  Source Stream: %s\n", *sourceStream)
	if *outputDir != "" {
		fmt.Printf("  Output Dir:    %s\n", *outputDir)
	} else {
		fmt.Printf("  Output Dir:    (none - frames not saved)\n")
	}
	if *maxFrames > 0 {
		fmt.Printf("  Max Frames:    %d\n", *maxFrames)
	} else {
		fmt.Printf("  Max Frames:    unlimited\n")
	}
	fmt.Printf("\n")

	// Parse acceleration mode
	var accelMode streamcapture.HardwareAccel
	switch *accel {
	case "auto":
		accelMode = streamcapture.AccelAuto
	case "vaapi":
		accelMode = streamcapture.AccelVAAPI
	case "software":
		accelMode = streamcapture.AccelSoftware
	default:
		log.Fatalf("Invalid acceleration mode: %s (must be auto, vaapi, or software)", *accel)
	}

	// Create RTSP stream
	cfg := streamcapture.RTSPConfig{
		URL:          *rtspURL,
		Resolution:   res,
		TargetFPS:    *fps,
		SourceStream: *sourceStream,
		Acceleration: accelMode,
	}

	stream, err := streamcapture.NewRTSPStream(cfg)
	if err != nil {
		log.Fatalf("Failed to create RTSP stream: %v", err)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start stream (non-blocking, returns immediately)
	slog.Info("Starting RTSP stream...")
	frameChan, err := stream.Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}

	slog.Info("Stream started successfully")

	// Warmup: measure FPS stability before processing frames
	if !*skipWarmup {
		fmt.Printf("\n")
		fmt.Printf("Running warmup (5 seconds) to measure stream stability...\n")
		warmupStats, err := stream.Warmup(ctx, 5*time.Second)
		if err != nil {
			log.Fatalf("Warmup failed: %v", err)
		}

		fmt.Printf("\n")
		fmt.Printf("╭─────────────────────────────────────────────────────────╮\n")
		fmt.Printf("│ Warmup Complete\n")
		fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
		fmt.Printf("│ Frames Received:    %6d frames\n", warmupStats.FramesReceived)
		fmt.Printf("│ Duration:           %6.1f seconds\n", warmupStats.Duration.Seconds())
		fmt.Printf("│ FPS Mean:           %6.2f fps\n", warmupStats.FPSMean)
		fmt.Printf("│ FPS StdDev:         %6.2f fps\n", warmupStats.FPSStdDev)
		fmt.Printf("│ FPS Range:          %6.1f - %.1f fps\n", warmupStats.FPSMin, warmupStats.FPSMax)
		fmt.Printf("│ Jitter Mean:        %6.3f s\n", warmupStats.JitterMean)
		fmt.Printf("│ Jitter Max:         %6.3f s\n", warmupStats.JitterMax)
		fmt.Printf("│ Stable:             %6v\n", warmupStats.IsStable)
		fmt.Printf("╰─────────────────────────────────────────────────────────╯\n")

		if !warmupStats.IsStable {
			fmt.Printf("\n⚠️  WARNING: Stream is unstable (high FPS variance or jitter)\n")
		}

		fmt.Printf("\n")
	}
	fmt.Printf("Starting frame capture...\n")
	fmt.Printf("Press Ctrl+C to stop gracefully\n")
	fmt.Printf("═══════════════════════════════════════════════════════════\n\n")

	// Stats tracking
	startTime := time.Now()
	framesSaved := 0
	framesDropped := 0

	// Launch stats reporter goroutine
	statsTicker := time.NewTicker(time.Duration(*statsInterval) * time.Second)
	defer statsTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsTicker.C:
				stats := stream.Stats()
				uptime := time.Since(startTime)

				fmt.Printf("\n")
				fmt.Printf("╭─────────────────────────────────────────────────────────╮\n")
				fmt.Printf("│ Stream Statistics (Uptime: %s)\n", uptime.Round(time.Second))
				fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
				fmt.Printf("│ Frames Captured:    %6d frames\n", stats.FrameCount)
				fmt.Printf("│ Frames Saved:       %6d frames\n", framesSaved)
				// Show stream-level drops (from channel full)
				if stats.FramesDropped > 0 || stats.DropRate > 0 {
					fmt.Printf("│ Stream Drops:       %6d frames (%.1f%%)\n", stats.FramesDropped, stats.DropRate)
				}
				// Show local drops (from save failures)
				if *outputDir != "" && stats.FrameCount > 0 {
					dropRate := float64(framesDropped) / float64(stats.FrameCount) * 100
					fmt.Printf("│ Save Drops:         %6d frames (%.1f%%)\n", framesDropped, dropRate)
				}
				fmt.Printf("│ Target FPS:         %6.2f fps\n", stats.FPSTarget)
				fmt.Printf("│ Real FPS:           %6.2f fps\n", stats.FPSReal)
				fmt.Printf("│ Latency:            %6d ms\n", stats.LatencyMS)
				fmt.Printf("│ Bytes Read:         %6.2f MB\n", float64(stats.BytesRead)/1024/1024)
				fmt.Printf("│ Reconnects:         %6d\n", stats.Reconnects)
				fmt.Printf("│ Connected:          %6v\n", stats.IsConnected)
				// Show VAAPI telemetry if hardware acceleration is active
				if stats.UsingVAAPI {
					fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
					fmt.Printf("│ VAAPI Decode Latency Telemetry\n")
					fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
					fmt.Printf("│ Mean Latency:       %6.2f ms\n", stats.DecodeLatencyMeanMS)
					fmt.Printf("│ P95 Latency:        %6.2f ms\n", stats.DecodeLatencyP95MS)
					fmt.Printf("│ Max Latency:        %6.2f ms\n", stats.DecodeLatencyMaxMS)
				}
				// Show error telemetry if any errors occurred
				totalErrors := stats.ErrorsNetwork + stats.ErrorsCodec + stats.ErrorsAuth + stats.ErrorsUnknown
				if totalErrors > 0 {
					fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
					fmt.Printf("│ Error Telemetry\n")
					fmt.Printf("├─────────────────────────────────────────────────────────┤\n")
					fmt.Printf("│ Network Errors:     %6d\n", stats.ErrorsNetwork)
					fmt.Printf("│ Codec Errors:       %6d\n", stats.ErrorsCodec)
					fmt.Printf("│ Auth Errors:        %6d\n", stats.ErrorsAuth)
					fmt.Printf("│ Unknown Errors:     %6d\n", stats.ErrorsUnknown)
				}
				fmt.Printf("╰─────────────────────────────────────────────────────────╯\n")
				fmt.Printf("\n")
			}
		}
	}()

	// Main frame processing loop
	frameCount := 0
	for {
		select {
		case <-sigChan:
			fmt.Printf("\n\nReceived interrupt signal, shutting down...\n")
			cancel()
			goto shutdown

		case frame, ok := <-frameChan:
			if !ok {
				slog.Warn("Frame channel closed unexpectedly")
				goto shutdown
			}

			frameCount++

			// Log frame arrival (compact format)
			fmt.Printf("[%s] Frame #%-6d | Seq: %-8d | Size: %6.1f KB | Timestamp: %s\n",
				time.Now().Format("15:04:05"),
				frameCount,
				frame.Seq,
				float64(len(frame.Data))/1024,
				frame.Timestamp.Format("15:04:05.000"),
			)

			// Save frame if output directory specified
			if *outputDir != "" {
				if err := saveFrame(*outputDir, frame, *outputFormat, *jpegQuality); err != nil {
					slog.Error("Failed to save frame", "error", err, "seq", frame.Seq)
					framesDropped++
				} else {
					framesSaved++
				}
			}

			// Stop if max frames reached
			if *maxFrames > 0 && frameCount >= *maxFrames {
				fmt.Printf("\nReached maximum frames (%d), stopping...\n", *maxFrames)
				cancel()
				goto shutdown
			}
		}
	}

shutdown:
	slog.Info("Stopping stream...")
	if err := stream.Stop(); err != nil {
		slog.Error("Error stopping stream", "error", err)
	}

	// Final stats
	finalStats := stream.Stats()
	uptime := time.Since(startTime)

	fmt.Printf("\n")
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("                     Final Statistics                      \n")
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("  Total Uptime:       %s\n", uptime.Round(time.Second))
	fmt.Printf("  Frames Captured:    %d frames\n", finalStats.FrameCount)
	if *outputDir != "" {
		fmt.Printf("  Frames Saved:       %d frames\n", framesSaved)
		fmt.Printf("  Frames Dropped:     %d frames\n", framesDropped)
		fmt.Printf("  Save Success Rate:  %.1f%%\n", float64(framesSaved)/float64(finalStats.FrameCount)*100)
	}
	fmt.Printf("  Average FPS:        %.2f fps\n", finalStats.FPSReal)
	fmt.Printf("  Bytes Read:         %.2f MB\n", float64(finalStats.BytesRead)/1024/1024)
	fmt.Printf("  Reconnection Count: %d\n", finalStats.Reconnects)
	if finalStats.UsingVAAPI {
		fmt.Printf("─────────────────────────────────────────────────────────\n")
		fmt.Printf("  VAAPI Acceleration: Active\n")
		fmt.Printf("  Mean Decode Latency: %.2f ms\n", finalStats.DecodeLatencyMeanMS)
		fmt.Printf("  P95 Decode Latency:  %.2f ms\n", finalStats.DecodeLatencyP95MS)
		fmt.Printf("  Max Decode Latency:  %.2f ms\n", finalStats.DecodeLatencyMaxMS)
	}
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("\n")

	slog.Info("Test capture completed successfully")
}

// saveFrame saves a frame to disk as PNG or JPEG
func saveFrame(outputDir string, frame streamcapture.Frame, format string, jpegQuality int) error {
	// Create filename with timestamp and sequence
	ext := format
	filename := fmt.Sprintf("frame_%06d_%s.%s", frame.Seq, frame.Timestamp.Format("20060102_150405.000"), ext)
	filepath := filepath.Join(outputDir, filename)

	// Convert raw RGB bytes to image.Image
	img := &image.RGBA{
		Pix:    make([]uint8, len(frame.Data)+frame.Width*frame.Height), // RGBA needs alpha channel
		Stride: frame.Width * 4,
		Rect:   image.Rect(0, 0, frame.Width, frame.Height),
	}

	// Convert RGB to RGBA (add alpha = 255)
	for i := 0; i < frame.Width*frame.Height; i++ {
		img.Pix[i*4+0] = frame.Data[i*3+0] // R
		img.Pix[i*4+1] = frame.Data[i*3+1] // G
		img.Pix[i*4+2] = frame.Data[i*3+2] // B
		img.Pix[i*4+3] = 255               // A (opaque)
	}

	// Create output file
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Encode based on format
	switch format {
	case "png":
		if err := png.Encode(file, img); err != nil {
			return fmt.Errorf("failed to encode PNG: %w", err)
		}
	case "jpeg":
		if err := jpeg.Encode(file, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return fmt.Errorf("failed to encode JPEG: %w", err)
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
}
