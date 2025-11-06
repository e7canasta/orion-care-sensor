package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/framesupplier"
	"gopkg.in/yaml.v3"
)

// RunManager handles timestamp-based run tracking
type RunManager struct {
	RunsDir   string
	RunPath   string
	OutputDir string    // outputs/ within run
	StartTime time.Time
	Config    RunConfig
	Stats     *RunStats
}

// RunConfig stores run configuration
type RunConfig struct {
	Run struct {
		Timestamp string   `yaml:"timestamp"`
		Command   string   `yaml:"command"`
		RunPath   string   `yaml:"run_path"`
	} `yaml:"run"`
	
	Config struct {
		FPS          float64       `yaml:"fps"`
		Loops        int           `yaml:"loops"`
		InputDir     string        `yaml:"input_dir"`
		Pattern      string        `yaml:"pattern"`
		WorkerDelays []string      `yaml:"worker_delays"`
		MQTTWorker   bool          `yaml:"mqtt_worker"`
		StatsEnabled bool          `yaml:"stats_enabled"`
	} `yaml:"config"`
}

// RunStats stores final run statistics
type RunStats struct {
	Duration       float64                `json:"duration_seconds"`
	TotalPublished uint64                 `json:"total_frames_published"`
	TotalLoops     int                    `json:"total_loops"`
	Workers        map[string]WorkerStats `json:"workers"`
	InboxDrops     uint64                 `json:"inbox_drops"`
}

type WorkerStats struct {
	DropRate       float64 `json:"drop_rate_percent"`
	TotalDrops     uint64  `json:"total_drops"`
	TotalConsumed  uint64  `json:"total_consumed"`
}

var globalRunManager *RunManager

func initRunManager() (*RunManager, error) {
	startTime := time.Now()
	
	rm := &RunManager{
		RunsDir:   *runsDir,
		StartTime: startTime,
		Stats:     &RunStats{Workers: make(map[string]WorkerStats)},
	}

	// Generate timestamp-based run name: YYYYMMDD_HHMMSS_NNN
	runPath, err := rm.generateRunPath()
	if err != nil {
		return nil, err
	}
	rm.RunPath = runPath
	rm.OutputDir = filepath.Join(runPath, "outputs")

	// Create run directory + outputs subdir
	if err := os.MkdirAll(rm.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create run dir: %w", err)
	}

	// Save config
	if err := rm.saveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// Create symlink to latest
	latestLink := filepath.Join(*runsDir, "latest")
	os.Remove(latestLink) // Remove old symlink
	os.Symlink(filepath.Base(runPath), latestLink)

	log.Printf("Run initialized: %s", runPath)
	return rm, nil
}

func (rm *RunManager) generateRunPath() (string, error) {
	// Format: YYYYMMDD_HHMMSS
	timestamp := rm.StartTime.Format("20060102_150405")
	
	// Find next sequence number for this timestamp
	seq := rm.getNextSequence(timestamp)
	
	// Final name: YYYYMMDD_HHMMSS_NNN
	runName := fmt.Sprintf("%s_%03d", timestamp, seq)
	return filepath.Join(rm.RunsDir, runName), nil
}

func (rm *RunManager) getNextSequence(timestamp string) int {
	entries, err := os.ReadDir(rm.RunsDir)
	if err != nil {
		return 1
	}

	maxSeq := 0
	// Pattern: 20251105_222015_001
	prefix := timestamp + "_"
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) {
			// Extract sequence number
			parts := strings.Split(name, "_")
			if len(parts) >= 3 {
				seqStr := parts[len(parts)-1]
				if seq, err := strconv.Atoi(seqStr); err == nil {
					if seq > maxSeq {
						maxSeq = seq
					}
				}
			}
		}
	}
	
	return maxSeq + 1
}

func (rm *RunManager) saveConfig() error {
	cfg := RunConfig{}
	cfg.Run.Timestamp = rm.StartTime.Format(time.RFC3339)
	cfg.Run.Command = strings.Join(os.Args, " ")
	cfg.Run.RunPath = rm.RunPath
	
	cfg.Config.FPS = *fps
	cfg.Config.Loops = *loops
	cfg.Config.InputDir = *inputDir
	cfg.Config.Pattern = *pattern
	cfg.Config.WorkerDelays = strings.Split(*workerDelays, ",")
	cfg.Config.MQTTWorker = *mqttWorker
	cfg.Config.StatsEnabled = *stats

	rm.Config = cfg

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	configPath := filepath.Join(rm.RunPath, "config.yaml")
	return os.WriteFile(configPath, data, 0644)
}

