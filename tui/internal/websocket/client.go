package websocket

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"github.com/netty/tui/internal/models"
)

type Client struct {
	conn         *websocket.Conn
	url          string
	messages     chan interface{}
	mu           sync.Mutex
	isConnected  bool
	statusUpdate chan ConnectionStatusMsg
	stopRead     chan struct{}
}

type EventMsg models.NetworkEvent
type ConnectionStatusMsg struct {
	Connected bool
	Error     error
}
type ConversationsMsg []models.Conversation

func NewClient(host string, port int) *Client {
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", host, port), Path: "/ws"}
	return &Client{
		url:          u.String(),
		messages:     make(chan interface{}, 100),
		statusUpdate: make(chan ConnectionStatusMsg, 10),
		stopRead:     make(chan struct{}),
	}
}

func (c *Client) Connect() tea.Cmd {
	return func() tea.Msg {
		c.mu.Lock()
		defer c.mu.Unlock()
		
		// Close existing connection if any
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		
		// Stop any existing read goroutine
		select {
		case c.stopRead <- struct{}{}:
		default:
		}
		
		conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
		if err != nil {
			c.isConnected = false
			return ConnectionStatusMsg{Connected: false, Error: err}
		}
		c.conn = conn
		c.isConnected = true
		
		go c.readMessages()
		
		return ConnectionStatusMsg{Connected: true, Error: nil}
	}
}

func (c *Client) readMessages() {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.isConnected = false
		c.mu.Unlock()
		
		// Send disconnection status
		select {
		case c.statusUpdate <- ConnectionStatusMsg{Connected: false, Error: fmt.Errorf("connection lost")}:
		default:
		}
	}()
	
	for {
		select {
		case <-c.stopRead:
			return
		default:
			// Set read deadline to allow periodic checks
			c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				// Check if it's a timeout (which is expected)
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					// Real error, not a timeout
					return
				}
				if e, ok := err.(*websocket.CloseError); ok && e.Code != websocket.CloseNormalClosure {
					// Abnormal close
					return
				}
				// Timeout or normal close, continue
				continue
			}
		
			// Try to parse as a typed message first
			var typedMsg struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			}
			
			if err := json.Unmarshal(message, &typedMsg); err == nil && typedMsg.Type != "" {
				// Handle typed messages
				switch typedMsg.Type {
				case "network_event":
					var event models.NetworkEvent
					if err := json.Unmarshal(typedMsg.Data, &event); err == nil {
						select {
						case c.messages <- event:
						default:
						}
					}
				case "conversations", "conversation_summaries":
					var conversations []models.Conversation
					if err := json.Unmarshal(typedMsg.Data, &conversations); err == nil {
						select {
						case c.messages <- ConversationsMsg(conversations):
						default:
						}
					}
				case "conversation", "conversation_update":
					var conversation models.Conversation
					if err := json.Unmarshal(typedMsg.Data, &conversation); err == nil {
						// For now, we'll just request a full update
						// In the future, we could handle individual updates
						c.RequestConversations()
					}
				}
			} else {
				// Try to parse as network event (backward compatibility)
				var event models.NetworkEvent
				if err := json.Unmarshal(message, &event); err != nil {
					// Silently skip malformed messages
					continue
				}
				
				select {
				case c.messages <- event:
				default:
					// Drop message if channel is full
				}
			}
		}
	}
}

func (c *Client) WaitForEvent() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-c.messages:
			switch m := msg.(type) {
			case models.NetworkEvent:
				return EventMsg(m)
			case ConversationsMsg:
				return m
			default:
				return nil
			}
		case status := <-c.statusUpdate:
			return status
		case <-time.After(100 * time.Millisecond):
			// Return nil to allow the UI to continue processing
			return nil
		}
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Signal read goroutine to stop
	select {
	case c.stopRead <- struct{}{}:
	default:
	}
	
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.isConnected = false
		return err
	}
	return nil
}

func (c *Client) SendCommand(cmd interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn == nil || !c.isConnected {
		return fmt.Errorf("not connected")
	}
	
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	
	// Set write deadline
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	err = c.conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		// Connection error, mark as disconnected
		c.isConnected = false
		return err
	}
	
	return nil
}

func (c *Client) Reconnect() tea.Cmd {
	return c.Connect()
}

// RequestConversations sends a request for conversation data
func (c *Client) RequestConversations() error {
	cmd := struct {
		Type string `json:"type"`
	}{
		Type: "get_conversation_summaries",
	}
	return c.SendCommand(cmd)
}

// IsConnected returns the current connection status
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isConnected
}