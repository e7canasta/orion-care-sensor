package streamcapture

import "time"

// Frame represents a single video frame with metadata
type Frame struct {
	// Seq is the monotonic sequence number
	Seq uint64
	// Timestamp is when the frame was captured/decoded
	Timestamp time.Time
	// Width in pixels
	Width int
	// Height in pixels
	Height int
	// Data contains the frame data (RGB format from GStreamer)
	Data []byte
	// SourceStream identifies the stream (e.g., "LQ", "HQ")
	SourceStream string
	// TraceID is a unique identifier for distributed tracing
	TraceID string
}

// StreamStats contains current stream statistics
type StreamStats struct {
	// FrameCount is the total number of frames captured
	FrameCount uint64
	// FramesDropped is the total number of frames dropped (channel full)
	FramesDropped uint64
	// DropRate is the percentage of frames dropped (0-100)
	DropRate float64
	// FPSTarget is the configured target FPS
	FPSTarget float64
	// FPSReal is the measured real FPS
	FPSReal float64
	// LatencyMS is the time since last frame in milliseconds
	LatencyMS int64
	// SourceStream identifies the stream (e.g., "LQ", "HQ")
	SourceStream string
	// Resolution is the frame resolution (e.g., "1280x720")
	Resolution string
	// Reconnects is the number of reconnection attempts
	Reconnects uint32
	// BytesRead is the total bytes read from the stream
	BytesRead uint64
	// IsConnected indicates if the stream is currently connected
	IsConnected bool
	// ErrorsNetwork is the count of network-related errors (connection, timeout, not found)
	ErrorsNetwork uint64
	// ErrorsCodec is the count of codec/stream errors (decode failures, format issues)
	ErrorsCodec uint64
	// ErrorsAuth is the count of authentication/authorization errors
	ErrorsAuth uint64
	// ErrorsUnknown is the count of unclassified errors
	ErrorsUnknown uint64
	// DecodeLatencyMeanMS is the mean decode latency in milliseconds (PTS → callback arrival)
	// Only populated when using hardware acceleration (VAAPI)
	DecodeLatencyMeanMS float64
	// DecodeLatencyP95MS is the 95th percentile decode latency in milliseconds
	// Represents the latency experienced by 95% of frames (SLO metric)
	DecodeLatencyP95MS float64
	// DecodeLatencyMaxMS is the maximum decode latency observed in milliseconds
	// Useful for detecting tail latency (thermal throttling, I/O stalls)
	DecodeLatencyMaxMS float64
	// UsingVAAPI indicates if VAAPI hardware acceleration is active
	UsingVAAPI bool
}

// ErrorCategory represents the classification of GStreamer errors for telemetry
type ErrorCategory int

const (
	// ErrCategoryNetwork indicates network-related failures (connection, timeout, DNS)
	ErrCategoryNetwork ErrorCategory = iota
	// ErrCategoryCodec indicates codec/stream failures (decode errors, format issues)
	ErrCategoryCodec
	// ErrCategoryAuth indicates authentication/authorization failures
	ErrCategoryAuth
	// ErrCategoryUnknown indicates unclassified errors
	ErrCategoryUnknown
)

// String returns a human-readable string representation of the error category
func (e ErrorCategory) String() string {
	switch e {
	case ErrCategoryNetwork:
		return "network"
	case ErrCategoryCodec:
		return "codec"
	case ErrCategoryAuth:
		return "auth"
	case ErrCategoryUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// Resolution represents supported video resolutions
type Resolution int

const (
	// Res512p represents 910x512 resolution
	Res512p Resolution = iota
	// Res720p represents 1280x720 resolution (HD)
	Res720p
	// Res1080p represents 1920x1080 resolution (Full HD)
	Res1080p
)

// Dimensions returns the width and height for the resolution
func (r Resolution) Dimensions() (width, height int) {
	switch r {
	case Res512p:
		return 910, 512
	case Res720p:
		return 1280, 720
	case Res1080p:
		return 1920, 1080
	default:
		// Safe default: 720p
		return 1280, 720
	}
}

// String returns a human-readable string representation of the resolution
func (r Resolution) String() string {
	switch r {
	case Res512p:
		return "512p"
	case Res720p:
		return "720p"
	case Res1080p:
		return "1080p"
	default:
		return "720p"
	}
}

// HardwareAccel represents hardware acceleration mode for video decoding
type HardwareAccel int

const (
	// AccelAuto attempts VAAPI hardware acceleration, falls back to software decode if unavailable.
	// This is the recommended default for production deployments on Intel hardware.
	AccelAuto HardwareAccel = iota
	// AccelVAAPI forces VAAPI hardware acceleration (Intel Quick Sync).
	// Fails fast at construction time if VAAPI is not available.
	// Use this for deployments where hardware acceleration is mandatory.
	AccelVAAPI
	// AccelSoftware forces software decoding (CPU-based).
	// Use this for maximum compatibility or debugging hardware issues.
	AccelSoftware
)

// String returns a human-readable string representation of the acceleration mode
func (a HardwareAccel) String() string {
	switch a {
	case AccelAuto:
		return "auto"
	case AccelVAAPI:
		return "vaapi"
	case AccelSoftware:
		return "software"
	default:
		return "auto"
	}
}

// RTSPConfig contains configuration for RTSP stream capture
type RTSPConfig struct {
	// URL is the RTSP stream URL (required)
	URL string
	// Resolution is the target video resolution
	Resolution Resolution
	// TargetFPS is the target frames per second (0.1 - 30.0)
	TargetFPS float64
	// SourceStream identifies the stream (e.g., "LQ", "HQ")
	SourceStream string
	// MaxReconnectAttempts is the maximum number of reconnection attempts (default: 5)
	// Set to 0 to use default value
	MaxReconnectAttempts int
	// ReconnectInitialDelay is the initial delay before first reconnection attempt (default: 1s)
	// Set to 0 to use default value
	ReconnectInitialDelay time.Duration
	// ReconnectMaxDelay is the maximum delay between reconnection attempts (default: 30s)
	// Set to 0 to use default value
	ReconnectMaxDelay time.Duration
	// Acceleration specifies hardware acceleration mode (default: AccelAuto)
	// AccelAuto: Try VAAPI, fallback to software (recommended)
	// AccelVAAPI: Force VAAPI, fail-fast if unavailable
	// AccelSoftware: Force software decode
	Acceleration HardwareAccel
}

// WarmupStats contains statistics collected during stream warm-up phase
type WarmupStats struct {
	// FramesReceived is the number of frames received during warm-up
	FramesReceived int
	// Duration is the actual warm-up duration
	Duration time.Duration
	// FPSMean is the mean FPS across all frames
	FPSMean float64
	// FPSStdDev is the standard deviation of FPS
	FPSStdDev float64
	// FPSMin is the minimum instantaneous FPS
	FPSMin float64
	// FPSMax is the maximum instantaneous FPS
	FPSMax float64
	// IsStable is true if FPS is stable (stddev < 15% of mean AND jitter < 20%)
	IsStable bool
	// JitterMean is the average inter-frame interval variance (seconds)
	JitterMean float64
	// JitterStdDev is the standard deviation of jitter (seconds)
	JitterStdDev float64
	// JitterMax is the maximum jitter observed (seconds)
	JitterMax float64
}
