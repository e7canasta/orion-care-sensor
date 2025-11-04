package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	streamcapture "github.com/e7canasta/orion-care-sensor/modules/stream-capture"
)

func main() {
	// Parse command-line flags
	rtspURL := flag.String("url", "", "RTSP stream URL (required)")
	resolution := flag.String("resolution", "720p", "Resolution: 512p, 720p, 1080p")
	initialFPS := flag.Float64("fps", 2.0, "Initial target FPS (0.1-30)")
	sourceStream := flag.String("source", "LQ", "Source stream identifier")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Validate required flags
	if *rtspURL == "" {
		log.Fatal("Error: --url flag is required\n\nUsage example:\n  go run examples/hot_reload.go --url rtsp://192.168.1.100/stream")
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
		log.Fatalf("Invalid resolution: %s", *resolution)
	}

	// Create stream configuration
	cfg := streamcapture.RTSPConfig{
		URL:          *rtspURL,
		Resolution:   res,
		TargetFPS:    *initialFPS,
		SourceStream: *sourceStream,
	}

	fmt.Printf("🔥 Hot-Reload FPS Example\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("URL:         %s\n", *rtspURL)
	fmt.Printf("Resolution:  %s\n", *resolution)
	fmt.Printf("Initial FPS: %.1f Hz\n", *initialFPS)
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

	// Start stream (blocks during warm-up)
	slog.Info("Starting stream (warm-up ~5s)...")
	frameChan, err := stream.Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}

	fmt.Printf("\n✅ Stream started successfully!\n\n")
	printHelp()

	// Print statistics every 3 seconds
	statsTicker := time.NewTicker(3 * time.Second)
	defer statsTicker.Stop()

	// Frame counter
	frameCount := 0
	startTime := time.Now()

	// Launch goroutine to consume frames
	go func() {
		for frame := range frameChan {
			frameCount++
			if frameCount%10 == 0 {
				fmt.Printf("📷 Frame %d received (seq=%d, trace=%s)\n",
					frameCount, frame.Seq, frame.TraceID[:8])
			}
		}
	}()

	// Launch goroutine for stats
	go func() {
		for {
			select {
			case <-statsTicker.C:
				printStats(stream, frameCount, time.Since(startTime).Seconds())
			case <-ctx.Done():
				return
			}
		}
	}()

	// Interactive command loop
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			fmt.Print("> ")
			continue
		}

		// Parse command
		parts := strings.Fields(input)
		cmd := parts[0]

		switch cmd {
		case "fps", "set_fps":
			if len(parts) < 2 {
				fmt.Println("❌ Usage: fps <value>  (example: fps 0.5)")
				break
			}

			newFPS, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				fmt.Printf("❌ Invalid FPS value: %v\n", err)
				break
			}

			fmt.Printf("\n🔄 Changing FPS from %.2f to %.2f Hz...\n", stream.Stats().FPSTarget, newFPS)
			fmt.Println("   (expect ~2 second interruption)")

			before := time.Now()
			err = stream.SetTargetFPS(newFPS)
			elapsed := time.Since(before)

			if err != nil {
				fmt.Printf("❌ Failed to set FPS: %v\n\n", err)
			} else {
				fmt.Printf("✅ FPS updated successfully!\n")
				fmt.Printf("   Interruption time: %v\n\n", elapsed.Round(time.Millisecond))
			}

		case "stats", "s":
			printStats(stream, frameCount, time.Since(startTime).Seconds())

		case "help", "h":
			printHelp()

		case "quit", "exit", "q":
			fmt.Println("\n🛑 Shutting down...")
			goto cleanup

		default:
			fmt.Printf("❌ Unknown command: %s\n", cmd)
			fmt.Println("   Type 'help' for available commands")
		}

		fmt.Print("> ")
	}

cleanup:
	cancel()

	// Stop stream
	slog.Info("Stopping stream...")
	if err := stream.Stop(); err != nil {
		log.Printf("Error stopping stream: %v", err)
	}

	// Final statistics
	printStats(stream, frameCount, time.Since(startTime).Seconds())

	fmt.Printf("\n✅ Hot-reload example completed!\n")
}

func printHelp() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📖 Available Commands:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  fps <value>    Change target FPS (0.1-30)")
	fmt.Println("                 Examples: fps 0.5, fps 1.0, fps 5.0")
	fmt.Println()
	fmt.Println("  stats          Show current stream statistics")
	fmt.Println("  help           Show this help message")
	fmt.Println("  quit           Exit the program")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}

func printStats(stream *streamcapture.RTSPStream, localFrames int, elapsed float64) {
	stats := stream.Stats()

	localFPS := 0.0
	if elapsed > 0 {
		localFPS = float64(localFrames) / elapsed
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 Stream Statistics:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Target FPS:     %.2f Hz\n", stats.FPSTarget)
	fmt.Printf("  Measured FPS:   %.2f Hz (stream) / %.2f Hz (consumed)\n", stats.FPSReal, localFPS)
	fmt.Printf("  Frames:         %d (stream) / %d (consumed)\n", stats.FrameCount, localFrames)
	fmt.Printf("  Resolution:     %s\n", stats.Resolution)
	fmt.Printf("  Latency:        %d ms\n", stats.LatencyMS)
	fmt.Printf("  Reconnects:     %d\n", stats.Reconnects)
	fmt.Printf("  Data read:      %.2f MB\n", float64(stats.BytesRead)/(1024*1024))
	fmt.Printf("  Connected:      %v\n", stats.IsConnected)
	fmt.Printf("  Uptime:         %.1f seconds\n", elapsed)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}
