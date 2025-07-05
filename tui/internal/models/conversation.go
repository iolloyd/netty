package models

import (
	"fmt"
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

// ConversationKey uniquely identifies a network conversation
type ConversationKey struct {
	Protocol string `json:"protocol"`
	SrcIP    string `json:"src_ip"`
	SrcPort  int    `json:"src_port"`
	DstIP    string `json:"dst_ip"`
	DstPort  int    `json:"dst_port"`
}

// Conversation represents a network conversation between two endpoints
type Conversation struct {
	ID             string            `json:"id"`
	Protocol       string            `json:"protocol"`
	LocalAddr      string            `json:"local_addr"`
	RemoteAddr     string            `json:"remote_addr"`
	State          ConversationState `json:"state"`
	Duration       string            `json:"duration"`
	PacketsIn      int64             `json:"packets_in"`
	PacketsOut     int64             `json:"packets_out"`
	BytesIn        int64             `json:"bytes_in"`
	BytesOut       int64             `json:"bytes_out"`
	Service        string            `json:"service,omitempty"`
	LastActivity   time.Time         `json:"last_activity"`
}

// TCPFlags tracks which TCP flags have been seen in the conversation
type TCPFlags struct {
	SYN bool `json:"syn"`
	ACK bool `json:"ack"`
	FIN bool `json:"fin"`
	RST bool `json:"rst"`
	PSH bool `json:"psh"`
	URG bool `json:"urg"`
}

// DurationValue returns the duration of the conversation as a parsed duration
func (c *Conversation) DurationValue() time.Duration {
	d, _ := time.ParseDuration(c.Duration)
	return d
}

// IsActive returns true if the conversation is still active
func (c *Conversation) IsActive() bool {
	return c.State == ConversationStateNew || c.State == ConversationStateEstablished
}

// TotalPackets returns the total number of packets in the conversation
func (c *Conversation) TotalPackets() int64 {
	return c.PacketsIn + c.PacketsOut
}

// TotalBytes returns the total number of bytes in the conversation
func (c *Conversation) TotalBytes() int64 {
	return c.BytesIn + c.BytesOut
}

// GetEndpointPair returns a formatted string of the conversation endpoints
func (c *Conversation) GetEndpointPair() string {
	return fmt.Sprintf("%s → %s", c.LocalAddr, c.RemoteAddr)
}

// GetServiceInfo returns a formatted string of the service/protocol
func (c *Conversation) GetServiceInfo() string {
	if c.Service != "" {
		return c.Service
	}
	return c.Protocol
}