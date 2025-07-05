package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/netty/tui/internal/models"
	"github.com/netty/tui/internal/websocket"
)

const (
	maxEvents = 1000
)

type Model struct {
	wsClient         *websocket.Client
	events           []models.NetworkEvent
	filteredEvents   []models.NetworkEvent
	conversations    []models.Conversation
	width            int
	height           int
	scrollOffset     int
	connected        bool
	connectionError  string
	connectionStatus string
	filter           Filter
	stats            Stats
	showHelp         bool
	selectedIndex    int
	viewMode         ViewMode
	lastConvUpdate   time.Time
}

type ViewMode int

const (
	ViewModePackets ViewMode = iota
	ViewModeConversations
)

type Filter struct {
	Protocol string
	IP       string
	Port     string
}

type Stats struct {
	TotalPackets   int
	TotalBytes     int
	ProtocolCounts map[string]int
	LastUpdate     time.Time
}

func NewModel(wsClient *websocket.Client) Model {
	return Model{
		wsClient:         wsClient,
		events:           make([]models.NetworkEvent, 0, maxEvents),
		filteredEvents:   make([]models.NetworkEvent, 0),
		connectionStatus: "Connecting to daemon...",
		stats: Stats{
			ProtocolCounts: make(map[string]int),
			LastUpdate:     time.Now(),
		},
		viewMode: ViewModePackets,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.wsClient.Connect(),
		tea.EnterAltScreen,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	
	case websocket.ConnectionStatusMsg:
		m.connected = msg.Connected
		if msg.Connected {
			m.connectionStatus = "Connected"
			m.connectionError = ""
			return m, m.wsClient.WaitForEvent()
		} else if msg.Error != nil {
			m.connectionError = msg.Error.Error()
			m.connectionStatus = fmt.Sprintf("Connection failed: %s", msg.Error.Error())
			// Attempt to reconnect after a delay
			return m, tea.Sequence(
				tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					m.connectionStatus = "Reconnecting..."
					return nil
				}),
				m.wsClient.Reconnect(),
			)
		}
		return m, nil
	
	case websocket.EventMsg:
		event := models.NetworkEvent(msg)
		m.addEvent(event)
		m.updateStats(event)
		m.applyFilter()
		// Periodically request conversation updates
		if time.Since(m.lastConvUpdate) > 2*time.Second && m.viewMode == ViewModeConversations {
			m.lastConvUpdate = time.Now()
			return m, tea.Batch(m.wsClient.WaitForEvent(), m.requestConversations())
		}
		return m, m.wsClient.WaitForEvent()
	
	case websocket.ConversationsMsg:
		m.conversations = []models.Conversation(msg)
		// Sort conversations by last activity (most recent first)
		sort.Slice(m.conversations, func(i, j int) bool {
			return m.conversations[i].LastActivity.After(m.conversations[j].LastActivity)
		})
		return m, nil
	}
	
	return m, nil
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	
	case "?", "h":
		m.showHelp = !m.showHelp
		return m, nil
	
	case "j", "down":
		maxItems := len(m.filteredEvents) - 1
		if m.viewMode == ViewModeConversations {
			maxItems = len(m.conversations) - 1
		}
		if m.selectedIndex < maxItems {
			m.selectedIndex++
			m.ensureSelectedVisible()
		}
		return m, nil
	
	case "k", "up":
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.ensureSelectedVisible()
		}
		return m, nil
	
	case "G":
		if m.viewMode == ViewModePackets {
			m.selectedIndex = len(m.filteredEvents) - 1
		} else {
			m.selectedIndex = len(m.conversations) - 1
		}
		m.ensureSelectedVisible()
		return m, nil
	
	case "g":
		m.selectedIndex = 0
		m.scrollOffset = 0
		return m, nil
	
	case "ctrl+d":
		m.scrollDown(m.height / 2)
		return m, nil
	
	case "ctrl+u":
		m.scrollUp(m.height / 2)
		return m, nil
	
	case "c":
		m.clearEvents()
		return m, nil
	
	case "f":
		// TODO: Implement filter dialog
		return m, nil
	
	case "tab":
		// Toggle between packets and conversations view
		if m.viewMode == ViewModePackets {
			m.viewMode = ViewModeConversations
			m.selectedIndex = 0
			m.scrollOffset = 0
			// Request conversation update
			return m, m.requestConversations()
		} else {
			m.viewMode = ViewModePackets
			m.selectedIndex = 0
			m.scrollOffset = 0
		}
		return m, nil
	}
	
	return m, nil
}

