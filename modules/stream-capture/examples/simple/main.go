package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	streamcapture "github.com/e7canasta/orion-care-sensor/modules/stream-capture"
)

func main() {
	// Parse command-line flags
	rtspURL := flag.String("url", "", "RTSP stream URL (required)")
	resolution := flag.String("resolution", "720p", "Resolution: 512p, 720p, 1080p")
	fps := flag.Float64("fps", 2.0, "Target FPS (0.1-30)")
	sourceStream := flag.String("source", "LQ", "Source stream identifier (e.g., LQ, HQ)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Validate required flags
	if *rtspURL == "" {
		log.Fatal("Error: --url flag is required\n\nUsage example:\n  go run examples/simple_capture.go --url rtsp://192.168.1.100/stream")
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

	// Create stream configuration
	cfg := streamcapture.RTSPConfig{
		URL:          *rtspURL,
		Resolution:   res,
		TargetFPS:    *fps,
		SourceStream: *sourceStream,
	}

	fmt.Printf("🎥 Stream Capture Example\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("URL:        %s\n", *rtspURL)
	fmt.Printf("Resolution: %s\n", *resolution)
	fmt.Printf("Target FPS: %.1f Hz\n", *fps)
	fmt.Printf("Source:     %s\n", *sourceStream)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Create RTSP stream
	slog.Info("Creating RTSP stream...")
	stream, err := streamcapture.NewRTSPStream(cfg)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start stream (blocks during 5s warm-up)
	slog.Info("Starting stream (warm-up will take ~5 seconds)...")
	frameChan, err := stream.Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}

	fmt.Printf("\n✅ Stream started successfully!\n")
	fmt.Printf("Press Ctrl+C to stop\n\n")

	// Print statistics every 5 seconds
	statsTicker := time.NewTicker(5 * time.Second)
	defer statsTicker.Stop()

	// Frame counter
	frameCount := 0
	startTime := time.Now()

	// Main loop
	go func() {
		for {
			select {
			case <-statsTicker.C:
				stats := stream.Stats()
				elapsed := time.Since(startTime).Seconds()
				localFPS := 0.0
				if elapsed > 0 {
					localFPS = float64(frameCount) / elapsed
				}

				fmt.Printf("📊 Statistics:\n")
				fmt.Printf("   Frames captured:  %d (local: %d)\n", stats.FrameCount, frameCount)
				fmt.Printf("   FPS (target):     %.2f Hz\n", stats.FPSTarget)
				fmt.Printf("   FPS (measured):   %.2f Hz (stream) / %.2f Hz (local)\n", stats.FPSReal, localFPS)
				fmt.Printf("   Resolution:       %s\n", stats.Resolution)
				fmt.Printf("   Latency:          %d ms\n", stats.LatencyMS)
				fmt.Printf("   Reconnects:       %d\n", stats.Reconnects)
				fmt.Printf("   Bytes read:       %.2f MB\n", float64(stats.BytesRead)/(1024*1024))
				fmt.Printf("   Connected:        %v\n\n", stats.IsConnected)

			case <-ctx.Done():
				return
			}
		}
	}()

	// Consume frames
	for {
		select {
		case frame, ok := <-frameChan:
			if !ok {
				slog.Info("Frame channel closed")
				goto cleanup
			}

			frameCount++

			// Print frame info (only every 10th frame to avoid spam)
			if frameCount%10 == 0 {
				fmt.Printf("📷 Frame %d: seq=%d, size=%d KB, timestamp=%s, trace_id=%s\n",
					frameCount,
					frame.Seq,
					len(frame.Data)/1024,
					frame.Timestamp.Format("15:04:05.000"),
					frame.TraceID[:8], // First 8 chars of trace ID
				)
			}

		case <-sigChan:
			fmt.Printf("\n\n🛑 Interrupt received, shutting down...\n")
			goto cleanup
		}
	}

cleanup:
	// Stop stream
	slog.Info("Stopping stream...")
	if err := stream.Stop(); err != nil {
		log.Printf("Error stopping stream: %v", err)
	}

	// Final statistics
	stats := stream.Stats()
	elapsed := time.Since(startTime).Seconds()
	localFPS := 0.0
	if elapsed > 0 {
		localFPS = float64(frameCount) / elapsed
	}

	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📊 Final Statistics:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Total frames:     %d (stream) / %d (consumed)\n", stats.FrameCount, frameCount)
	fmt.Printf("Duration:         %.1f seconds\n", elapsed)
	fmt.Printf("Average FPS:      %.2f Hz (stream) / %.2f Hz (consumed)\n", stats.FPSReal, localFPS)
	fmt.Printf("Total bytes:      %.2f MB\n", float64(stats.BytesRead)/(1024*1024))
	fmt.Printf("Reconnections:    %d\n", stats.Reconnects)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("\n✅ Stream capture example completed successfully!\n")
}
