package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/framesupplier"
	streamcapture "github.com/e7canasta/orion-care-sensor/modules/stream-capture"
)

const (
	version = "v0.1.0"
)

// Configuration for the pipeline example
type Config struct {
	// Stream source
	RTSPUrl    string
	Resolution streamcapture.Resolution
	FPS        float64

	// Workers
	WorkerProfiles []WorkerProfile

	// Frame saving (optional)
	OutputDir   string
	OutputFormat string
	JPEGQuality int

	// Statistics
	StatsInterval time.Duration

	// Logging
	Debug bool
}

// WorkerProfile defines characteristics of a mock worker
type WorkerProfile struct {
	ID      string
	Latency time.Duration
	SLA     string // "Critical", "Normal", "BestEffort"
}

func main() {
	config := parseFlags()

	// Setup logging
	logLevel := slog.LevelInfo
	if config.Debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Print banner
	printBanner(config)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Shutdown signal received, stopping gracefully...")
		cancel()
	}()

	// Run pipeline
	if err := runPipeline(ctx, config, logger); err != nil && err != context.Canceled {
		logger.Error("Pipeline failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Pipeline stopped gracefully")
}

func parseFlags() Config {
	var config Config

	// Stream flags
	flag.StringVar(&config.RTSPUrl, "url", "", "RTSP stream URL (required)")
	resolutionStr := flag.String("resolution", "720p", "Stream resolution (512p, 720p, 1080p)")
	flag.Float64Var(&config.FPS, "fps", 1.0, "Target FPS")

	// Frame saving flags (optional)
	flag.StringVar(&config.OutputDir, "output", "", "Output directory to save frames (optional)")
	flag.StringVar(&config.OutputFormat, "format", "png", "Output format: png or jpeg")
	flag.IntVar(&config.JPEGQuality, "jpeg-quality", 90, "JPEG quality (1-100, only for JPEG)")

	// Stats flags
	var statsIntervalSec int
	flag.IntVar(&statsIntervalSec, "stats-interval", 5, "Statistics reporting interval (seconds)")

	// Debug flag
	flag.BoolVar(&config.Debug, "debug", false, "Enable debug logging")

	flag.Parse()

	// Validation
	if config.RTSPUrl == "" {
		fmt.Fprintf(os.Stderr, "Error: --url is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Parse resolution
	switch *resolutionStr {
	case "512p":
		config.Resolution = streamcapture.Res512p
	case "720p":
		config.Resolution = streamcapture.Res720p
	case "1080p":
		config.Resolution = streamcapture.Res1080p
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid resolution %s (must be 512p, 720p, or 1080p)\n", *resolutionStr)
		os.Exit(1)
	}

	// Validate output format (if output enabled)
	if config.OutputDir != "" {
		if config.OutputFormat != "png" && config.OutputFormat != "jpeg" {
			fmt.Fprintf(os.Stderr, "Error: invalid format %s (must be png or jpeg)\n", config.OutputFormat)
			os.Exit(1)
		}
		if config.JPEGQuality < 1 || config.JPEGQuality > 100 {
			fmt.Fprintf(os.Stderr, "Error: invalid JPEG quality %d (must be 1-100)\n", config.JPEGQuality)
			os.Exit(1)
		}
	}

	config.StatsInterval = time.Duration(statsIntervalSec) * time.Second

	// Define worker profiles (3 workers with different latencies)
	config.WorkerProfiles = []WorkerProfile{
		{ID: "Worker-Fast", Latency: 10 * time.Millisecond, SLA: "Critical"},
		{ID: "Worker-Medium", Latency: 50 * time.Millisecond, SLA: "Normal"},
		{ID: "Worker-Slow", Latency: 200 * time.Millisecond, SLA: "BestEffort"},
	}

	return config
}

func runPipeline(ctx context.Context, config Config, logger *slog.Logger) error {
	// 1. Create stream provider (RTSP only for now)
	logger.Info("Creating RTSP stream provider", "url", config.RTSPUrl)
	stream, err := streamcapture.NewRTSPStream(
		streamcapture.RTSPConfig{
			URL:          config.RTSPUrl,
			Resolution:   config.Resolution,
			TargetFPS:    config.FPS,
			SourceStream: "pipeline-example",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create stream provider: %w", err)
	}

	// 2. Create FrameSaver (optional)
	var frameSaver *FrameSaver
	if config.OutputDir != "" {
		frameSaver, err = NewFrameSaver(config.OutputDir, config.OutputFormat, config.JPEGQuality)
		if err != nil {
			return fmt.Errorf("failed to create frame saver: %w", err)
		}
		logger.Info("Frame saving enabled",
			"output_dir", config.OutputDir,
			"format", config.OutputFormat,
			"jpeg_quality", config.JPEGQuality)
	}

	// 3. Create FrameSupplier
	supplier := framesupplier.New()

	// 3. Create and start mock workers
	workers := make([]*MockWorker, len(config.WorkerProfiles))
	for i, profile := range config.WorkerProfiles {
		worker := NewMockWorker(profile.ID, profile.Latency, logger)
		workers[i] = worker

		// Start worker goroutine
		go func(w *MockWorker) {
			if err := w.Run(ctx, supplier); err != nil && err != context.Canceled {
				logger.Error("Worker failed", "worker", w.id, "error", err)
			}
		}(worker)

		logger.Info("Worker started", "id", profile.ID, "latency", profile.Latency, "sla", profile.SLA)
	}

	// 4. Start FrameSupplier distribution loop
	go func() {
		if err := supplier.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("FrameSupplier failed", "error", err)
		}
	}()
	logger.Info("FrameSupplier started")

	// 5. Start stream provider
	frameChan, err := stream.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}
	logger.Info("Stream provider started")

	// 6. Start producer goroutine (stream → supplier)
	go produceFrames(ctx, frameChan, supplier, frameSaver, logger)

	// 7. Start statistics reporter
	go reportStats(ctx, config, stream, supplier, workers, frameSaver, logger)

	// 8. Wait for context cancellation
	<-ctx.Done()

	// 9. Stop stream provider
	if err := stream.Stop(); err != nil {
		logger.Error("Failed to stop stream gracefully", "error", err)
	}

	// 10. Stop FrameSupplier
	if err := supplier.Stop(); err != nil {
		logger.Error("Failed to stop supplier gracefully", "error", err)
	}

	// Print final stats
	printFinalStats(stream, supplier, workers, frameSaver)

	return ctx.Err()
}

// produceFrames reads frames from stream channel and publishes to FrameSupplier
func produceFrames(ctx context.Context, frameChan <-chan streamcapture.Frame, supplier framesupplier.Supplier, frameSaver *FrameSaver, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case streamFrame, ok := <-frameChan:
			if !ok {
				logger.Info("Stream channel closed")
				return
			}

			// Convert to FrameSupplier.Frame (minimal structure)
			supplierFrame := &framesupplier.Frame{
				Data:      streamFrame.Data,
				Width:     streamFrame.Width,
				Height:    streamFrame.Height,
				Timestamp: streamFrame.Timestamp,
				// Seq will be assigned by FrameSupplier distribution loop
			}

			// Publish to FrameSupplier (non-blocking)
			supplier.Publish(supplierFrame)

			// Save frame to disk (optional, if enabled)
			if frameSaver != nil {
				// Save in background to not block distribution
				go func(f *framesupplier.Frame) {
					if err := frameSaver.SaveFrame(f); err != nil {
						logger.Error("Failed to save frame", "error", err)
					}
				}(supplierFrame)
			}

			logger.Debug("Frame published to supplier",
				"stream_seq", streamFrame.Seq,
				"size_kb", len(supplierFrame.Data)/1024)
		}
	}
}

func printBanner(config Config) {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║    Orion Pipeline Example - Module Composition Demo          ║")
	fmt.Printf("║                    Version %-30s ║\n", version)
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Configuration:")

	fmt.Printf("  Stream Source:   RTSP (%s)\n", config.RTSPUrl)
	fmt.Printf("  Resolution:      %s\n", config.Resolution)
	fmt.Printf("  Target FPS:      %.2f fps\n", config.FPS)
	fmt.Printf("  Workers:         %d\n", len(config.WorkerProfiles))

	for _, profile := range config.WorkerProfiles {
		fmt.Printf("    - %-15s: %6s latency (%s)\n",
			profile.ID, profile.Latency, profile.SLA)
	}

	fmt.Printf("  Stats Interval:  %v\n", config.StatsInterval)
	fmt.Println()
	fmt.Println("Pipeline:")
	fmt.Println("  stream-capture → FrameSupplier → Workers")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop gracefully")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}
