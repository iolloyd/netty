# Netty TUI (Terminal User Interface)

A real-time terminal interface for monitoring network traffic captured by the Netty daemon.

## Features

- Real-time network event display
- Scrollable event list with keyboard navigation
- Connection status indicator
- Network statistics (packets, bytes, protocol breakdown)
- Color-coded traffic direction (inbound/outbound)
- Vi-like keyboard shortcuts
- Help screen

## Building

```bash
go build -o netty-tui cmd/netty-tui/main.go
```

## Usage

First, ensure the Netty daemon is running:
```bash
# In the daemon directory
sudo ./netty-daemon -i en0
```

Then run the TUI:
```bash
./netty-tui
```

Connect to a remote daemon:
```bash
./netty-tui -host 192.168.1.100 -port 8080
```

## Keyboard Shortcuts

- `j/↓` - Move down
- `k/↑` - Move up
- `g` - Go to top
- `G` - Go to bottom
- `Ctrl+d` - Page down
- `Ctrl+u` - Page up
- `c` - Clear all events
- `f` - Open filter dialog (coming soon)
- `?/h` - Toggle help
- `q` - Quit

## Architecture

The TUI is built using:
- **Bubbletea** - Terminal UI framework with Elm Architecture
- **Lipgloss** - Styling and layout
- **Gorilla WebSocket** - Real-time connection to daemon

The TUI connects to the daemon's WebSocket endpoint and displays network events in real-time. Multiple TUI instances can connect to a single daemon.