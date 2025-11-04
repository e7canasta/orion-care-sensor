# Makefile for Stream Capture Module
# Orion 2.0 - Sprint 1.1

.PHONY: help build test test-capture clean examples install

# Default target
help:
	@echo ""
	@echo "╔═══════════════════════════════════════════════════════════╗"
	@echo "║     Stream Capture Module - Available Commands           ║"
	@echo "╚═══════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "Build Commands:"
	@echo "  make build          Build the module library"
	@echo "  make test-capture   Build the test-capture binary"
	@echo "  make examples       Build all example binaries"
	@echo "  make all            Build everything"
	@echo ""
	@echo "Testing Commands:"
	@echo "  make test           Run unit tests"
	@echo "  make test-verbose   Run tests with verbose output"
	@echo ""
	@echo "Run Commands (requires RTSP_URL):"
	@echo "  make run-test       Run test-capture with default params"
	@echo "  make run-simple     Run simple example"
	@echo "  make run-hotreload  Run hot-reload example"
	@echo ""
	@echo "Utility Commands:"
	@echo "  make clean          Remove build artifacts"
	@echo "  make install        Install test-capture to GOPATH/bin"
	@echo ""
	@echo "Examples:"
	@echo "  RTSP_URL=rtsp://camera/stream make run-test"
	@echo "  RTSP_URL=rtsp://camera/stream OUTPUT_DIR=./frames make run-test"
	@echo "  RTSP_URL=rtsp://camera/stream FPS=1.0 RES=720p make run-test"
	@echo ""

# Build targets
build:
	@echo "Building stream-capture module..."
	@go build -v .
	@go build -v ./internal/...
	@echo "✅ Build successful"

test-capture:
	@echo "Building test-capture binary..."
	@go build -v -o bin/test-capture ./cmd/test-capture
	@echo "✅ test-capture built: bin/test-capture"

examples: examples/simple examples/hot-reload
	@echo "✅ All examples built"

examples/simple:
	@echo "Building simple example..."
	@go build -v -o bin/simple ./examples/simple
	@echo "✅ simple built: bin/simple"

examples/hot-reload:
	@echo "Building hot-reload example..."
	@go build -v -o bin/hot-reload ./examples/hot-reload
	@echo "✅ hot-reload built: bin/hot-reload"

all: build test-capture examples
	@echo "✅ All targets built successfully"

# Test targets
test:
	@echo "Running tests..."
	@go test -v ./...

test-verbose:
	@go test -v -race ./...

# Run targets (require RTSP_URL environment variable)
run-test: test-capture
	@if [ -z "$(RTSP_URL)" ]; then \
		echo "Error: RTSP_URL environment variable not set"; \
		echo "Usage: RTSP_URL=rtsp://camera/stream make run-test"; \
		exit 1; \
	fi
	@echo "Running test-capture..."
	@./bin/test-capture \
		--url "$(RTSP_URL)" \
		--resolution "$(or $(RES),720p)" \
		--fps "$(or $(FPS),2.0)" \
		--output "$(OUTPUT_DIR)" \
		--stats-interval "$(or $(STATS_INTERVAL),10)" \
		$(if $(DEBUG),--debug) \
		$(if $(MAX_FRAMES),--max-frames $(MAX_FRAMES))

run-simple: examples/simple
	@if [ -z "$(RTSP_URL)" ]; then \
		echo "Error: RTSP_URL environment variable not set"; \
		echo "Usage: RTSP_URL=rtsp://camera/stream make run-simple"; \
		exit 1; \
	fi
	@./bin/simple --url "$(RTSP_URL)" $(if $(DEBUG),--debug)

run-hotreload: examples/hot-reload
	@if [ -z "$(RTSP_URL)" ]; then \
		echo "Error: RTSP_URL environment variable not set"; \
		echo "Usage: RTSP_URL=rtsp://camera/stream make run-hotreload"; \
		exit 1; \
	fi
	@./bin/hot-reload --url "$(RTSP_URL)" $(if $(DEBUG),--debug)

# Utility targets
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f simple hot_reload  # Legacy binaries in module root
	@echo "✅ Clean complete"

install: test-capture
	@echo "Installing test-capture to $(GOPATH)/bin..."
	@cp bin/test-capture $(GOPATH)/bin/
	@echo "✅ Installed: $(GOPATH)/bin/test-capture"

# Development helpers
.PHONY: fmt vet lint
fmt:
	@echo "Formatting code..."
	@go fmt ./...

vet:
	@echo "Running go vet..."
	@go vet ./...

lint: fmt vet
	@echo "✅ Linting complete"

# Quick development workflow
.PHONY: dev
dev: lint build test
	@echo "✅ Development checks passed"
