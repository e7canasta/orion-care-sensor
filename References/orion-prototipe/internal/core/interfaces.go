package core

import (
	"context"

	"github.com/care/orion/internal/types"
)

// StreamProvider provides a stream of video frames
type StreamProvider interface {
	// Start begins streaming frames
	Start(ctx context.Context) error
	// Frames returns a channel of frames
	Frames() <-chan types.Frame
	// Stop stops the stream
	Stop() error
	// Stats returns stream statistics
	Stats() types.StreamStats
}

// Publisher publishes messages to a message broker
type Publisher interface {
	// Connect establishes connection to the broker
	Connect(ctx context.Context) error
	// Publish publishes a message to a topic
	Publish(topic string, payload []byte, qos byte) error
	// Disconnect closes the connection
	Disconnect() error
}
