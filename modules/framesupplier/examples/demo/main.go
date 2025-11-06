package main

import (
"context"
"log"
"os"
"os/signal"
"syscall"
"time"

"github.com/e7canasta/orion-care-sensor/modules/framesupplier"
)

func main() {
log.SetFlags(log.LstdFlags | log.Lmicroseconds)

// Setup
supplier := framesupplier.New()
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Start supplier
go func() {
if err := supplier.Start(ctx); err != nil {
log.Printf("Supplier stopped: %v", err)
}
}()

// Start 2 workers (simulated inference)
go runWorker(supplier, "person-detector", 20*time.Millisecond)
go runWorker(supplier, "pose-detector", 100*time.Millisecond)

// Start frame producer (simulated GStreamer)
go produceFrames(supplier, 30) // 30fps

// Wait for CTRL+C
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
<-sigChan

log.Println("Shutting down...")
cancel()
time.Sleep(100 * time.Millisecond)
}

func produceFrames(supplier framesupplier.Supplier, fps int) {
ticker := time.NewTicker(time.Second / time.Duration(fps))
defer ticker.Stop()

for range ticker.C {
frame := &framesupplier.Frame{
Data:      make([]byte, 100*1024), // 100KB
Width:     1920,
Height:    1080,
Timestamp: time.Now(),
}
supplier.Publish(frame)
}
}

func runWorker(supplier framesupplier.Supplier, workerID string, inferenceTime time.Duration) {
readFunc := supplier.Subscribe(workerID)
defer supplier.Unsubscribe(workerID)

for {
frame := readFunc()
if frame == nil {
log.Printf("[%s] Stopped", workerID)
break
}

// Simulate inference
time.Sleep(inferenceTime)
log.Printf("[%s] Processed frame seq=%d", workerID, frame.Seq)
}
}
