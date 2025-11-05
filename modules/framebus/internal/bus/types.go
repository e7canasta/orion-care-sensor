package bus

import "errors"

// Internal errors - mapped to public errors in framebus package
var (
ErrBusClosed          = errors.New("framebus: bus is closed")
ErrSubscriberExists   = errors.New("framebus: subscriber already exists")
ErrSubscriberNotFound = errors.New("framebus: subscriber not found")
ErrNilChannel         = errors.New("framebus: nil channel provided")
ErrReceiverClosed     = errors.New("framebus: receiver is closed")
)

// DropPolicy defines how the bus handles frames when subscriber cannot keep up
type DropPolicy int

const (
DropNew DropPolicy = iota
DropOld
)

// Frame represents a video frame with metadata
type Frame struct {
Data      []byte
Width     int
Height    int
Sequence  uint64
Timestamp int64
Meta      map[string]interface{}
}

// FrameReceiver provides blocking/non-blocking frame access
type FrameReceiver interface {
Receive() Frame
TryReceive() (Frame, bool)
Close()
}

// SubscriberStats tracks frame distribution metrics
type SubscriberStats struct {
Sent    uint64
Dropped uint64
}

// Bus distributes frames to multiple subscribers
type Bus interface {
Subscribe(id string, ch chan<- Frame) error
SubscribeDropOld(id string) (FrameReceiver, error)
Publish(frame Frame)
Unsubscribe(id string) error
Stats(id string) (*SubscriberStats, error)
Close()
}
