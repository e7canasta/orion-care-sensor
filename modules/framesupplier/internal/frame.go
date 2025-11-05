package internal

import "time"

// Frame represents a video frame with immutability contract for zero-copy sharing.
//
// IMMUTABILITY CONTRACT (ADR-002):
//   - Publisher: MUST NOT modify frame.Data after calling Publish(frame)
//   - Workers: MUST NOT modify frame.Data (read-only access)
//   - Enforcement: Documentation-based (runtime checks would add overhead)
//
// Zero-copy chain:
//
//	GStreamer (C) → C.GoBytes (1 copy) → *Frame.Data (Go heap)
//	                                           ↓ (0 copies)
//	                                      Inbox mailbox
//	                                           ↓ (0 copies)
//	                                      Worker slots (N)
//	                                           ↓ (0 copies)
//	                                      Worker goroutines (N)
//
// See: ADR-002 (Zero-Copy Frame Sharing), ARCHITECTURE.md (Memory Model)
type Frame struct {
	// Data contains the raw frame bytes (typically JPEG-encoded).
	// MUST NOT be modified after Publish() call (shared by reference).
	Data []byte

	// Width of the frame in pixels
	Width int

	// Height of the frame in pixels
	Height int

	// Timestamp when frame was captured (source time, not processing time)
	Timestamp time.Time

	// Seq is a global sequence number assigned by Supplier during distribution.
	// Monotonically increasing. Used for drop detection and ordering verification.
	// Set by distributeToWorkers(), not by publisher.
	Seq uint64
}