func (rm *RunManager) SaveFinalStats(supplier framesupplier.Supplier, loopCount int) error {
	if !*saveStats {
		return nil
	}

	rm.Stats.Duration = time.Since(rm.StartTime).Seconds()
	rm.Stats.TotalLoops = loopCount

	// Get final supplier stats
	supplierStats := supplier.Stats()
	rm.Stats.InboxDrops = supplierStats.InboxDrops

	for workerID, ws := range supplierStats.Workers {
		dropRate := 0.0
		if ws.LastConsumedSeq > 0 {
			dropRate = float64(ws.TotalDrops) / float64(ws.LastConsumedSeq) * 100
		}
		
		rm.Stats.Workers[workerID] = WorkerStats{
			DropRate:      dropRate,
			TotalDrops:    ws.TotalDrops,
			TotalConsumed: ws.LastConsumedSeq,
		}
		
		if ws.LastConsumedSeq > rm.Stats.TotalPublished {
			rm.Stats.TotalPublished = ws.LastConsumedSeq
		}
	}

	// Save stats.json
	data, err := json.MarshalIndent(rm.Stats, "", "  ")
	if err != nil {
		return err
	}

	statsPath := filepath.Join(rm.RunPath, "stats.json")
	if err := os.WriteFile(statsPath, data, 0644); err != nil {
		return err
	}

	log.Printf("Stats saved: %s", statsPath)
	return nil
}

var (
	inputDir     = flag.String("input", "data/frames", "Input directory with frame images")
	fps          = flag.Float64("fps", 30.0, "Frames per second to simulate")
	loops        = flag.Int("n", 0, "Number of loops (0 = infinite)")
	pattern      = flag.String("pattern", "*.png", "File pattern to match")
	stats        = flag.Bool("stats", false, "Enable real-time stats monitoring")
	workerDelays = flag.String("worker-delays", "20ms,50ms,100ms", "Inference delays per worker (comma-separated)")
	mqttWorker   = flag.Bool("mqtt-worker", false, "Add MQTT emitter worker")
	mqttBroker   = flag.String("mqtt-broker", "tcp://localhost:1883", "MQTT broker address")
	
	// Runs system (timestamp-based)
	runsDir   = flag.String("runs-dir", "runs", "Base runs directory")
	saveStats = flag.Bool("save-stats", true, "Save stats.json at end of run")
)

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Initialize run tracking
	runMgr, err := initRunManager()
	if err != nil {
		log.Fatalf("Failed to initialize run manager: %v", err)
	}
	globalRunManager = runMgr

	// Validate input
	if _, err := os.Stat(*inputDir); err != nil {
		log.Fatalf("Input directory not found: %s", *inputDir)
	}

	// Load frame list
	frames, err := loadFrames(*inputDir, *pattern)
	if err != nil {
		log.Fatalf("Failed to load frames: %v", err)
	}
	if len(frames) == 0 {
		log.Fatalf("No frames found in %s matching %s", *inputDir, *pattern)
	}
	log.Printf("Loaded %d frames from %s", len(frames), *inputDir)

	// Setup supplier
	supplier := framesupplier.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := supplier.Start(ctx); err != nil {
			log.Printf("Supplier stopped: %v", err)
		}
	}()

	// Parse worker delays
	delays, err := parseDelays(*workerDelays)
	if err != nil {
		log.Fatalf("Invalid worker-delays: %v", err)
	}

	// Create output workers (inside run directory)
	workers := []string{"worker-1", "worker-2", "worker-3"}
	for i, workerID := range workers {
		workerOutDir := filepath.Join(globalRunManager.OutputDir, workerID)
		if err := os.MkdirAll(workerOutDir, 0755); err != nil {
			log.Fatalf("Failed to create output dir %s: %v", workerOutDir, err)
		}
		delay := time.Duration(0)
		if i < len(delays) {
			delay = delays[i]
		}
		go runCopyWorker(supplier, workerID, workerOutDir, delay)
	}

	// Optional: MQTT worker
	if *mqttWorker {
		go runMQTTWorker(supplier, "mqtt-emitter", *mqttBroker)
	}

	// Start stats monitor (if enabled)
	if *stats {
		go statsMonitor(ctx, supplier, *fps)
	}

	// Start frame producer
	loopCount := 0
	producerDone := make(chan struct{})
	go func() {
		loopCount = produceFramesFromDisk(ctx, supplier, frames, *fps, *loops)
		close(producerDone)
	}()

	// Wait for completion or CTRL+C
	if *loops > 0 {
		<-producerDone
		log.Println("Producer finished, shutting down...")
		cancel()
	} else {
		<-ctx.Done()
		log.Println("Shutting down...")
	}
	
	time.Sleep(200 * time.Millisecond)
	
	// Save final stats
	if err := globalRunManager.SaveFinalStats(supplier, loopCount); err != nil {
		log.Printf("WARNING: Failed to save stats: %v", err)
	}
	
	log.Printf("Results saved to: %s", globalRunManager.RunPath)
}

