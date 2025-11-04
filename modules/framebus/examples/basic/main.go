// Package main demonstrates basic FrameBus usage with simulated video processing.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/visiona/orion/modules/framebus"
)

func main() {
	fmt.Println("FrameBus Example: Simulated Video Processing")
	fmt.Println("==============================================\n")

	// Create bus
	bus := framebus.New()
	defer bus.Close()

	// Create 3 workers with different processing speeds
	worker1 := startWorker("fast-worker", bus, 5*time.Millisecond)
	worker2 := startWorker("medium-worker", bus, 20*time.Millisecond)
	worker3 := startWorker("slow-worker", bus, 50*time.Millisecond)

	// Simulate video stream at 30 FPS (33ms per frame)
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	frameCount := uint64(0)
	done := make(chan bool)

	// Stats reporter
	go reportStats(bus, 2*time.Second, done)

	// Publish frames for 10 seconds
	go func() {
		for range ticker.C {
			frameCount++
			frame := framebus.Frame{
				Seq:       frameCount,
				Data:      []byte(fmt.Sprintf("frame-%d", frameCount)),
				Timestamp: time.Now(),
				Metadata: map[string]string{
					"source": "simulated-camera",
					"fps":    "30",
				},
			}

			bus.Publish(frame)

			if frameCount >= 300 { // 10 seconds @ 30 FPS
				break
			}
		}
		done <- true
	}()

	// Wait for completion
	<-done

	// Final stats
	fmt.Println("\n=== Final Statistics ===")
	stats := bus.Stats()
	printStats(stats)
	
	// Show drop rates with helper functions
	fmt.Printf("\nGlobal Drop Rate: %.2f%%\n", framebus.CalculateDropRate(stats)*100)
	fmt.Println("\nPer-Worker Drop Rates:")
	for id := range stats.Subscribers {
		rate := framebus.CalculateSubscriberDropRate(stats, id)
		fmt.Printf("  %s: %.2f%%\n", id, rate*100)
	}

	// Stop workers
	close(worker1)
	close(worker2)
	close(worker3)

	time.Sleep(100 * time.Millisecond) // Let workers finish
}

// startWorker creates a worker that processes frames from the bus.
func startWorker(id string, bus framebus.Bus, processingTime time.Duration) chan bool {
	ch := make(chan framebus.Frame, 5)
	stop := make(chan bool)

	if err := bus.Subscribe(id, ch); err != nil {
		log.Fatalf("Failed to subscribe %s: %v", id, err)
	}

	go func() {
		processed := 0
		for {
			select {
			case frame := <-ch:
				// Simulate processing
				time.Sleep(processingTime)
				processed++

				// Log occasionally
				if processed%50 == 0 {
					fmt.Printf("[%s] Processed %d frames (latest: seq=%d)\n",
						id, processed, frame.Seq)
				}

			case <-stop:
				fmt.Printf("[%s] Stopping. Total processed: %d\n", id, processed)
				return
			}
		}
	}()

	return stop
}

// reportStats periodically prints bus statistics.
func reportStats(bus framebus.Bus, interval time.Duration, done chan bool) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	prevStats := bus.Stats()

	for {
		select {
		case <-ticker.C:
			stats := bus.Stats()

			// Calculate deltas
			deltaPublished := stats.TotalPublished - prevStats.TotalPublished
			deltaSent := stats.TotalSent - prevStats.TotalSent
			deltaDropped := stats.TotalDropped - prevStats.TotalDropped

			fmt.Printf("\n--- Stats (last %v) ---\n", interval)
			fmt.Printf("Published: %d frames (%.1f FPS)\n",
				deltaPublished, float64(deltaPublished)/interval.Seconds())
			fmt.Printf("Sent: %d, Dropped: %d\n", deltaSent, deltaDropped)

			if deltaSent+deltaDropped > 0 {
				dropRate := float64(deltaDropped) / float64(deltaSent+deltaDropped)
				fmt.Printf("Drop rate: %.1f%%\n", dropRate*100)
			}

			// Per-subscriber details
			for id, sub := range stats.Subscribers {
				prevSub := prevStats.Subscribers[id]
				deltaSent := sub.Sent - prevSub.Sent
				deltaDropped := sub.Dropped - prevSub.Dropped

				if deltaSent+deltaDropped > 0 {
					dropRate := float64(deltaDropped) / float64(deltaSent+deltaDropped)
					fmt.Printf("  [%s] Sent: %d, Dropped: %d (%.1f%% drop)\n",
						id, deltaSent, deltaDropped, dropRate*100)
				}
			}

			prevStats = stats

		case <-done:
			return
		}
	}
}

// printStats prints detailed statistics.
func printStats(stats framebus.BusStats) {
	fmt.Printf("Total Published: %d\n", stats.TotalPublished)
	fmt.Printf("Total Sent: %d\n", stats.TotalSent)
	fmt.Printf("Total Dropped: %d\n", stats.TotalDropped)

	if stats.TotalSent+stats.TotalDropped > 0 {
		globalDropRate := float64(stats.TotalDropped) / float64(stats.TotalSent+stats.TotalDropped)
		fmt.Printf("Global Drop Rate: %.2f%%\n", globalDropRate*100)
	}

	fmt.Println("\nPer-Subscriber Stats:")
	for id, sub := range stats.Subscribers {
		total := sub.Sent + sub.Dropped
		if total > 0 {
			dropRate := float64(sub.Dropped) / float64(total)
			fmt.Printf("  %s:\n", id)
			fmt.Printf("    Sent: %d\n", sub.Sent)
			fmt.Printf("    Dropped: %d\n", sub.Dropped)
			fmt.Printf("    Drop Rate: %.2f%%\n", dropRate*100)
		}
	}

	// Verify conservation law
	expected := stats.TotalPublished * uint64(len(stats.Subscribers))
	actual := stats.TotalSent + stats.TotalDropped
	fmt.Printf("\nConservation Law Check: %d == %d? %v\n",
		actual, expected, actual == expected)
}
