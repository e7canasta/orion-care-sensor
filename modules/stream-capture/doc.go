// Package streamcapture provides RTSP video stream acquisition using GStreamer.
//
// This module is part of Orion 2.0 and implements Bounded Context "Stream Acquisition"
// (Sprint 1.1). It handles low-latency video capture from IP cameras with automatic
// reconnection, hardware acceleration (VAAPI), and hot-reload capabilities.
//
// # Quick Start
//
// The simplest way to capture frames from an RTSP stream:
//
//	cfg := streamcapture.RTSPConfig{
//	    URL:          "rtsp://192.168.1.100/stream",
//	    Resolution:   streamcapture.Res720p,
//	    TargetFPS:    2.0,
//	    SourceStream: "camera-1",
//	    Acceleration: streamcapture.AccelAuto,
//	}
//
//	stream, err := streamcapture.NewRTSPStream(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Stop()
//
//	ctx := context.Background()
//	frameChan, err := stream.Start(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Recommended: Warmup to measure FPS stability
//	stats, _ := stream.Warmup(ctx, 5*time.Second)
//	log.Printf("Stream stable: %v, FPS: %.2f", stats.IsStable, stats.FPSMean)
//
//	// Process frames
//	for frame := range frameChan {
//	    // frame.Data contains raw RGB bytes
//	    // frame.Width x frame.Height pixels
//	    processFrame(frame)
//	}
//
// # Features
//
//   - RTSP streaming via GStreamer (requires gstreamer1.0 runtime)
//   - Hardware acceleration (Intel VAAPI with automatic fallback)
//   - Automatic reconnection with exponential backoff (max 5 attempts)
//   - Hot-reload FPS without stream restart (~2s interruption vs 5-10s full restart)
//   - Non-blocking frame distribution (drop policy to maintain <2s latency)
//   - Comprehensive telemetry: FPS, latency, error categorization, decode metrics
//   - Thread-safe statistics access
//
// # Supported Resolutions
//
//   - 480p (640x480) - VGA
//   - 512p (910x512) - Custom
//   - 640p (960x640) - Custom
//   - 720p (1280x720) - HD (recommended default)
//   - 1080p (1920x1080) - Full HD
//
// # Hardware Acceleration
//
// Three acceleration modes are supported via RTSPConfig.Acceleration:
//
//   - AccelAuto (default): Attempts VAAPI hardware decode, falls back to software
//   - AccelVAAPI: Forces VAAPI, fails fast if unavailable
//   - AccelSoftware: Forces CPU decode (debugging/compatibility)
//
// VAAPI provides ~4x lower decode latency on Intel hardware:
//   - Software decode: ~50-80ms per frame
//   - VAAPI decode: ~10-20ms per frame
//
// Hardware acceleration metrics are exposed via StreamStats when UsingVAAPI is true.
//
// # Frame Format
//
// Frames are delivered as raw RGB bytes (no compression):
//
//   - Format: Interleaved RGB (RGBRGBRGB...)
//   - Size: Width × Height × 3 bytes
//   - Example (720p): 1280 × 720 × 3 = 2,764,800 bytes (~2.6 MB)
//
// Convert to image.Image:
//
//	img := &image.RGBA{
//	    Pix:    make([]uint8, len(frame.Data)+frame.Width*frame.Height),
//	    Stride: frame.Width * 4,
//	    Rect:   image.Rect(0, 0, frame.Width, frame.Height),
//	}
//	for i := 0; i < frame.Width*frame.Height; i++ {
//	    img.Pix[i*4+0] = frame.Data[i*3+0] // R
//	    img.Pix[i*4+1] = frame.Data[i*3+1] // G
//	    img.Pix[i*4+2] = frame.Data[i*3+2] // B
//	    img.Pix[i*4+3] = 255               // A (opaque)
//	}
//
// # Hot-Reload FPS
//
// Change FPS dynamically without full stream restart:
//
//	err := stream.SetTargetFPS(0.5) // 1 frame every 2 seconds
//	if err != nil {
//	    log.Printf("FPS change failed: %v", err)
//	}
//
// This triggers a GStreamer caps renegotiation (~2s interruption) instead of
// full pipeline teardown/rebuild (5-10s).
//
// # Error Handling and Reconnection
//
// The module automatically reconnects on transient failures:
//
//   - Network errors: Connection timeout, DNS failure, stream not found
//   - Codec errors: Decode failure, unsupported format
//   - Auth errors: 401/403 responses
//
// Reconnection strategy:
//   - Exponential backoff: 1s, 2s, 4s, 8s, 16s (max 30s)
//   - Maximum 5 attempts before giving up
//   - Errors categorized for telemetry (StreamStats.Errors*)
//
// Monitor reconnection status:
//
//	stats := stream.Stats()
//	if stats.Reconnects > 0 {
//	    log.Printf("Stream reconnected %d times", stats.Reconnects)
//	}
//
// # Statistics and Telemetry
//
// Real-time statistics are available via Stats():
//
//	stats := stream.Stats()
//	fmt.Printf("FPS: %.2f (target: %.2f)\n", stats.FPSReal, stats.FPSTarget)
//	fmt.Printf("Latency: %dms\n", stats.LatencyMS)
//	fmt.Printf("Frames captured: %d\n", stats.FrameCount)
//	fmt.Printf("Drop rate: %.2f%%\n", stats.DropRate)
//
//	if stats.UsingVAAPI {
//	    fmt.Printf("VAAPI decode latency (mean): %.2fms\n", stats.DecodeLatencyMeanMS)
//	    fmt.Printf("VAAPI decode latency (P95): %.2fms\n", stats.DecodeLatencyP95MS)
//	}
//
// # Dependencies
//
// GStreamer 1.x must be installed on the system:
//
//	# Ubuntu/Debian
//	sudo apt-get install \
//	    gstreamer1.0-tools \
//	    gstreamer1.0-plugins-base \
//	    gstreamer1.0-plugins-good \
//	    gstreamer1.0-plugins-bad \
//	    gstreamer1.0-plugins-ugly \
//	    gstreamer1.0-libav
//
//	# Fedora/RHEL
//	sudo dnf install \
//	    gstreamer1 \
//	    gstreamer1-plugins-base \
//	    gstreamer1-plugins-good \
//	    gstreamer1-plugins-bad-free \
//	    gstreamer1-plugins-ugly-free
//
// For VAAPI hardware acceleration (Intel GPUs):
//
//	# Ubuntu/Debian
//	sudo apt-get install intel-media-va-driver-non-free
//
//	# Fedora/RHEL
//	sudo dnf install intel-media-driver
//
// Verify GStreamer installation:
//
//	gst-inspect-1.0 --version
//	gst-inspect-1.0 rtspsrc
//	gst-inspect-1.0 vaapidecodebin  # For VAAPI
//
// # Thread Safety
//
// All public methods are thread-safe:
//
//   - Start() returns a channel safe for concurrent reads
//   - Stop() is idempotent and can be called from any goroutine
//   - Stats() uses atomic operations for lock-free reads
//   - SetTargetFPS() uses internal locking for safe updates
//
// # Design Philosophy
//
// This module follows Orion's core principle: "Complejidad por diseño, no por accidente"
//
//   - Non-blocking channels: Drop frames rather than queue (latency over completeness)
//   - Fail-fast validation: Configuration errors detected at construction time
//   - Adaptive timeouts: Watchdog adapts to configured FPS (3× inference period)
//   - Process isolation: GStreamer runs in Go process (no subprocess overhead)
//
// # Architecture Notes
//
// Design decisions are documented in ADRs (Architecture Decision Records):
//
//   - ADR-001: Non-blocking channels with drop policy
//   - ADR-002: Go-CGo boundary for GStreamer integration
//   - ADR-003: Adaptive reconnection (exponential backoff)
//   - ADR-004: VAAPI acceleration strategy
//   - ADR-005: Hot-reload mechanism (caps renegotiation)
//
// See docs/ARCHITECTURE.md for complete 4+1 architectural views.
// See docs/C4_MODEL.md for C1-C4 diagrams.
//
// # Examples
//
// Complete working examples are available in the examples/ directory:
//
//   - examples/simple/: Basic stream capture with statistics
//   - examples/hot-reload/: Interactive FPS changes via MQTT
//
// # Testing
//
// A command-line testing tool is provided:
//
//	# Build test tool
//	make test-capture
//
//	# Capture and display statistics
//	./bin/test-capture --url rtsp://camera/stream
//
//	# Save frames to disk for inspection
//	./bin/test-capture \
//	    --url rtsp://camera/stream \
//	    --output ./frames \
//	    --format png \
//	    --max-frames 100
//
// See cmd/test-capture/README.md for full documentation.
//
// # Performance Characteristics
//
//   - Pipeline startup: ~3 seconds (GStreamer PLAYING state)
//   - Hot-reload latency: ~2 seconds (caps renegotiation)
//   - Full restart latency: 5-10 seconds (teardown + rebuild)
//   - Frame latency: <500ms (network + decode)
//   - Memory per frame (720p): ~2.6 MB (raw RGB)
//   - CPU usage (software decode): ~15-25% per stream
//   - CPU usage (VAAPI decode): ~3-5% per stream
//
// # Limitations
//
//   - RTSP only (no HLS, WebRTC, or file sources)
//   - RGB output only (no YUV or compressed formats)
//   - Single stream per RTSPStream instance
//   - FPS range: 0.1 - 30.0 Hz
//   - No audio capture (video only)
//
// # Roadmap (Orion 2.0)
//
//   - Sprint 1.2: Multi-stream support (parallel stream instances)
//   - Sprint 2.0: ROI (Region of Interest) extraction
//   - Sprint 3.0: Stream health monitoring and alerting
//
// # Project Context
//
// This module is part of Orion, a real-time AI inference system for geriatric
// patient monitoring. It operates as a "smart sensor" following the philosophy:
// "Orión Ve, No Interpreta" (Orion Sees, Doesn't Interpret).
//
// Repository: https://github.com/e7canasta/orion-care-sensor
// License: Proprietary (Visiona Health)
package streamcapture
