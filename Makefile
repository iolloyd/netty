# Netty Network Monitor Makefile

# Default network interface (can be overridden with IFACE=eth0 make run-daemon)
IFACE ?= en0

# Default daemon host/port for TUI
DAEMON_HOST ?= localhost
DAEMON_PORT ?= 8080

# Build flags
GO_BUILD_FLAGS ?= -ldflags="-s -w"

.PHONY: all build daemon tui clean test run-daemon run-tui run dev help

# Default target
all: build

# Build both components
build: daemon tui
	@echo "✅ Build complete! Both daemon and TUI are ready."

# Build daemon
daemon:
	@echo "🔨 Building daemon..."
	@cd daemon && go build $(GO_BUILD_FLAGS) -o netty-daemon cmd/netty-daemon/main.go
	@echo "✅ Daemon built: daemon/netty-daemon"

# Build TUI
tui:
	@echo "🔨 Building TUI..."
	@cd tui && go build $(GO_BUILD_FLAGS) -o netty-tui cmd/netty-tui/main.go
	@echo "✅ TUI built: tui/netty-tui"

# Run daemon (requires sudo)
run-daemon: daemon
	@echo "🚀 Starting daemon on interface $(IFACE)..."
	@echo "⚠️  Requires sudo privileges for packet capture"
	sudo ./daemon/netty-daemon -i $(IFACE) -v

# Run TUI
run-tui: tui
	@echo "🚀 Starting TUI (connecting to $(DAEMON_HOST):$(DAEMON_PORT))..."
	./tui/netty-tui -host $(DAEMON_HOST) -port $(DAEMON_PORT)

# Run instructions
run: build
	@echo "🚀 Netty is built and ready to run!"
	@echo ""
	@echo "To start netty, open two terminal windows:"
	@echo ""
	@echo "Terminal 1 (daemon):"
	@echo "  make run-daemon"
	@echo ""
	@echo "Terminal 2 (TUI):"
	@echo "  make run-tui"
	@echo ""
	@echo "The daemon requires sudo privileges for packet capture."

# Development mode with hot reload (requires air)
dev:
	@if command -v air >/dev/null 2>&1; then \
		echo "🔄 Starting development mode with hot reload..."; \
		cd daemon && air; \
	else \
		echo "❌ air not found. Install with: go install github.com/cosmtrek/air@latest"; \
	fi

# Run tests
test:
	@echo "🧪 Running daemon tests..."
	@cd daemon && go test -v ./...
	@echo "🧪 Running TUI tests..."
	@cd tui && go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "📊 Running tests with coverage..."
	@cd daemon && go test -coverprofile=coverage.out ./...
	@cd daemon && go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: daemon/coverage.html"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -f daemon/netty-daemon
	@rm -f tui/netty-tui
	@rm -f daemon/coverage.out daemon/coverage.html
	@echo "✅ Clean complete"

# Install to system (requires sudo)
install: build
	@echo "📦 Installing netty..."
	@sudo cp daemon/netty-daemon /usr/local/bin/
	@sudo cp tui/netty-tui /usr/local/bin/
	@echo "✅ Installed to /usr/local/bin/"
	@echo "   Run daemon: sudo netty-daemon -i $(IFACE)"
	@echo "   Run TUI: netty-tui"

# Uninstall from system (requires sudo)
uninstall:
	@echo "🗑️  Uninstalling netty..."
	@sudo rm -f /usr/local/bin/netty-daemon
	@sudo rm -f /usr/local/bin/netty-tui
	@echo "✅ Uninstalled"

# Check dependencies
deps-check:
	@echo "🔍 Checking dependencies..."
	@cd daemon && go mod verify
	@cd tui && go mod verify
	@echo "✅ Dependencies OK"

# Update dependencies
deps-update:
	@echo "📦 Updating dependencies..."
	@cd daemon && go get -u ./... && go mod tidy
	@cd tui && go get -u ./... && go mod tidy
	@echo "✅ Dependencies updated"

# Lint code (requires golangci-lint)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "🔍 Linting code..."; \
		cd daemon && golangci-lint run; \
		cd ../tui && golangci-lint run; \
	else \
		echo "❌ golangci-lint not found. Install from https://golangci-lint.run/usage/install/"; \
	fi

# Format code
fmt:
	@echo "🎨 Formatting code..."
	@cd daemon && go fmt ./...
	@cd tui && go fmt ./...
	@echo "✅ Code formatted"

# Show help
help:
	@echo "Netty Network Monitor - Makefile targets:"
	@echo ""
	@echo "Building:"
	@echo "  make build        - Build both daemon and TUI"
	@echo "  make daemon       - Build daemon only"
	@echo "  make tui          - Build TUI only"
	@echo ""
	@echo "Running:"
	@echo "  make run-daemon   - Run daemon (requires sudo)"
	@echo "  make run-tui      - Run TUI"
	@echo "  make run          - Show instructions to run both"
	@echo ""
	@echo "Development:"
	@echo "  make dev          - Run with hot reload (requires air)"
	@echo "  make test         - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make lint         - Lint code (requires golangci-lint)"
	@echo "  make fmt          - Format code"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps-check   - Verify dependencies"
	@echo "  make deps-update  - Update dependencies"
	@echo ""
	@echo "Installation:"
	@echo "  make install      - Install to /usr/local/bin (requires sudo)"
	@echo "  make uninstall    - Remove from /usr/local/bin (requires sudo)"
	@echo ""
	@echo "Other:"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make help         - Show this help"
	@echo ""
	@echo "Environment variables:"
	@echo "  IFACE=$(IFACE)    - Network interface (default: en0)"
	@echo "  DAEMON_HOST=$(DAEMON_HOST) - Daemon host for TUI (default: localhost)"
	@echo "  DAEMON_PORT=$(DAEMON_PORT) - Daemon port for TUI (default: 8080)"
	@echo ""
	@echo "Examples:"
	@echo "  make run-daemon IFACE=eth0"
	@echo "  make run-tui DAEMON_HOST=192.168.1.100"