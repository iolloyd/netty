# Netty Daemon

Network monitoring daemon that captures packets and streams them via WebSocket.

## Requirements

- Go 1.21 or higher
- libpcap development files
  - macOS: `brew install libpcap`
  - Ubuntu/Debian: `sudo apt-get install libpcap-dev`
  - RHEL/CentOS: `sudo yum install libpcap-devel`
- Root/Administrator privileges for packet capture

## Building

```bash
go build -o netty-daemon cmd/netty-daemon/main.go
```

## Running

The daemon requires root privileges to capture packets:

```bash
# List available network interfaces
ifconfig -a

# Run the daemon (replace en0 with your interface)
sudo ./netty-daemon -i en0

# With verbose logging
sudo ./netty-daemon -i en0 -v

# With a BPF filter (capture only HTTP/HTTPS traffic)
sudo ./netty-daemon -i en0 -f "tcp port 80 or tcp port 443"

# Custom WebSocket port
sudo ./netty-daemon -i en0 -port 9090
```

## WebSocket API

Connect to `ws://localhost:8080/ws` to receive real-time network events.

Each event is a JSON object:

```json
{
  "timestamp": "2025-07-01T10:30:45Z",
  "interface": "en0",
  "direction": "outgoing",
  "protocol": "IPv4",
  "transport_protocol": "TCP",
  "app_protocol": "HTTPS",
  "source_ip": "192.168.1.100",
  "dest_ip": "151.101.1.140",
  "source_port": 54321,
  "dest_port": 443,
  "size": 1500
}
```

## Health Check

```bash
curl http://localhost:8080/health
```