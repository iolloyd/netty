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
}