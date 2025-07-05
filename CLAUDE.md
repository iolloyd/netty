# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Netty is a network monitoring application designed to provide human-consumable information about network traffic. The project is currently in the initial setup phase with no implementation yet.

## Project Status

The project is being implemented with:
- **Backend**: Go daemon using gopacket for network capture
- **Communication**: WebSocket server for real-time updates
- **Frontend**: 
  - Terminal UI (TUI) using Bubbletea (implemented)
  - React with Electron (planned)

## Core Requirements

The application must include:
- A daemon for continuous network monitoring
- A UI for displaying network information in real-time
- Capabilities to monitor:
  - Incoming network requests
  - Outgoing network requests
  - Machine/IP addresses involved in request-response events

## Development Commands

### Using Makefile
```bash
# Build both daemon and TUI
make all

# Build daemon only
make build-daemon

# Build TUI only  
make build-tui

# Run daemon (builds first, requires sudo)
make run-daemon

# Run TUI (builds first)
make run-tui

# Run tests
make test

# Clean build artifacts
make clean
```

### Manual Commands
```bash
# Build the daemon
cd daemon && go build -o netty-daemon cmd/netty-daemon/main.go

# Run the daemon (requires root/admin privileges)
sudo ./netty-daemon -i en0  # Replace en0 with your network interface

# Run with verbose logging
sudo ./netty-daemon -i en0 -v

# Run with BPF filter
sudo ./netty-daemon -i en0 -f "tcp port 80 or tcp port 443"

# Build the TUI
cd tui && go build -o netty-tui cmd/netty-tui/main.go

# Run the TUI (connects to daemon on localhost:8080)
./netty-tui

# Run TUI with custom daemon host/port
./netty-tui -host 192.168.1.100 -port 8080
```

### Frontend (React) - To be implemented
```bash
# Commands will be added when UI is implemented
```

## Architecture

The application consists of:
1. **Go Daemon** (`daemon/`): Captures network packets and serves WebSocket API
   - `cmd/netty-daemon/`: Main application entry point
   - `internal/capture/`: Packet capture logic using gopacket
   - `internal/websocket/`: WebSocket server for real-time updates
   - `internal/models/`: Data models for network events

2. **Terminal UI** (`tui/`): Bubbletea-based TUI client
   - `cmd/netty-tui/`: TUI application entry point
   - `internal/ui/`: Bubbletea model and view components
   - `internal/websocket/`: WebSocket client for daemon connection
   - `internal/models/`: Shared data models

3. **React UI** (planned): Will connect to daemon via WebSocket for real-time display

## Development Notes

- The daemon requires root/admin privileges for packet capture
- WebSocket server runs on port 8080 by default
- Network events are broadcast to all connected WebSocket clients
- Use the health endpoint (`http://localhost:8080/health`) to check daemon status
- The TUI provides real-time network monitoring with keyboard navigation
- Multiple TUI clients can connect to a single daemon instance

## TUI Features

The Terminal UI provides:
- Real-time network event display with scrollable list
- DNS hostname resolution for IP addresses
- TLS SNI extraction showing actual HTTPS hostnames
- Keyboard navigation (j/k or arrow keys)
- Connection status indicator
- Network statistics (packets, bytes, protocol breakdown)
- Event filtering by protocol, IP, or port (coming soon)
- Color-coded traffic direction (inbound/outbound)
- Help screen (press '?' or 'h')

### TUI Keyboard Shortcuts
- `j/↓`: Move down
- `k/↑`: Move up
- `g`: Go to top
- `G`: Go to bottom
- `Ctrl+d`: Page down
- `Ctrl+u`: Page up
- `c`: Clear all events
- `f`: Open filter dialog (coming soon)
- `?/h`: Toggle help
- `q`: Quit