package rtsp

import (
	"strings"

	"github.com/tinyzimmer/go-gst/gst"
)

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

// ClassifyGStreamerError analyzes a GStreamer error and categorizes it for telemetry
//
// This enables better debugging in production by distinguishing between:
// - Network issues (reconnect may help)
// - Codec issues (stream format problem, reconnect unlikely to help)
// - Auth issues (credentials needed)
// - Unknown issues (need investigation)
//
// Classification is based on GStreamer error domain and code, plus error message heuristics.
func ClassifyGStreamerError(gerr *gst.GError) ErrorCategory {
	if gerr == nil {
		return ErrCategoryUnknown
	}

	errMsg := strings.ToLower(gerr.Error())
	debugStr := strings.ToLower(gerr.DebugString())

	// Classify by GStreamer error domain
	switch gerr.Domain() {
	case gst.ResourceError:
		// Resource errors typically indicate network or file issues
		switch gerr.Code() {
		case int(gst.ResourceErrorNotFound):
			// Camera not found, DNS failure, or URL invalid
			return ErrCategoryNetwork
		case int(gst.ResourceErrorOpenRead), int(gst.ResourceErrorRead):
			// Connection issues, timeouts
			return ErrCategoryNetwork
		case int(gst.ResourceErrorOpenWrite), int(gst.ResourceErrorWrite):
			// Write failures (unlikely in RTSP source)
			return ErrCategoryNetwork
		case int(gst.ResourceErrorSettings):
			// Configuration issues (could be auth)
			if containsAuthKeywords(errMsg, debugStr) {
				return ErrCategoryAuth
			}
			return ErrCategoryNetwork
		default:
			// Other resource errors default to network
			return ErrCategoryNetwork
		}

	case gst.StreamError:
		// Stream errors indicate codec/format issues
		switch gerr.Code() {
		case int(gst.StreamErrorDecode), int(gst.StreamErrorCodecNotFound):
			// Decode failures, missing codec
			return ErrCategoryCodec
		case int(gst.StreamErrorFormat), int(gst.StreamErrorWrongType):
			// Format negotiation failures
			return ErrCategoryCodec
		case int(gst.StreamErrorDemux), int(gst.StreamErrorMux):
			// Demuxing issues (corrupt stream)
			return ErrCategoryCodec
		default:
			return ErrCategoryCodec
		}

	case gst.CoreError:
		// Core errors can be various issues
		if containsAuthKeywords(errMsg, debugStr) {
			return ErrCategoryAuth
		}
		// Core errors often indicate missing plugins or negotiation failures (codec-related)
		return ErrCategoryCodec

	case gst.LibraryError:
		// Library errors typically indicate plugin/codec issues
		return ErrCategoryCodec
	}

	// Fallback: Heuristic-based classification using error message keywords
	if containsAuthKeywords(errMsg, debugStr) {
		return ErrCategoryAuth
	}

	if containsNetworkKeywords(errMsg, debugStr) {
		return ErrCategoryNetwork
	}

	if containsCodecKeywords(errMsg, debugStr) {
		return ErrCategoryCodec
	}

	// Default to unknown if no classification matches
	return ErrCategoryUnknown
}

// containsAuthKeywords checks if error message contains authentication-related keywords
func containsAuthKeywords(errMsg, debugStr string) bool {
	keywords := []string{
		"unauthorized",
		"401",
		"403",
		"forbidden",
		"authentication",
		"credentials",
		"password",
		"username",
	}

	combined := errMsg + " " + debugStr
	for _, kw := range keywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

// containsNetworkKeywords checks if error message contains network-related keywords
func containsNetworkKeywords(errMsg, debugStr string) bool {
	keywords := []string{
		"connection",
		"timeout",
		"unreachable",
		"network",
		"dns",
		"resolve",
		"socket",
		"tcp",
		"udp",
		"rtsp",
		"not found",
		"could not connect",
		"failed to connect",
	}

	combined := errMsg + " " + debugStr
	for _, kw := range keywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

// containsCodecKeywords checks if error message contains codec-related keywords
func containsCodecKeywords(errMsg, debugStr string) bool {
	keywords := []string{
		"codec",
		"decode",
		"encode",
		"format",
		"negotiation",
		"caps",
		"h264",
		"h265",
		"mjpeg",
		"jpeg",
		"not negotiated",
		"no decoder",
		"missing plugin",
	}

	combined := errMsg + " " + debugStr
	for _, kw := range keywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}
