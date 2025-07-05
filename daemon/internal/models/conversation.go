package models

import (
	"fmt"
	"net"
	"time"
)

// ConversationState represents the state of a network conversation
type ConversationState string

const (
	ConversationStateNew         ConversationState = "NEW"
	ConversationStateEstablished ConversationState = "ESTABLISHED"
	ConversationStateClosing     ConversationState = "CLOSING"
	ConversationStateClosed      ConversationState = "CLOSED"
)

// ConversationKey uniquely identifies a network conversation using the 5-tuple
type ConversationKey struct {
	Protocol string
	SrcIP    string
	SrcPort  uint16
	DstIP    string
	DstPort  uint16
}

// String returns a string representation of the conversation key
func (ck ConversationKey) String() string {
	return fmt.Sprintf("%s:%s:%d->%s:%d", ck.Protocol, ck.SrcIP, ck.SrcPort, ck.DstIP, ck.DstPort)
}

// Reverse returns the reversed conversation key (for bidirectional matching)
func (ck ConversationKey) Reverse() ConversationKey {
	return ConversationKey{
		Protocol: ck.Protocol,
		SrcIP:    ck.DstIP,
		SrcPort:  ck.DstPort,
		DstIP:    ck.SrcIP,
		DstPort:  ck.SrcPort,
	}
}

// Normalize ensures consistent ordering of src/dst for bidirectional flows
func (ck ConversationKey) Normalize() ConversationKey {
	// Compare IPs first, then ports
	srcIP := net.ParseIP(ck.SrcIP)
	dstIP := net.ParseIP(ck.DstIP)
	
	if srcIP == nil || dstIP == nil {
		return ck
	}
	
	// Use lexicographical ordering of IPs, then ports
	if srcIP.String() > dstIP.String() || 
		(srcIP.String() == dstIP.String() && ck.SrcPort > ck.DstPort) {
		return ck.Reverse()
	}
	
	return ck
}

// ConversationStats tracks statistics for a conversation
type ConversationStats struct {
	PacketsIn    uint64
	PacketsOut   uint64
	BytesIn      uint64
	BytesOut     uint64
	FirstPacket  time.Time
	LastActivity time.Time
}

// Conversation represents an ongoing network conversation between two endpoints
type Conversation struct {
	ID          string            // Unique conversation ID
	Key         ConversationKey   // 5-tuple identifying the conversation
	State       ConversationState // Current state of the conversation
	StartTime   time.Time         // When the conversation started
	EndTime     *time.Time        // When the conversation ended (if closed)
	Stats       ConversationStats // Traffic statistics
	
	// TCP-specific fields
	TCPState    *TCPConversationState // TCP state tracking
	
	// Application layer info
	Service     string            // Detected service/application
	Hostname    string            // Resolved hostname if available
}

// TCPConversationState tracks TCP-specific conversation state
type TCPConversationState struct {
	// Connection establishment
	SYNSeen      bool
	SYNACKSeen   bool
	ACKSeen      bool
	
	// Sequence tracking
	InitialSeqClient uint32
	InitialSeqServer uint32
	LastSeqClient    uint32
	LastSeqServer    uint32
	
	// Connection termination
	FINSeenClient bool
	FINSeenServer bool
	RSTSeen       bool
	
	// Window sizes
	WindowClient uint16
	WindowServer uint16
}

// Duration returns the duration of the conversation
func (c *Conversation) Duration() time.Duration {
	if c.EndTime != nil {
		return c.EndTime.Sub(c.StartTime)
	}
	return time.Since(c.StartTime)
}

// IsActive returns true if the conversation is still active
func (c *Conversation) IsActive() bool {
	return c.State == ConversationStateNew || c.State == ConversationStateEstablished
}

// TotalPackets returns the total number of packets in the conversation
func (c *Conversation) TotalPackets() uint64 {
	return c.Stats.PacketsIn + c.Stats.PacketsOut
}

// TotalBytes returns the total number of bytes in the conversation
func (c *Conversation) TotalBytes() uint64 {
	return c.Stats.BytesIn + c.Stats.BytesOut
}

// ConversationSummary provides a simplified view of a conversation for UI display
type ConversationSummary struct {
	ID           string            `json:"id"`
	Protocol     string            `json:"protocol"`
	LocalAddr    string            `json:"local_addr"`
	RemoteAddr   string            `json:"remote_addr"`
	State        ConversationState `json:"state"`
	Duration     string            `json:"duration"`
	PacketsIn    uint64            `json:"packets_in"`
	PacketsOut   uint64            `json:"packets_out"`
	BytesIn      uint64            `json:"bytes_in"`
	BytesOut     uint64            `json:"bytes_out"`
	Service      string            `json:"service,omitempty"`
	LastActivity time.Time         `json:"last_activity"`
}

// ToSummary converts a Conversation to a ConversationSummary
func (c *Conversation) ToSummary(localIP string) ConversationSummary {
	var localAddr, remoteAddr string
	
	// Determine which side is local
	if c.Key.SrcIP == localIP {
		localAddr = fmt.Sprintf("%s:%d", c.Key.SrcIP, c.Key.SrcPort)
		remoteAddr = fmt.Sprintf("%s:%d", c.Key.DstIP, c.Key.DstPort)
	} else {
		localAddr = fmt.Sprintf("%s:%d", c.Key.DstIP, c.Key.DstPort)
		remoteAddr = fmt.Sprintf("%s:%d", c.Key.SrcIP, c.Key.SrcPort)
	}
	
	return ConversationSummary{
		ID:           c.ID,
		Protocol:     c.Key.Protocol,
		LocalAddr:    localAddr,
		RemoteAddr:   remoteAddr,
		State:        c.State,
		Duration:     c.Duration().Round(time.Second).String(),
		PacketsIn:    c.Stats.PacketsIn,
		PacketsOut:   c.Stats.PacketsOut,
		BytesIn:      c.Stats.BytesIn,
		BytesOut:     c.Stats.BytesOut,
		Service:      c.Service,
		LastActivity: c.Stats.LastActivity,
	}
}