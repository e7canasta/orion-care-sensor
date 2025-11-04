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
// Classification is based on error message heuristics and error codes.
// Note: go-gst's GError does not expose Domain(), so we rely on string matching.
func ClassifyGStreamerError(gerr *gst.GError) ErrorCategory {
	if gerr == nil {
		return ErrCategoryUnknown
	}

	errMsg := strings.ToLower(gerr.Error())
	debugStr := strings.ToLower(gerr.DebugString())

	// Priority 1: Check for authentication errors (most specific)
	if containsAuthKeywords(errMsg, debugStr) {
		return ErrCategoryAuth
	}

	// Priority 2: Check for codec/format errors
	if containsCodecKeywords(errMsg, debugStr) {
		return ErrCategoryCodec
	}

	// Priority 3: Check for network errors (most common)
	if containsNetworkKeywords(errMsg, debugStr) {
		return ErrCategoryNetwork
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
