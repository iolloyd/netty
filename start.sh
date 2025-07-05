#!/bin/bash

# Netty startup script

echo "🚀 Starting Netty Network Monitor"
echo ""

# Check if running on macOS or Linux
if [[ "$OSTYPE" == "darwin"* ]]; then
    DEFAULT_IFACE="en0"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    DEFAULT_IFACE="eth0"
else
    DEFAULT_IFACE="eth0"
fi

# Use provided interface or default
IFACE=${1:-$DEFAULT_IFACE}

# Build first
echo "🔨 Building netty..."
make build || exit 1

echo ""
echo "📡 Starting daemon on interface: $IFACE"
echo "⚠️  The daemon requires sudo privileges for packet capture"
echo ""

# Function to cleanup on exit
cleanup() {
    echo ""
    echo "🛑 Shutting down netty..."
    # Kill any running daemon
    sudo pkill -f "netty-daemon" 2>/dev/null
    exit 0
}

# Set up trap for cleanup
trap cleanup INT TERM

# Start daemon in background
sudo ./daemon/netty-daemon -i "$IFACE" -v &
DAEMON_PID=$!

# Wait a moment for daemon to start
sleep 2

# Check if daemon is running
if ! sudo kill -0 $DAEMON_PID 2>/dev/null; then
    echo "❌ Failed to start daemon. Please check the error messages above."
    exit 1
fi

echo ""
echo "✅ Daemon started successfully"
echo "🖥️  Starting TUI..."
echo ""

# Start TUI in foreground
./tui/netty-tui

# When TUI exits, cleanup
cleanup