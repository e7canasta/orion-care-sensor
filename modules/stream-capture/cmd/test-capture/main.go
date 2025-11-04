package main

import (
	"context"
	"flag"
	"fmt"
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
	maxFrames := flag.Int("max-frames", 0, "Maximum frames to capture (0 = unlimited)")
	statsInterval := flag.Int("stats-interval", 10, "Seconds between stats reports")
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

	// Create output directory if specified
	if *outputDir != "" {
		if err := os.MkdirAll(*outputDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
		slog.Info("Frame saving enabled", "directory", *outputDir)
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

	// Create RTSP stream
	cfg := streamcapture.RTSPConfig{
		URL:          *rtspURL,
		Resolution:   res,
		TargetFPS:    *fps,
		SourceStream: *sourceStream,
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

	// Start stream
	slog.Info("Starting RTSP stream (warm-up will take ~5 seconds)...")
	frameChan, err := stream.Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}

	slog.Info("Stream started successfully, capturing frames...")
	fmt.Printf("\n")
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
				if *outputDir != "" && stats.FrameCount > 0 {
					dropRate := float64(framesDropped) / float64(stats.FrameCount) * 100
					fmt.Printf("│ Frames Dropped:     %6d frames (%.1f%%)\n", framesDropped, dropRate)
				}
				fmt.Printf("│ Target FPS:         %6.2f fps\n", stats.FPSTarget)
				fmt.Printf("│ Real FPS:           %6.2f fps\n", stats.FPSReal)
				fmt.Printf("│ Latency:            %6d ms\n", stats.LatencyMS)
				fmt.Printf("│ Bytes Read:         %6.2f MB\n", float64(stats.BytesRead)/1024/1024)
				fmt.Printf("│ Reconnects:         %6d\n", stats.Reconnects)
				fmt.Printf("│ Connected:          %6v\n", stats.IsConnected)
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
				if err := saveFrame(*outputDir, frame); err != nil {
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
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("\n")

	slog.Info("Test capture completed successfully")
}

// saveFrame saves a frame to disk as a raw RGB file
func saveFrame(outputDir string, frame streamcapture.Frame) error {
	// Create filename with timestamp and sequence
	filename := fmt.Sprintf("frame_%06d_%s.rgb", frame.Seq, frame.Timestamp.Format("20060102_150405.000"))
	filepath := filepath.Join(outputDir, filename)

	// Write frame data
	if err := os.WriteFile(filepath, frame.Data, 0644); err != nil {
		return fmt.Errorf("failed to write frame: %w", err)
	}

	return nil
}
