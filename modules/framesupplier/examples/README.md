# FrameSupplier Examples

Examples demonstrating FrameSupplier usage patterns.

## Available Examples

### 1. **demo** - Simple Producer/Consumer
Location: `examples/demo/`

Minimal example showing basic FrameSupplier usage:
- In-memory frame generation @ 30fps
- 2 workers with different inference times
- Demonstrates frame drops on slow workers

**Run:**
```bash
cd examples/demo
go run main.go
```

**Press CTRL+C to stop**

---

### 2. **filesim** - Filesystem Stream Simulator ⭐
Location: `examples/filesim/`

Production-like simulator reading frames from disk:
- Read frames from directory (sorted by name)
- Multiple workers writing to separate outputs
- **Real-time stats monitoring** (FPS accuracy, drop rates, inbox health)
- **Variable worker speeds** (simulate YOLO320/640, VLM, etc)
- **MQTT emitter worker** (stub for integration testing)
- Configurable FPS, loop count, file patterns

**Quick start:**
```bash
cd examples/filesim

# Test with stats monitoring
go run main.go -fps 30 -stats

# Simulate multi-model workers (fast/medium/slow)
go run main.go -fps 30 -stats -worker-delays "20ms,50ms,100ms"

# MQTT integration test
go run main.go -mqtt-worker -mqtt-broker tcp://localhost:1883
```

**Documentation:**
- `examples/filesim/README.md` - Full feature documentation
- `examples/filesim/EXAMPLES.md` - Usage examples & benchmarks ⭐

---

## Use Cases

| Example  | Use Case                                    | Best For                          |
|----------|---------------------------------------------|-----------------------------------|
| demo     | Understanding basic API                     | Learning FrameSupplier interface  |
| filesim  | Testing with real frame sequences           | Integration testing, benchmarking |

---

## Creating Your Own Example

1. Import the module:
```go
import "github.com/e7canasta/orion-care-sensor/modules/framesupplier"
```

2. Create supplier:
```go
supplier := framesupplier.New()
ctx, cancel := context.WithCancel(context.Background())
go supplier.Start(ctx)
```

3. **Producer side** (stream-capture):
```go
frame := &framesupplier.Frame{
    Data:      jpegBytes,
    Width:     1920,
    Height:    1080,
    Timestamp: time.Now(),
}
supplier.Publish(frame) // Non-blocking (~1µs)
```

4. **Consumer side** (worker):
```go
readFunc := supplier.Subscribe("worker-id")
defer supplier.Unsubscribe("worker-id")

for {
    frame := readFunc() // Blocking
    if frame == nil { break }
    
    // Process frame...
    runInference(frame)
}
```

See `examples/stream_capture_client.go` and `examples/worker_client.go` for detailed patterns.
