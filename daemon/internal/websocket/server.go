package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/iolloyd/netty/daemon/internal/conversation"
	"github.com/iolloyd/netty/daemon/internal/models"
)

type Server struct {
	port      string
	clients   map[*Client]bool
	broadcast chan []byte
	register  chan *Client
	unregister chan *Client
	upgrader  websocket.Upgrader
	mu        sync.RWMutex
	convMgr   *conversation.Manager
	statsFunc func() map[string]interface{} // Function to get capture statistics
}

type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	server *Server
	mu     sync.Mutex
	closed bool
}

func NewServer(port string) *Server {
	return &Server{
		port:       port,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from any origin for development
				// TODO: Restrict this in production
				return true
			},
		},
	}
}

// getClientCount returns the current number of connected clients
func (s *Server) getClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// SetConversationManager sets the conversation manager for the server
func (s *Server) SetConversationManager(mgr *conversation.Manager) {
	s.convMgr = mgr
}

// SetStatsFunction sets the function to retrieve capture statistics
func (s *Server) SetStatsFunction(fn func() map[string]interface{}) {
	s.statsFunc = fn
}

func (s *Server) Start() error {
	go s.run()

	http.HandleFunc("/ws", s.handleWebSocket)
	http.HandleFunc("/health", s.handleHealth)
	http.HandleFunc("/api/conversations", s.handleConversations)
	http.HandleFunc("/api/conversations/summary", s.handleConversationSummary)

	log.Printf("WebSocket server starting on port %s", s.port)
	return http.ListenAndServe(":"+s.port, nil)
}

func (s *Server) run() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = true
			s.mu.Unlock()
			log.Printf("Client connected. Total clients: %d", len(s.clients))

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				s.mu.Unlock()
				
				// Close the client's send channel safely
				client.mu.Lock()
				if !client.closed {
					client.closed = true
					close(client.send)
				}
				client.mu.Unlock()
				
				log.Printf("Client disconnected. Total clients: %d", s.getClientCount())
			} else {
				s.mu.Unlock()
			}

		case message := <-s.broadcast:
			s.mu.RLock()
			clientsCopy := make([]*Client, 0, len(s.clients))
			for client := range s.clients {
				clientsCopy = append(clientsCopy, client)
			}
			s.mu.RUnlock()

			for _, client := range clientsCopy {
				// Use safeSend to avoid panic
				if !client.safeSend(message) {
					// Client's send channel is full or closed, unregister it
					s.unregister <- client
				}
			}
		}
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		conn:   conn,
		send:   make(chan []byte, 256),
		server: s,
	}

	s.register <- client

	go client.writePump()
	go client.readPump()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientCount := len(s.clients)
	s.mu.RUnlock()

	response := map[string]interface{}{
		"status":  "healthy",
		"clients": clientCount,
	}

	// Add capture statistics if available
	if s.statsFunc != nil {
		response["capture_stats"] = s.statsFunc()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for development
	json.NewEncoder(w).Encode(response)
}

func (s *Server) Broadcast(event *models.NetworkEvent) {
	// Debug log
	// Event broadcast is handled silently
	
	// Wrap event in a message type
	message := struct {
		Type string               `json:"type"`
		Data *models.NetworkEvent `json:"data"`
	}{
		Type: "network_event",
		Data: event,
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	select {
	case s.broadcast <- data:
		// Event queued successfully
	default:
		log.Println("Broadcast channel full, dropping event")
	}
}

// BroadcastConversationUpdate sends conversation updates to all clients
func (s *Server) BroadcastConversationUpdate(conversationID string) {
	if s.convMgr == nil {
		return
	}

	conv, exists := s.convMgr.GetConversation(conversationID)
	if !exists {
		return
	}

	message := struct {
		Type string                  `json:"type"`
		Data *models.Conversation    `json:"data"`
	}{
		Type: "conversation_update",
		Data: conv,
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal conversation update: %v", err)
		return
	}

	select {
	case s.broadcast <- data:
	default:
		log.Println("Broadcast channel full, dropping conversation update")
	}
}

func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	for {
		// Read message from client (for ping/pong and potential future commands)
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		
		// Handle client commands
		c.handleCommand(message)
	}
}

// safeSend safely sends data to the client, checking if the channel is closed
func (c *Client) safeSend(data []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return false
	}
	
	select {
	case c.send <- data:
		return true
	default:
		// Channel is full
		return false
	}
}

// handleCommand processes commands from clients
func (c *Client) handleCommand(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			// Silently handle panic
		}
	}()
	
	var cmd struct {
		Type string `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	
	if err := json.Unmarshal(message, &cmd); err != nil {
		return // Ignore malformed messages
	}
	
	switch cmd.Type {
	case "get_conversations":
		// Send active conversations to this client
		if c.server.convMgr != nil {
			conversations := c.server.convMgr.GetActiveConversations()
			response := struct {
				Type string `json:"type"`
				Data interface{} `json:"data"`
			}{
				Type: "conversations",
				Data: conversations,
			}
			
			if data, err := json.Marshal(response); err == nil {
				c.safeSend(data)
			}
		}
	
	case "get_conversation_summaries":
		// Send conversation summaries to this client
		if c.server.convMgr != nil {
			summaries := c.server.convMgr.GetConversationSummaries()
			response := struct {
				Type string `json:"type"`
				Data interface{} `json:"data"`
			}{
				Type: "conversation_summaries",
				Data: summaries,
			}
			
			if data, err := json.Marshal(response); err == nil {
				c.safeSend(data)
			}
		}
	
	case "get_conversation":
		// Get specific conversation by ID
		var params struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(cmd.Data, &params); err == nil && c.server.convMgr != nil {
			if conv, exists := c.server.convMgr.GetConversation(params.ID); exists {
				response := struct {
					Type string `json:"type"`
					Data interface{} `json:"data"`
				}{
					Type: "conversation",
					Data: conv,
				}
				
				if data, err := json.Marshal(response); err == nil {
					c.safeSend(data)
				}
			}
		}
	}
}

func (c *Client) writePump() {
	defer func() {
		if r := recover(); r != nil {
			// Silently handle panic
		}
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				// Write error handled silently
				return
			}
		}
	}
}

// handleConversations handles HTTP API requests for conversations
func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	if s.convMgr == nil {
		http.Error(w, "Conversation manager not initialized", http.StatusInternalServerError)
		return
	}
	
	conversations := s.convMgr.GetActiveConversations()
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for development
	json.NewEncoder(w).Encode(conversations)
}

// handleConversationSummary handles HTTP API requests for conversation summary
func (s *Server) handleConversationSummary(w http.ResponseWriter, r *http.Request) {
	if s.convMgr == nil {
		http.Error(w, "Conversation manager not initialized", http.StatusInternalServerError)
		return
	}
	
	summaries := s.convMgr.GetConversationSummaries()
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // CORS for development
	json.NewEncoder(w).Encode(summaries)
}