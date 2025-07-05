package models

import (
	"time"
)

type NetworkEvent struct {
	Timestamp         time.Time `json:"timestamp"`
	Interface         string    `json:"interface"`
	Direction         string    `json:"direction"` // incoming, outgoing, unknown
	Protocol          string    `json:"protocol"`   // IPv4, IPv6
	TransportProtocol string    `json:"transport_protocol"` // TCP, UDP
	AppProtocol       string    `json:"app_protocol,omitempty"` // HTTP, HTTPS, SSH, etc.
	SourceIP          string    `json:"source_ip"`
	DestIP            string    `json:"dest_ip"`
	SourcePort        int       `json:"source_port"`
	DestPort          int       `json:"dest_port"`
	Size              int       `json:"size"`
	
	// Hostname resolution
	SourceHostname    string    `json:"source_hostname,omitempty"`
	DestHostname      string    `json:"dest_hostname,omitempty"`
	
	// TLS information
	TLSServerName     string    `json:"tls_server_name,omitempty"` // SNI hostname
	
	// Conversation tracking
	ConversationID    string    `json:"conversation_id,omitempty"`
	
	// TCP-specific fields for tracking
	TCPFlags          *TCPPacketFlags `json:"tcp_flags,omitempty"`
	SequenceNumber    uint32    `json:"sequence_number,omitempty"`
	AckNumber         uint32    `json:"ack_number,omitempty"`
}

// TCPPacketFlags represents TCP flags for a single packet
type TCPPacketFlags struct {
	SYN bool `json:"syn"`
	ACK bool `json:"ack"`
	FIN bool `json:"fin"`
	RST bool `json:"rst"`
	PSH bool `json:"psh"`
	URG bool `json:"urg"`
}