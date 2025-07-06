package capture

import (
	"sync"
	"sync/atomic"
	"time"
)

// PacketStats tracks packet capture statistics
type PacketStats struct {
	startTime       time.Time
	totalPackets    uint64
	totalBytes      uint64
	tcpPackets      uint64
	udpPackets      uint64
	droppedPackets  uint64
	processedEvents uint64
	lastPacketTime  time.Time
	mu              sync.RWMutex
}

// NewPacketStats creates a new statistics tracker
func NewPacketStats() *PacketStats {
	return &PacketStats{
		startTime: time.Now(),
	}
}

// IncrementPackets increments the packet counter
func (ps *PacketStats) IncrementPackets() {
	atomic.AddUint64(&ps.totalPackets, 1)
}

// IncrementBytes adds to the byte counter
func (ps *PacketStats) IncrementBytes(bytes uint64) {
	atomic.AddUint64(&ps.totalBytes, bytes)
}

// IncrementTCP increments TCP packet counter
func (ps *PacketStats) IncrementTCP() {
	atomic.AddUint64(&ps.tcpPackets, 1)
}

// IncrementUDP increments UDP packet counter
func (ps *PacketStats) IncrementUDP() {
	atomic.AddUint64(&ps.udpPackets, 1)
}

// IncrementDropped increments dropped packet counter
func (ps *PacketStats) IncrementDropped() {
	atomic.AddUint64(&ps.droppedPackets, 1)
}

// IncrementProcessed increments processed events counter
func (ps *PacketStats) IncrementProcessed() {
	atomic.AddUint64(&ps.processedEvents, 1)
}

// UpdateLastPacketTime updates the last packet timestamp
func (ps *PacketStats) UpdateLastPacketTime() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.lastPacketTime = time.Now()
}

// GetStats returns a snapshot of current statistics
func (ps *PacketStats) GetStats() map[string]interface{} {
	ps.mu.RLock()
	lastPacket := ps.lastPacketTime
	ps.mu.RUnlock()

	uptime := time.Since(ps.startTime).Seconds()
	totalPackets := atomic.LoadUint64(&ps.totalPackets)
	
	stats := map[string]interface{}{
		"uptime_seconds":     uptime,
		"total_packets":      totalPackets,
		"total_bytes":        atomic.LoadUint64(&ps.totalBytes),
		"tcp_packets":        atomic.LoadUint64(&ps.tcpPackets),
		"udp_packets":        atomic.LoadUint64(&ps.udpPackets),
		"dropped_packets":    atomic.LoadUint64(&ps.droppedPackets),
		"processed_events":   atomic.LoadUint64(&ps.processedEvents),
		"packets_per_second": float64(totalPackets) / uptime,
	}

	if !lastPacket.IsZero() {
		stats["last_packet_ago_seconds"] = time.Since(lastPacket).Seconds()
		stats["last_packet_time"] = lastPacket.Format(time.RFC3339)
	} else {
		stats["last_packet_ago_seconds"] = -1
		stats["last_packet_time"] = "never"
	}

	return stats
}