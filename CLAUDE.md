# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Netty is a network monitoring application designed to provide human-consumable information about network traffic. The project is currently in the initial setup phase with no implementation yet.

## Project Status

The project is being implemented with:
- **Backend**: Go daemon using gopacket for network capture
- **Communication**: WebSocket server for real-time updates
- **Frontend**: React with Electron (planned)

## Core Requirements

The application must include:
- A daemon for continuous network monitoring
- A UI for displaying network information in real-time
- Capabilities to monitor:
  - Incoming network requests
  - Outgoing network requests
  - Machine/IP addresses involved in request-response events

## Development Commands

### Daemon (Go)
```bash
# Build the daemon
cd daemon && go build -o netty-daemon cmd/netty-daemon/main.go

# Run the daemon (requires root/admin privileges)
sudo ./netty-daemon -i en0  # Replace en0 with your network interface

# Run with verbose logging
sudo ./netty-daemon -i en0 -v

# Run with BPF filter
sudo ./netty-daemon -i en0 -f "tcp port 80 or tcp port 443"
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

2. **React UI** (planned): Will connect to daemon via WebSocket for real-time display

## Development Notes

- The daemon requires root/admin privileges for packet capture
- WebSocket server runs on port 8080 by default
- Network events are broadcast to all connected WebSocket clients
- Use the health endpoint (`http://localhost:8080/health`) to check daemon status