// loadFrames returns sorted list of frame paths
func loadFrames(dir, pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

// produceFramesFromDisk simulates stream by reading files at target FPS
func produceFramesFromDisk(ctx context.Context, supplier framesupplier.Supplier, frames []string, fps float64, loops int) int {
	interval := time.Duration(float64(time.Second) / fps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	iteration := 0
	frameIdx := 0

	for {
		select {
		case <-ctx.Done():
			return iteration
		case <-ticker.C:
			// Read frame from disk
			framePath := frames[frameIdx]
			data, err := os.ReadFile(framePath)
			if err != nil {
				log.Printf("ERROR: Failed to read %s: %v", framePath, err)
				continue
			}

			// Publish to supplier
			frame := &framesupplier.Frame{
				Data:      data,
				Width:     1920, // TODO: detect from image
				Height:    1080,
				Timestamp: time.Now(),
			}
			supplier.Publish(frame)

			log.Printf("Published frame %d/%d from %s", 
				frameIdx+1, len(frames), filepath.Base(framePath))

			// Advance frame index
			frameIdx++
			if frameIdx >= len(frames) {
				frameIdx = 0
				iteration++
				log.Printf("--- Loop %d completed ---", iteration)

				// Check if we should stop
				if loops > 0 && iteration >= loops {
					log.Printf("Completed %d loops, stopping producer", loops)
					return iteration
				}
			}
		}
	}
}

// runCopyWorker consumes frames and copies them to output directory
func runCopyWorker(supplier framesupplier.Supplier, workerID, outDir string, inferenceDelay time.Duration) {
	readFunc := supplier.Subscribe(workerID)
	defer supplier.Unsubscribe(workerID)

	log.Printf("[%s] Worker started, writing to %s (inference delay: %v)", workerID, outDir, inferenceDelay)

	for {
		frame := readFunc()
		if frame == nil {
			log.Printf("[%s] Stopped", workerID)
			break
		}

		// Simulate inference delay
		if inferenceDelay > 0 {
			time.Sleep(inferenceDelay)
		}

		// Write frame to output
		outPath := filepath.Join(outDir, fmt.Sprintf("frame_%05d.png", frame.Seq))
		if err := os.WriteFile(outPath, frame.Data, 0644); err != nil {
			log.Printf("[%s] ERROR: Failed to write %s: %v", workerID, outPath, err)
			continue
		}

		if !*stats {
			log.Printf("[%s] Wrote frame seq=%d to %s", workerID, frame.Seq, filepath.Base(outPath))
		}
	}
}

// parseDelays parses comma-separated duration strings
func parseDelays(s string) ([]time.Duration, error) {
	parts := strings.Split(s, ",")
	delays := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		d, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid duration %q: %w", part, err)
		}
		delays = append(delays, d)
	}
	return delays, nil
}

// statsMonitor displays real-time statistics
func statsMonitor(ctx context.Context, supplier framesupplier.Supplier, targetFPS float64) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastStats := make(map[string]uint64) // workerID -> lastConsumedSeq
	lastTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Println() // New line after stats
			return
		case <-ticker.C:
			stats := supplier.Stats()
			now := time.Now()
			elapsed := now.Sub(lastTime).Seconds()

			// Calculate FPS from fastest worker (most accurate)
			maxSeqDelta := uint64(0)
			for workerID, ws := range stats.Workers {
				last := lastStats[workerID]
				delta := ws.LastConsumedSeq - last
				if delta > maxSeqDelta {
					maxSeqDelta = delta
				}
				lastStats[workerID] = ws.LastConsumedSeq
			}
			actualFPS := float64(maxSeqDelta) / elapsed

			// Worker drop rates
			workerStatsStr := make([]string, 0, len(stats.Workers))
			for workerID, ws := range stats.Workers {
				dropRate := 0.0
				if ws.LastConsumedSeq > 0 {
					dropRate = float64(ws.TotalDrops) / float64(ws.LastConsumedSeq) * 100
				}
				workerStatsStr = append(workerStatsStr, fmt.Sprintf("%s=%.1f%%", workerID, dropRate))
			}

			// Print stats line (overwrite previous)
			fmt.Printf("\r[STATS] FPS: %.1f/%.1f | Drops: %s | Inbox: %d     ",
				actualFPS, targetFPS, strings.Join(workerStatsStr, " "), stats.InboxDrops)

			lastTime = now
		}
	}
}

// runMQTTWorker consumes frames and publishes to MQTT (stub for now)
func runMQTTWorker(supplier framesupplier.Supplier, workerID, broker string) {
	readFunc := supplier.Subscribe(workerID)
	defer supplier.Unsubscribe(workerID)

	log.Printf("[%s] MQTT worker started, broker=%s (STUB - not publishing)", workerID, broker)

	for {
		frame := readFunc()
		if frame == nil {
			log.Printf("[%s] Stopped", workerID)
			break
		}

		// TODO: Publish to MQTT
		// For now, just log
		if !*stats {
			log.Printf("[%s] Would publish frame seq=%d to MQTT", workerID, frame.Seq)
		}
	}
}
