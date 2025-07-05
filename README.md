# Netty - Network Monitor

A human-readable network monitoring tool that captures and displays network traffic in real-time.

## Features

- **Real-time packet capture** with DNS hostname resolution
- **TLS SNI extraction** to show actual HTTPS domain names  
- **Conversation tracking** to group related network flows
- **Terminal UI** with keyboard navigation
- **WebSocket API** for real-time updates
- **Service detection** for common protocols

## Quick Start

```bash
# Build everything
make build

# Option 1: Run in separate terminals
# Terminal 1:
make run-daemon

# Terminal 2:
make run-tui

# Option 2: Use the startup script
./start.sh

# Option 3: See run instructions
make run
```

## Installation

### Prerequisites

- Go 1.19 or later
- Root/sudo access (for packet capture)
- macOS or Linux

### Install to System

```bash
make install
```

This installs `netty-daemon` and `netty-tui` to `/usr/local/bin/`.

## Usage

### Running the Daemon

The daemon requires root privileges to capture network packets:

```bash
sudo netty-daemon -i en0
```

Options:
- `-i <interface>`: Network interface to monitor (required)
- `-f <filter>`: BPF filter expression (e.g., "tcp port 80 or tcp port 443")
- `-v`: Enable verbose logging
- `-p <port>`: WebSocket server port (default: 8080)

### Running the TUI

Connect to the daemon's WebSocket server:

```bash
netty-tui
```

Options:
- `-host <host>`: Daemon host (default: localhost)
- `-port <port>`: Daemon port (default: 8080)

### TUI Keyboard Shortcuts

- `j/↓`: Move down
- `k/↑`: Move up  
- `g`: Go to top
- `G`: Go to bottom
- `Tab`: Switch between packets/conversations view
- `c`: Clear events
- `?/h`: Show help
- `q`: Quit

## Development

### Building

```bash
# Build daemon only
make daemon

# Build TUI only  
make tui

# Build both
make build
```

### Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage
```

### Code Quality

```bash
# Format code
make fmt

# Lint code (requires golangci-lint)
make lint
```

## Architecture

- **Daemon** (`daemon/`): Go service that captures packets using gopacket
- **TUI** (`tui/`): Terminal UI built with Bubbletea
- **Communication**: WebSocket for real-time event streaming

## License

MIT