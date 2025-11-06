package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/framesupplier"
	streamcapture "github.com/e7canasta/orion-care-sensor/modules/stream-capture"
)

// reportStats periodically prints statistics from all pipeline components
func reportStats(
	ctx context.Context,
	config Config,
	stream streamcapture.StreamProvider,
	supplier framesupplier.Supplier,
	workers []*MockWorker,
	frameSaver *FrameSaver,
	logger *slog.Logger,
) {
	ticker := time.NewTicker(config.StatsInterval)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			uptime := time.Since(startTime)
			printLiveStats(uptime, stream, supplier, workers, frameSaver)
		}
	}
}

// printLiveStats prints current statistics from all components
func printLiveStats(
	uptime time.Duration,
	stream streamcapture.StreamProvider,
	supplier framesupplier.Supplier,
	workers []*MockWorker,
	frameSaver *FrameSaver,
) {
	// Stream stats
	streamStats := stream.Stats()

	// FrameSupplier stats
	supplierStats := supplier.Stats()

	// Worker stats
	workerStats := make([]WorkerStats, len(workers))
	for i, w := range workers {
		workerStats[i] = w.Stats()
	}

	// Print formatted stats
	fmt.Println()
	fmt.Println("╭─────────────────────────────────────────────────────────────────╮")
	fmt.Printf("│ Pipeline Statistics (Uptime: %v)\n", uptime.Round(time.Second))
	fmt.Println("├─────────────────────────────────────────────────────────────────┤")

	// Stream Capture Stats
	fmt.Println("│ Stream Capture:")
	fmt.Printf("│   Frames Captured:    %6d frames\n", streamStats.FrameCount)
	fmt.Printf("│   Frames Dropped:     %6d frames (%.1f%%)\n",
		streamStats.FramesDropped,
		streamStats.DropRate)
	fmt.Printf("│   Target FPS:         %6.2f fps\n", streamStats.FPSTarget)
	fmt.Printf("│   Real FPS:           %6.2f fps\n", streamStats.FPSReal)
	fmt.Printf("│   Latency:            %6d ms\n", streamStats.LatencyMS)
	fmt.Printf("│   Reconnects:         %6d\n", streamStats.Reconnects)
	fmt.Printf("│   Connected:          %6v\n", streamStats.IsConnected)

	// Frame Saving Stats (if enabled)
	if frameSaver != nil {
		saved, dropped := frameSaver.Stats()
		total := saved + dropped
		saveRate := 100.0
		if total > 0 {
			saveRate = float64(saved) / float64(total) * 100.0
		}
		fmt.Println("│")
		fmt.Println("│ Frame Saving:")
		fmt.Printf("│   Frames Saved:       %6d frames\n", saved)
		fmt.Printf("│   Save Drops:         %6d frames (%.1f%% success)\n", dropped, saveRate)
	}

	// FrameSupplier Stats
	fmt.Println("│")
	fmt.Println("│ FrameSupplier:")
	fmt.Printf("│   Inbox Drops:        %6d\n", supplierStats.InboxDrops)
	fmt.Printf("│   Active Workers:     %6d\n", len(supplierStats.Workers))

	// Detect idle workers
	idleWorkers := detectIdleWorkers(supplierStats)
	if len(idleWorkers) > 0 {
		fmt.Printf("│   Idle Workers:       ")
		for i, id := range idleWorkers {
			if i > 0 {
				fmt.Print(", ")
			}
			idleDuration := time.Since(supplierStats.Workers[id].LastConsumedAt)
			fmt.Printf("%s (%.1fs)", id, idleDuration.Seconds())
		}
		fmt.Println()
	}

	// Distribution method
	activeWorkers := len(supplierStats.Workers)
	if activeWorkers >= 8 {
		fmt.Printf("│   Distribution:       Batched (threshold=8)\n")
	} else {
		fmt.Printf("│   Distribution:       Sequential (<%d workers)\n", 8)
	}

	// Worker Stats
	fmt.Println("│")
	fmt.Println("│ Workers:")
	for _, ws := range workerStats {
		// Get mailbox stats from supplier
		mailboxStats, exists := supplierStats.Workers[ws.ID]
		var mailboxDrops uint64
		if exists {
			mailboxDrops = mailboxStats.TotalDrops
		}

		fmt.Printf("│   %-15s: %4d processed, %3d drops (%.1f%%), avg=%3dms\n",
			ws.ID,
			ws.Processed,
			mailboxDrops,
			dropRateFromCounts(ws.Processed, mailboxDrops),
			ws.AvgLatency.Milliseconds())
	}

	fmt.Println("╰─────────────────────────────────────────────────────────────────╯")
	fmt.Println()
}

// printFinalStats prints final statistics at shutdown
func printFinalStats(
	stream streamcapture.StreamProvider,
	supplier framesupplier.Supplier,
	workers []*MockWorker,
	frameSaver *FrameSaver,
) {
	streamStats := stream.Stats()
	supplierStats := supplier.Stats()

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("                     Final Statistics                         ")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	// Stream stats
	fmt.Printf("  Frames Captured:       %d frames\n", streamStats.FrameCount)
	fmt.Printf("  Stream Drops:          %d frames (%.1f%%)\n",
		streamStats.FramesDropped,
		streamStats.DropRate)
	fmt.Printf("  Average FPS:           %.2f fps\n", streamStats.FPSReal)
	fmt.Printf("  Reconnection Count:    %d\n", streamStats.Reconnects)

	// Frame Saving stats (if enabled)
	if frameSaver != nil {
		saved, dropped := frameSaver.Stats()
		total := saved + dropped
		saveRate := 100.0
		if total > 0 {
			saveRate = float64(saved) / float64(total) * 100.0
		}
		fmt.Println()
		fmt.Printf("  Frames Saved:          %d (%.1f%% success)\n", saved, saveRate)
		if dropped > 0 {
			fmt.Printf("  Save Drops:            %d\n", dropped)
		}
	}

	// FrameSupplier stats
	fmt.Println()
	fmt.Printf("  Inbox Drops:           %d\n", supplierStats.InboxDrops)
	fmt.Printf("  Active Workers:        %d\n", len(supplierStats.Workers))

	// Worker stats
	fmt.Println()
	fmt.Println("  Worker Summary:")
	for _, w := range workers {
		ws := w.Stats()
		mailboxStats, exists := supplierStats.Workers[ws.ID]
		var mailboxDrops uint64
		if exists {
			mailboxDrops = mailboxStats.TotalDrops
		}

		fmt.Printf("    %-15s: %d processed, %d drops (%.1f%%)\n",
			ws.ID,
			ws.Processed,
			mailboxDrops,
			dropRateFromCounts(ws.Processed, mailboxDrops))
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

// detectIdleWorkers returns IDs of workers that are marked as idle
func detectIdleWorkers(stats framesupplier.SupplierStats) []string {
	var idle []string
	for id, ws := range stats.Workers {
		if ws.IsIdle {
			idle = append(idle, id)
		}
	}
	return idle
}

// dropRate calculates drop percentage
func dropRate(total, drops uint64) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(drops) / float64(total) * 100.0
}

// dropRateFromCounts calculates drop rate from processed + drops
func dropRateFromCounts(processed, drops uint64) float64 {
	total := processed + drops
	if total == 0 {
		return 0.0
	}
	return float64(drops) / float64(total) * 100.0
}
