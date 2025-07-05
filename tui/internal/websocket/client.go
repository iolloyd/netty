package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"github.com/netty/tui/internal/models"
)

type Client struct {
	conn     *websocket.Conn
	url      string
	messages chan interface{}
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
		url:      u.String(),
		messages: make(chan interface{}, 100),
	}
}

func (c *Client) Connect() tea.Cmd {
	return func() tea.Msg {
		conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
		if err != nil {
			return ConnectionStatusMsg{Connected: false, Error: err}
		}
		c.conn = conn
		
		go c.readMessages()
		
		return ConnectionStatusMsg{Connected: true, Error: nil}
	}
}

func (c *Client) readMessages() {
	defer c.Close()
	
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
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
				log.Printf("JSON unmarshal error: %v", err)
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

func (c *Client) WaitForEvent() tea.Cmd {
	return func() tea.Msg {
		msg := <-c.messages
		
		switch m := msg.(type) {
		case models.NetworkEvent:
			return EventMsg(m)
		case ConversationsMsg:
			return m
		default:
			return nil
		}
	}
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) SendCommand(cmd interface{}) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) Reconnect() tea.Cmd {
	return tea.Sequence(
		tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return nil
		}),
		c.Connect(),
	)
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