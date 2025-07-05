package conversation

import (
	"sync"
	"time"
	
	"github.com/google/uuid"
	"github.com/iolloyd/netty/daemon/internal/models"
)

// Manager manages network conversations
type Manager struct {
	conversations map[string]*models.Conversation
	keyToID       map[string]string // Maps normalized conversation keys to IDs
	mu            sync.RWMutex
	
	// Configuration
	tcpTimeout time.Duration
	udpTimeout time.Duration
	localIP    string
}

// NewManager creates a new conversation manager
func NewManager(localIP string) *Manager {
	return &Manager{
		conversations: make(map[string]*models.Conversation),
		keyToID:       make(map[string]string),
		tcpTimeout:    5 * time.Minute,  // TCP connections timeout after 5 minutes of inactivity
		udpTimeout:    30 * time.Second, // UDP flows timeout after 30 seconds
		localIP:       localIP,
	}
}

// ProcessEvent processes a network event and updates conversations
func (m *Manager) ProcessEvent(event *models.NetworkEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Create conversation key from event
	key := models.ConversationKey{
		Protocol: event.TransportProtocol,
		SrcIP:    event.SourceIP,
		SrcPort:  uint16(event.SourcePort),
		DstIP:    event.DestIP,
		DstPort:  uint16(event.DestPort),
	}
	
	// Normalize the key for bidirectional matching
	normalizedKey := key.Normalize()
	normalizedKeyStr := normalizedKey.String()
	
	// Check if conversation exists
	conversationID, exists := m.keyToID[normalizedKeyStr]
	var conv *models.Conversation
	
	if exists {
		conv = m.conversations[conversationID]
	} else {
		// Create new conversation
		conversationID = uuid.New().String()
		conv = &models.Conversation{
			ID:        conversationID,
			Key:       normalizedKey,
			State:     models.ConversationStateNew,
			StartTime: event.Timestamp,
			Stats: models.ConversationStats{
				FirstPacket: event.Timestamp,
			},
		}
		
		// Initialize TCP state if TCP
		if event.TransportProtocol == "TCP" && event.TCPFlags != nil {
			conv.TCPState = &models.TCPConversationState{}
		}
		
		m.conversations[conversationID] = conv
		m.keyToID[normalizedKeyStr] = conversationID
	}
	
	// Update event with conversation ID
	event.ConversationID = conversationID
	
	// Update conversation statistics
	m.updateConversationStats(conv, event, key)
	
	// Update TCP state if applicable
	if event.TransportProtocol == "TCP" && event.TCPFlags != nil {
		m.updateTCPState(conv, event, key)
	}
	
	// Detect service/application
	m.detectService(conv, event)
}

// updateConversationStats updates conversation statistics based on the event
func (m *Manager) updateConversationStats(conv *models.Conversation, event *models.NetworkEvent, key models.ConversationKey) {
	conv.Stats.LastActivity = event.Timestamp
	
	// Determine direction based on local IP
	isOutgoing := key.SrcIP == m.localIP
	
	if isOutgoing {
		conv.Stats.PacketsOut++
		conv.Stats.BytesOut += uint64(event.Size)
	} else {
		conv.Stats.PacketsIn++
		conv.Stats.BytesIn += uint64(event.Size)
	}
}

// updateTCPState updates the TCP state machine for the conversation
func (m *Manager) updateTCPState(conv *models.Conversation, event *models.NetworkEvent, key models.ConversationKey) {
	if conv.TCPState == nil {
		return
	}
	
	flags := event.TCPFlags
	tcpState := conv.TCPState
	
	// Track which side sent this packet
	isClient := key.SrcIP == conv.Key.SrcIP && key.SrcPort == conv.Key.SrcPort
	
	// Handle SYN flag
	if flags.SYN && !flags.ACK {
		tcpState.SYNSeen = true
		if isClient {
			tcpState.InitialSeqClient = event.SequenceNumber
		} else {
			tcpState.InitialSeqServer = event.SequenceNumber
		}
		conv.State = models.ConversationStateNew
	}
	
	// Handle SYN-ACK
	if flags.SYN && flags.ACK {
		tcpState.SYNACKSeen = true
		if !isClient {
			tcpState.InitialSeqServer = event.SequenceNumber
		}
	}
	
	// Handle ACK (connection established)
	if flags.ACK && !flags.SYN && tcpState.SYNSeen && tcpState.SYNACKSeen && !tcpState.ACKSeen {
		tcpState.ACKSeen = true
		conv.State = models.ConversationStateEstablished
	}
	
	// Update sequence numbers
	if isClient {
		tcpState.LastSeqClient = event.SequenceNumber
	} else {
		tcpState.LastSeqServer = event.SequenceNumber
	}
	
	// Handle FIN flag
	if flags.FIN {
		if isClient {
			tcpState.FINSeenClient = true
		} else {
			tcpState.FINSeenServer = true
		}
		
		// If both sides have sent FIN, connection is closing
		if tcpState.FINSeenClient && tcpState.FINSeenServer {
			conv.State = models.ConversationStateClosing
		} else {
			conv.State = models.ConversationStateClosing
		}
	}
	
	// Handle RST flag
	if flags.RST {
		tcpState.RSTSeen = true
		conv.State = models.ConversationStateClosed
		now := event.Timestamp
		conv.EndTime = &now
	}
}