func (m *Model) addEvent(event models.NetworkEvent) {
	m.events = append(m.events, event)
	
	// Keep only the last maxEvents
	if len(m.events) > maxEvents {
		m.events = m.events[len(m.events)-maxEvents:]
	}
}

func (m *Model) updateStats(event models.NetworkEvent) {
	m.stats.TotalPackets++
	m.stats.TotalBytes += event.Size
	m.stats.ProtocolCounts[event.Protocol]++
	m.stats.LastUpdate = time.Now()
}

func (m *Model) applyFilter() {
	m.filteredEvents = m.filteredEvents[:0]
	
	for _, event := range m.events {
		if m.matchesFilter(event) {
			m.filteredEvents = append(m.filteredEvents, event)
		}
	}
	
	// Adjust selection if needed
	if m.selectedIndex >= len(m.filteredEvents) {
		m.selectedIndex = len(m.filteredEvents) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
}

func (m *Model) matchesFilter(event models.NetworkEvent) bool {
	if m.filter.Protocol != "" && !strings.EqualFold(event.Protocol, m.filter.Protocol) {
		return false
	}
	
	if m.filter.IP != "" {
		if !strings.Contains(event.SourceIP, m.filter.IP) && !strings.Contains(event.DestIP, m.filter.IP) {
			return false
		}
	}
	
	if m.filter.Port != "" {
		portStr := fmt.Sprintf("%d", event.SourcePort)
		destPortStr := fmt.Sprintf("%d", event.DestPort)
		if portStr != m.filter.Port && destPortStr != m.filter.Port {
			return false
		}
	}
	
	return true
}

func (m *Model) clearEvents() {
	m.events = m.events[:0]
	m.filteredEvents = m.filteredEvents[:0]
	m.selectedIndex = 0
	m.scrollOffset = 0
	m.stats = Stats{
		ProtocolCounts: make(map[string]int),
		LastUpdate:     time.Now(),
	}
}

func (m *Model) scrollDown(lines int) {
	maxOffset := len(m.filteredEvents) - m.viewportHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	
	m.scrollOffset += lines
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}

func (m *Model) scrollUp(lines int) {
	m.scrollOffset -= lines
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *Model) ensureSelectedVisible() {
	viewHeight := m.viewportHeight()
	
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	} else if m.selectedIndex >= m.scrollOffset+viewHeight {
		m.scrollOffset = m.selectedIndex - viewHeight + 1
	}
}

func (m *Model) viewportHeight() int {
	// Account for header, stats, and footer
	return m.height - 8
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}
	
	if m.showHelp {
		return m.renderHelp()
	}
	
	var s strings.Builder
	
	s.WriteString(m.renderHeader())
	s.WriteString("\n")
	s.WriteString(m.renderStats())
	s.WriteString("\n")
	
	if m.viewMode == ViewModePackets {
		s.WriteString(m.renderEventList())
	} else {
		s.WriteString(m.renderConversationList())
	}
	
	s.WriteString("\n")
	s.WriteString(m.renderFooter())
	
	return s.String()
}

func (m *Model) renderHeader() string {
	title := " Netty Network Monitor "
	status := m.connectionStatus
	if status == "" {
		status = "Disconnected"
	}
	
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	
	if m.connected {
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	} else if strings.Contains(status, "Connecting") || strings.Contains(status, "Reconnecting") {
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	}
	
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Padding(0, 1).
		Render(title)
	
	statusText := statusStyle.Padding(0, 1).Render(status)
	
	// Truncate status if it's too long
	maxStatusWidth := m.width / 2
	if lipgloss.Width(statusText) > maxStatusWidth {
		status = status[:maxStatusWidth-5] + "..."
		statusText = statusStyle.Padding(0, 1).Render(status)
	}
	
	headerLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		header,
		lipgloss.NewStyle().Width(m.width - lipgloss.Width(header) - lipgloss.Width(statusText)).Render(""),
		statusText,
	)
	
	return lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("235")).
		Render(headerLine)
}

