package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/care/orion/internal/core"
)

const (
	defaultConfigPath = "config/orion.yaml"
	healthCheckPort   = "8080"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Setup structured logger
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("starting orion service",
		"config", *configPath,
		"debug", *debug,
	)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize Orion service
	orion, err := core.NewOrion(*configPath)
	if err != nil {
		slog.Error("failed to create orion service", "error", err)
		os.Exit(1)
	}

	// Start health check HTTP server (non-blocking)
	if err := orion.StartHealthServer(healthCheckPort); err != nil {
		slog.Error("failed to start health check server", "error", err)
		os.Exit(1)
	}

	// Run service in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- orion.Run(ctx) // Always send, even if nil
	}()

	// Wait for shutdown signal or error
	var shutdownErr error
	select {
	case sig := <-sigChan:
		slog.Info("received shutdown signal", "signal", sig)
		cancel() // Cancel the context
	case shutdownErr = <-errChan:
		if shutdownErr != nil {
			slog.Error("service error", "error", shutdownErr)
		} else {
			slog.Info("service stopped (via MQTT shutdown command)")
		}
	}

	// Graceful shutdown
	shutdownTimeout := orion.ShutdownTimeout()
	slog.Info("shutting down gracefully", "timeout", shutdownTimeout)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := orion.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("orion service stopped successfully")
}
