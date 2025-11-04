package rtsp

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// ReconnectConfig contains configuration for exponential backoff reconnection
type ReconnectConfig struct {
	MaxRetries    int           // Maximum number of reconnection attempts (default: 5)
	RetryDelay    time.Duration // Initial retry delay (default: 1 second)
	MaxRetryDelay time.Duration // Maximum retry delay cap (default: 30 seconds)
}

// DefaultReconnectConfig returns default reconnection configuration
func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		MaxRetries:    5,
		RetryDelay:    1 * time.Second,
		MaxRetryDelay: 30 * time.Second,
	}
}

// ReconnectState tracks the current state of reconnection attempts
type ReconnectState struct {
	CurrentRetries int
	Reconnects     *uint32 // Atomic counter for total reconnection attempts
}

// ConnectFunc is a function that attempts to establish a connection
// Returns an error if connection fails
type ConnectFunc func(ctx context.Context) error

// RunWithReconnect executes a connection function with exponential backoff retry logic
//
// This function continuously attempts to connect using the provided connectFn.
// On failure, it waits with exponential backoff before retrying.
//
// Exponential backoff schedule:
//   - Attempt 1: 1 second
//   - Attempt 2: 2 seconds
//   - Attempt 3: 4 seconds
//   - Attempt 4: 8 seconds
//   - Attempt 5: 16 seconds
//   - After 5 failures: Stop (max retries exceeded)
//
// Returns an error if max retries are exceeded or context is cancelled.
func RunWithReconnect(
	ctx context.Context,
	connectFn ConnectFunc,
	cfg ReconnectConfig,
	state *ReconnectState,
) error {
	for {
		// Check context before attempting connection
		select {
		case <-ctx.Done():
			slog.Info("rtsp: context cancelled, stopping reconnection")
			return ctx.Err()
		default:
		}

		// Attempt connection
		err := connectFn(ctx)
		if err == nil {
			// Connection successful - reset retry counter
			state.CurrentRetries = 0
			slog.Info("rtsp: connection established successfully")
			return nil
		}

		// Connection failed
		slog.Error("rtsp: connection failed", "error", err)

		// Check if max retries exceeded
		state.CurrentRetries++
		atomic.AddUint32(state.Reconnects, 1)

		if state.CurrentRetries > cfg.MaxRetries {
			return fmt.Errorf("rtsp: max retries exceeded (%d attempts)", cfg.MaxRetries)
		}

		// Calculate exponential backoff delay
		delay := calculateBackoff(state.CurrentRetries, cfg)

		slog.Warn("rtsp: retrying connection",
			"attempt", state.CurrentRetries,
			"max_retries", cfg.MaxRetries,
			"delay", delay,
		)

		// Wait with backoff (or until context cancelled)
		select {
		case <-time.After(delay):
			continue // Retry
		case <-ctx.Done():
			slog.Info("rtsp: context cancelled during backoff")
			return ctx.Err()
		}
	}
}

// calculateBackoff calculates the exponential backoff delay for a given attempt
//
// Formula: delay = retryDelay * 2^(attempt-1)
// Cap: min(delay, maxRetryDelay)
//
// Example with default config (retryDelay=1s, maxRetryDelay=30s):
//   - Attempt 1: 1s * 2^0 = 1s
//   - Attempt 2: 1s * 2^1 = 2s
//   - Attempt 3: 1s * 2^2 = 4s
//   - Attempt 4: 1s * 2^3 = 8s
//   - Attempt 5: 1s * 2^4 = 16s
func calculateBackoff(attempt int, cfg ReconnectConfig) time.Duration {
	// Calculate exponential delay: retryDelay * 2^(attempt-1)
	delay := cfg.RetryDelay * time.Duration(1<<uint(attempt-1))

	// Cap delay at maxRetryDelay
	if delay > cfg.MaxRetryDelay {
		delay = cfg.MaxRetryDelay
	}

	return delay
}

// ResetReconnectState resets the reconnection state after a successful connection
//
// This should be called when a connection is successfully established to reset
// the retry counter for future reconnection attempts.
func ResetReconnectState(state *ReconnectState) {
	state.CurrentRetries = 0
	slog.Debug("rtsp: reconnect state reset")
}
