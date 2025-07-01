package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
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
}

type Client struct {
	conn   *websocket.Conn
	send   chan []byte
	server *Server
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

func (s *Server) Start() error {
	go s.run()

	http.HandleFunc("/ws", s.handleWebSocket)
	http.HandleFunc("/health", s.handleHealth)

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
				close(client.send)
				s.mu.Unlock()
				log.Printf("Client disconnected. Total clients: %d", len(s.clients))
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
				select {
				case client.send <- message:
				default:
					// Client's send channel is full, close it
					s.mu.Lock()
					delete(s.clients, client)
					s.mu.Unlock()
					close(client.send)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) Broadcast(event *models.NetworkEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	select {
	case s.broadcast <- data:
	default:
		log.Println("Broadcast channel full, dropping event")
	}
}

func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	for {
		// Read message from client (for ping/pong and potential future commands)
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}