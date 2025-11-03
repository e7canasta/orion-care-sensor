package stream

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/care/orion/internal/types"
	"github.com/google/uuid"
)

// MockStream generates synthetic frames for testing
type MockStream struct {
	width  int
	height int
	fps    int
	source string

	framesCh chan types.Frame
	stopCh   chan struct{}
	wg       sync.WaitGroup

	mu            sync.RWMutex
	seq           uint64
	framesEmitted uint64
	isRunning     bool
	startTime     time.Time
}

// NewMockStream creates a new mock stream provider
func NewMockStream(width, height, fps int, source string) *MockStream {
	return &MockStream{
		width:    width,
		height:   height,
		fps:      fps,
		source:   source,
		framesCh: make(chan types.Frame, 10),
		stopCh:   make(chan struct{}),
	}
}

// Start begins generating frames
func (m *MockStream) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("stream already running")
	}
	m.isRunning = true
	m.startTime = time.Now()
	m.mu.Unlock()

	slog.Info("mock stream starting",
		"width", m.width,
		"height", m.height,
		"fps", m.fps,
		"source", m.source,
	)

	m.wg.Add(1)
	go m.generateFrames(ctx)

	return nil
}

// Frames returns the frames channel
func (m *MockStream) Frames() <-chan types.Frame {
	return m.framesCh
}

// Stop stops the stream
func (m *MockStream) Stop() error {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	slog.Info("mock stream stopping")

	close(m.stopCh)
	m.wg.Wait()
	close(m.framesCh)

	m.mu.Lock()
	m.isRunning = false
	m.mu.Unlock()

	slog.Info("mock stream stopped",
		"frames_emitted", m.framesEmitted,
		"duration", time.Since(m.startTime),
	)

	return nil
}

// Stats returns stream statistics
func (m *MockStream) Stats() types.StreamStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var fpsReal float64
	if m.isRunning && m.framesEmitted > 0 {
		elapsed := time.Since(m.startTime).Seconds()
		if elapsed > 0 {
			fpsReal = float64(m.framesEmitted) / elapsed
		}
	}

	return types.StreamStats{
		FrameCount:   m.framesEmitted,
		FPSTarget:    m.fps,
		FPSReal:      fpsReal,
		LatencyMS:    0, // Mock has no latency
		SourceStream: m.source,
		Resolution:   fmt.Sprintf("%dx%d", m.width, m.height),
		Reconnects:   0, // Mock never reconnects
		BytesRead:    0, // Mock doesn't track bytes
		IsConnected:  m.isRunning,
		Errors:       0,
	}
}

// generateFrames generates frames at the target FPS
func (m *MockStream) generateFrames(ctx context.Context) {
	defer m.wg.Done()

	frameDuration := time.Second / time.Duration(m.fps)
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	slog.Debug("frame generator started", "frame_duration", frameDuration)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			frame := m.createFrame()
			select {
			case m.framesCh <- frame:
				m.mu.Lock()
				m.framesEmitted++
				m.mu.Unlock()
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			}
		}
	}
}

// createFrame creates a synthetic BGR24 frame
func (m *MockStream) createFrame() types.Frame {
	m.mu.Lock()
	seq := m.seq
	m.seq++
	m.mu.Unlock()

	// Create a black frame (BGR24 format)
	// For now, just allocate a black buffer
	// In a real implementation, we could draw timestamp, etc.
	frameSize := m.width * m.height * 3 // BGR24 = 3 bytes per pixel
	data := make([]byte, frameSize)

	// Optional: Fill with a pattern to make it more interesting
	// For now, leave it black (all zeros)

	return types.Frame{
		Seq:          seq,
		Timestamp:    time.Now(),
		Width:        m.width,
		Height:       m.height,
		Data:         data,
		SourceStream: m.source,
		TraceID:      uuid.New().String(),
	}
}
