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
	stats       *PacketStats
}

func NewPacketCapture(iface, filter, localIP string) (*PacketCapture, error) {
	log.Printf("[DEBUG] Opening packet capture on interface: %s", iface)
	handle, err := pcap.OpenLive(iface, 65536, true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("failed to open interface %s: %w", iface, err)
	}
	log.Printf("[DEBUG] Successfully opened interface %s", iface)

	if filter != "" {
		log.Printf("[DEBUG] Setting BPF filter: %s", filter)
		if err := handle.SetBPFFilter(filter); err != nil {
			handle.Close()
			return nil, fmt.Errorf("failed to set BPF filter: %w", err)
		}
		log.Printf("[DEBUG] BPF filter set successfully")
	} else {
		log.Printf("[DEBUG] No BPF filter specified, capturing all traffic")
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
		stats:       NewPacketStats(),
	}, nil
}

func (pc *PacketCapture) Start() <-chan *models.NetworkEvent {
	events := make(chan *models.NetworkEvent, 100)
	
	go func() {
		defer close(events)
		packetSource := gopacket.NewPacketSource(pc.handle, pc.handle.LinkType())
		log.Printf("[DEBUG] Starting packet capture loop on interface %s", pc.iface)
		
		// Start a timer to check if we're receiving packets
		noPacketTimer := time.NewTimer(10 * time.Second)
		defer noPacketTimer.Stop()
		
		go func() {
			<-noPacketTimer.C
			stats := pc.stats.GetStats()
			if stats["total_packets"].(uint64) == 0 {
				log.Printf("[WARNING] No packets captured after 10 seconds on interface %s", pc.iface)
				log.Printf("[WARNING] Possible issues:")
				log.Printf("[WARNING]   - Wrong interface (use -list to see available interfaces)")
				log.Printf("[WARNING]   - No network traffic on the interface")
				log.Printf("[WARNING]   - BPF filter too restrictive")
				log.Printf("[WARNING]   - Insufficient permissions (run with sudo)")
				log.Printf("[WARNING] Try running: sudo tcpdump -i %s -c 10", pc.iface)
			}
		}()
		
		packetCount := 0
		for packet := range packetSource.Packets() {
			packetCount++
			pc.stats.IncrementPackets()
			pc.stats.IncrementBytes(uint64(len(packet.Data())))
			pc.stats.UpdateLastPacketTime()
			
			// Reset timer on first packet
			if packetCount == 1 {
				noPacketTimer.Stop()
				log.Printf("[INFO] Successfully capturing packets on interface %s", pc.iface)
			}
			
			if packetCount%100 == 0 {
				log.Printf("[DEBUG] Captured %d packets so far", packetCount)
			}
			event := pc.processPacket(packet)
			if event != nil {
				if packetCount <= 10 {
					log.Printf("[DEBUG] Processed packet #%d: %s:%d -> %s:%d (%s)", 
						packetCount, event.SourceIP, event.SourcePort, 
						event.DestIP, event.DestPort, event.TransportProtocol)
				}
				// Process packet through conversation manager
				pc.convMgr.ProcessEvent(event)
				
				select {
				case events <- event:
					pc.stats.IncrementProcessed()
					if packetCount <= 10 {
						log.Printf("[DEBUG] Event sent to channel successfully")
					}
				default:
					pc.stats.IncrementDropped()
					log.Println("[WARNING] Event channel full, dropping packet")
				}
			} else {
				if packetCount <= 10 {
					log.Printf("[DEBUG] Packet #%d: No network/transport layer found", packetCount)
				}
			}
		}
	}()
	
	return events
}

func (pc *PacketCapture) processPacket(packet gopacket.Packet) *models.NetworkEvent {
	// Only return nil if packet has no network or transport layer
	if packet.NetworkLayer() == nil || packet.TransportLayer() == nil {
		return nil
	}
	
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
			pc.stats.IncrementTCP()
			
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
			pc.stats.IncrementUDP()
			
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

// GetStats returns packet capture statistics
func (pc *PacketCapture) GetStats() map[string]interface{} {
	return pc.stats.GetStats()
}