// detectService attempts to identify the service based on port and protocol
func (m *Manager) detectService(conv *models.Conversation, event *models.NetworkEvent) {
	// Skip if service already detected
	if conv.Service != "" {
		return
	}
	
	// Common port-based service detection
	services := map[int]string{
		20:   "FTP-DATA",
		21:   "FTP",
		22:   "SSH",
		23:   "TELNET",
		25:   "SMTP",
		53:   "DNS",
		80:   "HTTP",
		110:  "POP3",
		143:  "IMAP",
		443:  "HTTPS",
		445:  "SMB",
		587:  "SMTP-TLS",
		993:  "IMAPS",
		995:  "POP3S",
		1433: "MSSQL",
		3306: "MySQL",
		3389: "RDP",
		5432: "PostgreSQL",
		5900: "VNC",
		6379: "Redis",
		8080: "HTTP-ALT",
		8443: "HTTPS-ALT",
		9200: "Elasticsearch",
		27017: "MongoDB",
	}
	
	// Check destination port first (more likely to be the service port)
	if service, ok := services[event.DestPort]; ok {
		conv.Service = service
	} else if service, ok := services[event.SourcePort]; ok {
		conv.Service = service
	}
	
	// Override with app protocol if available
	if event.AppProtocol != "" {
		conv.Service = event.AppProtocol
	}
}

// GetConversation returns a conversation by ID
func (m *Manager) GetConversation(id string) (*models.Conversation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	conv, exists := m.conversations[id]
	return conv, exists
}

// GetActiveConversations returns all active conversations
func (m *Manager) GetActiveConversations() []*models.Conversation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var active []*models.Conversation
	for _, conv := range m.conversations {
		if conv.IsActive() {
			active = append(active, conv)
		}
	}
	
	return active
}

// GetAllConversations returns all conversations
func (m *Manager) GetAllConversations() []*models.Conversation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var all []*models.Conversation
	for _, conv := range m.conversations {
		all = append(all, conv)
	}
	
	return all
}

// CleanupStaleConversations removes conversations that have been inactive
func (m *Manager) CleanupStaleConversations() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	
	for id, conv := range m.conversations {
		var timeout time.Duration
		if conv.Key.Protocol == "TCP" {
			timeout = m.tcpTimeout
		} else {
			timeout = m.udpTimeout
		}
		
		// Check if conversation has timed out
		if now.Sub(conv.Stats.LastActivity) > timeout {
			// Mark as closed if not already
			if conv.State != models.ConversationStateClosed {
				conv.State = models.ConversationStateClosed
				conv.EndTime = &now
			}
			
			// Remove very old conversations (>1 hour)
			if now.Sub(conv.Stats.LastActivity) > time.Hour {
				delete(m.conversations, id)
				delete(m.keyToID, conv.Key.Normalize().String())
			}
		}
	}
}

// StartCleanupRoutine starts a goroutine to periodically clean up stale conversations
func (m *Manager) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			m.CleanupStaleConversations()
		}
	}()
}

// GetConversationSummaries returns summaries of all conversations
func (m *Manager) GetConversationSummaries() []models.ConversationSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	summaries := make([]models.ConversationSummary, 0, len(m.conversations))
	for _, conv := range m.conversations {
		summaries = append(summaries, conv.ToSummary(m.localIP))
	}
	
	return summaries
}