func (m *Model) renderStats() string {
	var stats string
	if m.viewMode == ViewModePackets {
		stats = fmt.Sprintf(
			" Packets: %d | Bytes: %s | Events: %d/%d",
			m.stats.TotalPackets,
			formatBytes(m.stats.TotalBytes),
			len(m.filteredEvents),
			len(m.events),
		)
	} else {
		activeCount := 0
		for _, conv := range m.conversations {
			if conv.IsActive() {
				activeCount++
			}
		}
		stats = fmt.Sprintf(
			" Conversations: %d active / %d total | Packets: %d | Bytes: %s",
			activeCount,
			len(m.conversations),
			m.stats.TotalPackets,
			formatBytes(m.stats.TotalBytes),
		)
	}
	
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Width(m.width).
		Padding(0, 1).
		Render(stats)
}

func (m *Model) renderEventList() string {
	viewHeight := m.viewportHeight()
	
	if len(m.filteredEvents) == 0 {
		message := "No network events captured yet"
		if !m.connected && m.connectionError != "" {
			message = fmt.Sprintf("Not connected to daemon\n\n%s\n\nMake sure the daemon is running:\nsudo ./netty-daemon -i en0", m.connectionError)
		} else if m.connected {
			message = "Waiting for network events...\n\nThe daemon is connected and monitoring traffic"
		}
		
		empty := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Align(lipgloss.Center).
			Width(m.width).
			Height(viewHeight).
			Render(message)
		return empty
	}
	
	var lines []string
	
	// Header row
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	header := fmt.Sprintf("%-8s %-25s %-6s %-25s %-6s %-8s %-8s",
		"Time", "Source", "Port", "Destination", "Port", "Protocol", "Size")
	lines = append(lines, headerStyle.Render(header))
	
	// Event rows
	endIdx := m.scrollOffset + viewHeight - 1
	if endIdx > len(m.filteredEvents) {
		endIdx = len(m.filteredEvents)
	}
	
	for i := m.scrollOffset; i < endIdx && i < len(m.filteredEvents); i++ {
		event := m.filteredEvents[i]
		line := m.renderEventLine(event, i == m.selectedIndex)
		lines = append(lines, line)
	}
	
	// Pad remaining space
	for len(lines) < viewHeight {
		lines = append(lines, "")
	}
	
	return strings.Join(lines, "\n")
}

func (m *Model) renderEventLine(event models.NetworkEvent, selected bool) string {
	timeStr := event.Timestamp.Format("15:04:05")
	
	// Use hostname if available, otherwise IP
	sourceDisplay := event.SourceIP
	if event.SourceHostname != "" && event.SourceHostname != event.SourceIP {
		sourceDisplay = event.SourceHostname
	}
	
	destDisplay := event.DestIP
	if event.DestHostname != "" && event.DestHostname != event.DestIP {
		destDisplay = event.DestHostname
	}
	
	// For HTTPS, prefer TLS SNI over hostname
	if event.TLSServerName != "" {
		destDisplay = event.TLSServerName
	}
	
	line := fmt.Sprintf("%-8s %-25s %-6d %-25s %-6d %-8s %-8s",
		timeStr,
		truncateString(sourceDisplay, 25),
		event.SourcePort,
		truncateString(destDisplay, 25),
		event.DestPort,
		event.Protocol,
		formatBytes(event.Size),
	)
	
	style := lipgloss.NewStyle()
	
	if selected {
		style = style.Background(lipgloss.Color("238")).Foreground(lipgloss.Color("255"))
	} else {
		// Color code by direction
		if event.Direction == "inbound" {
			style = style.Foreground(lipgloss.Color("45"))
		} else {
			style = style.Foreground(lipgloss.Color("213"))
		}
	}
	
	return style.Width(m.width).Render(line)
}

