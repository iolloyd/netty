package capture

import (
	"fmt"
	"log"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/iolloyd/netty/daemon/internal/conversation"
	"github.com/iolloyd/netty/daemon/internal/models"
	"github.com/iolloyd/netty/daemon/internal/parser"
	"github.com/iolloyd/netty/daemon/internal/resolver"
)

type PacketCapture struct {
	handle      *pcap.Handle
	iface       string
	filter      string
	convMgr     *conversation.Manager
	dnsResolver *resolver.DNSResolver
}

func NewPacketCapture(iface, filter, localIP string) (*PacketCapture, error) {
	handle, err := pcap.OpenLive(iface, 65536, true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("failed to open interface %s: %w", iface, err)
	}

	if filter != "" {
		if err := handle.SetBPFFilter(filter); err != nil {
			handle.Close()
			return nil, fmt.Errorf("failed to set BPF filter: %w", err)
		}
	}

	// Create conversation manager with local IP
	convMgr := conversation.NewManager(localIP)
	convMgr.StartCleanupRoutine()

	// Create DNS resolver with 5 minute TTL
	dnsResolver := resolver.NewDNSResolver(5 * time.Minute)
	dnsResolver.StartCleanup(time.Minute)

	return &PacketCapture{
		handle:      handle,
		iface:       iface,
		filter:      filter,
		convMgr:     convMgr,
		dnsResolver: dnsResolver,
	}, nil
}

func (pc *PacketCapture) Start() <-chan *models.NetworkEvent {
	events := make(chan *models.NetworkEvent, 100)
	
	go func() {
		defer close(events)
		packetSource := gopacket.NewPacketSource(pc.handle, pc.handle.LinkType())
		
		for packet := range packetSource.Packets() {
			event := pc.processPacket(packet)
			if event != nil {
				// Process packet through conversation manager
				pc.convMgr.ProcessEvent(event)
				
				select {
				case events <- event:
				default:
					log.Println("Event channel full, dropping packet")
				}
			}
		}
	}()
	
	return events
}

func (pc *PacketCapture) processPacket(packet gopacket.Packet) *models.NetworkEvent {
	event := &models.NetworkEvent{
		Timestamp: time.Now(),
		Interface: pc.iface,
	}

	// Extract network layer
	if netLayer := packet.NetworkLayer(); netLayer != nil {
		switch net := netLayer.(type) {
		case *layers.IPv4:
			event.Protocol = "IPv4"
			event.SourceIP = net.SrcIP.String()
			event.DestIP = net.DstIP.String()
		case *layers.IPv6:
			event.Protocol = "IPv6"
			event.SourceIP = net.SrcIP.String()
			event.DestIP = net.DstIP.String()
		}
	}

	// Extract transport layer
	if transLayer := packet.TransportLayer(); transLayer != nil {
		switch trans := transLayer.(type) {
		case *layers.TCP:
			event.TransportProtocol = "TCP"
			event.SourcePort = int(trans.SrcPort)
			event.DestPort = int(trans.DstPort)
			
			// Extract TCP flags
			event.TCPFlags = &models.TCPPacketFlags{
				SYN: trans.SYN,
				ACK: trans.ACK,
				FIN: trans.FIN,
				RST: trans.RST,
				PSH: trans.PSH,
				URG: trans.URG,
			}
			
			// Extract sequence and acknowledgment numbers
			event.SequenceNumber = trans.Seq
			event.AckNumber = trans.Ack
			
			// Determine direction based on SYN/ACK flags
			if trans.SYN && !trans.ACK {
				event.Direction = "outgoing"
			} else if trans.SYN && trans.ACK {
				event.Direction = "incoming"
			} else {
				// For established connections, use port heuristics
				if trans.DstPort < 1024 || isCommonPort(int(trans.DstPort)) {
					event.Direction = "outgoing"
				} else if trans.SrcPort < 1024 || isCommonPort(int(trans.SrcPort)) {
					event.Direction = "incoming"
				} else {
					event.Direction = "unknown"
				}
			}
			
			// Try to extract TLS SNI if this is HTTPS traffic
			if trans.DstPort == 443 || trans.SrcPort == 443 {
				if payload := trans.LayerPayload(); len(payload) > 0 {
					if sni := parser.ExtractSNI(payload); sni != "" {
						event.TLSServerName = sni
					}
				}
			}
		case *layers.UDP:
			event.TransportProtocol = "UDP"
			event.SourcePort = int(trans.SrcPort)
			event.DestPort = int(trans.DstPort)
			
			// Use port heuristics for UDP
			if trans.DstPort < 1024 || isCommonPort(int(trans.DstPort)) {
				event.Direction = "outgoing"
			} else if trans.SrcPort < 1024 || isCommonPort(int(trans.SrcPort)) {
				event.Direction = "incoming"
			} else {
				event.Direction = "unknown"
			}
		}
	}

	// Calculate packet size
	event.Size = len(packet.Data())

	// Extract application layer if present
	if appLayer := packet.ApplicationLayer(); appLayer != nil {
		event.AppProtocol = guessAppProtocol(event.SourcePort, event.DestPort)
	}

	// Perform DNS resolution (using cached results when available)
	if event.SourceIP != "" && event.DestIP != "" {
		event.SourceHostname = pc.dnsResolver.ResolveIP(event.SourceIP)
		event.DestHostname = pc.dnsResolver.ResolveIP(event.DestIP)
	}

	return event
}

func (pc *PacketCapture) Close() {
	if pc.handle != nil {
		pc.handle.Close()
	}
	// Conversation manager cleanup is handled by its goroutine
}

// GetConversationManager returns the conversation manager
func (pc *PacketCapture) GetConversationManager() *conversation.Manager {
	return pc.convMgr
}

func isCommonPort(port int) bool {
	commonPorts := map[int]bool{
		80:   true, // HTTP
		443:  true, // HTTPS
		22:   true, // SSH
		21:   true, // FTP
		25:   true, // SMTP
		53:   true, // DNS
		3306: true, // MySQL
		5432: true, // PostgreSQL
		6379: true, // Redis
		27017: true, // MongoDB
	}
	return commonPorts[port]
}

func guessAppProtocol(srcPort, dstPort int) string {
	portMap := map[int]string{
		80:   "HTTP",
		443:  "HTTPS",
		22:   "SSH",
		21:   "FTP",
		25:   "SMTP",
		53:   "DNS",
		3306: "MySQL",
		5432: "PostgreSQL",
		6379: "Redis",
		27017: "MongoDB",
	}
	
	if proto, ok := portMap[dstPort]; ok {
		return proto
	}
	if proto, ok := portMap[srcPort]; ok {
		return proto
	}
	return ""
}