package main

import (
	"fmt"
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/framebus"
)

func main() {
	bus := framebus.New()
	defer bus.Close()

	// Example 1: DropNew subscriber (backpressure-based dropping)
	dropNewCh := make(chan framebus.Frame, 2)
	bus.Subscribe("worker-dropnew", dropNewCh)

	go func() {
		for frame := range dropNewCh {
			fmt.Printf("[DropNew] Processing frame %d\n", frame.Sequence)
			time.Sleep(100 * time.Millisecond) // Simulate slow processing
		}
	}()

	// Example 2: DropOld subscriber (always latest)
	receiver, _ := bus.SubscribeDropOld("worker-dropold")
	defer receiver.Close()

	go func() {
		for {
			frame := receiver.Receive()
			if frame.Sequence == 0 {
				break
			}
			fmt.Printf("[DropOld] Processing frame %d\n", frame.Sequence)
			time.Sleep(150 * time.Millisecond) // Simulate slow processing
		}
	}()

	// Publish frames at 10 FPS
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for i := uint64(1); i <= 20; i++ {
		frame := framebus.Frame{
			Sequence:  i,
			Data:      []byte(fmt.Sprintf("frame-%d", i)),
			Timestamp: time.Now().UnixNano(),
		}
		bus.Publish(frame)
		fmt.Printf("Published frame %d\n", i)
		<-ticker.C
	}

	time.Sleep(500 * time.Millisecond)

	// Print stats
	if stats, err := bus.Stats("worker-dropnew"); err == nil {
		fmt.Printf("\n[DropNew] Stats: Sent=%d, Dropped=%d\n", stats.Sent, stats.Dropped)
	}

	if stats, err := bus.Stats("worker-dropold"); err == nil {
		fmt.Printf("[DropOld] Stats: Sent=%d, Dropped=%d (always 0 for DropOld)\n", stats.Sent, stats.Dropped)
	}
}