func (m *Model) renderFooter() string {
	var help string
	if m.viewMode == ViewModePackets {
		help = " q:quit | ?:help | j/k:navigate | c:clear | f:filter | tab:conversations "
	} else {
		help = " q:quit | ?:help | j/k:navigate | tab:packets "
	}
	
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(m.width).
		Align(lipgloss.Center).
		Background(lipgloss.Color("235")).
		Render(help)
}

func (m *Model) renderHelp() string {
	helpText := `
 Netty Network Monitor - Help
 
 Navigation:
   j/↓     Move down
   k/↑     Move up
   g       Go to top
   G       Go to bottom
   Ctrl+d  Page down
   Ctrl+u  Page up
 
 Actions:
   c       Clear all events
   f       Open filter dialog
   tab     Toggle between packets/conversations view
   ?/h     Toggle this help
   q       Quit
 
 Filters:
   You can filter events by protocol, IP address, or port.
   Use the 'f' key to open the filter dialog.
 
 Press any key to return...`
	
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(helpText)
}

func formatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// renderConversationList renders the list of active conversations
func (m *Model) renderConversationList() string {
	viewHeight := m.viewportHeight()
	
	if len(m.conversations) == 0 {
		message := "No active conversations"
		if !m.connected {
			message = "Not connected to daemon"
		}
		
		empty := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Align(lipgloss.Center).
			Width(m.width).
			Height(viewHeight).
			Render(message)
		return empty
	}
	
	var lines []string
	
	// Header row
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	header := fmt.Sprintf("%-40s %-15s %-8s %-10s %-10s %-8s",
		"Conversation", "Service", "State", "Packets", "Data", "Duration")
	lines = append(lines, headerStyle.Render(header))
	
	// Conversation rows
	endIdx := m.scrollOffset + viewHeight - 1
	if endIdx > len(m.conversations) {
		endIdx = len(m.conversations)
	}
	
	for i := m.scrollOffset; i < endIdx && i < len(m.conversations); i++ {
		conv := m.conversations[i]
		line := m.renderConversationLine(conv, i == m.selectedIndex)
		lines = append(lines, line)
	}
	
	// Pad remaining space
	for len(lines) < viewHeight {
		lines = append(lines, "")
	}
	
	return strings.Join(lines, "\n")
}

// renderConversationLine renders a single conversation line
func (m *Model) renderConversationLine(conv models.Conversation, selected bool) string {
	endpoints := conv.GetEndpointPair()
	if len(endpoints) > 40 {
		endpoints = endpoints[:37] + "..."
	}
	
	service := conv.GetServiceInfo()
	if len(service) > 15 {
		service = service[:12] + "..."
	}
	
	state := string(conv.State)
	if len(state) > 8 {
		state = state[:8]
	}
	
	packets := fmt.Sprintf("%d", conv.TotalPackets())
	data := formatBytes(int(conv.TotalBytes()))
	duration := conv.Duration
	
	line := fmt.Sprintf("%-40s %-15s %-8s %-10s %-10s %-8s",
		endpoints, service, state, packets, data, duration)
	
	style := lipgloss.NewStyle()
	
	if selected {
		style = style.Background(lipgloss.Color("238")).Foreground(lipgloss.Color("255"))
	} else {
		// Color by state
		switch conv.State {
		case models.ConversationStateEstablished:
			style = style.Foreground(lipgloss.Color("46")) // Green
		case models.ConversationStateNew:
			style = style.Foreground(lipgloss.Color("226")) // Yellow
		case models.ConversationStateClosing, models.ConversationStateClosed:
			style = style.Foreground(lipgloss.Color("245")) // Gray
		}
	}
	
	return style.Width(m.width).Render(line)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// requestConversations sends a request for conversation data
func (m *Model) requestConversations() tea.Cmd {
	return func() tea.Msg {
		// Send request to websocket
		if m.wsClient != nil {
			m.wsClient.RequestConversations()
		}
		return nil
	}
}