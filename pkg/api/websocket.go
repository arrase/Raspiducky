package api

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from web dashboard
	},
}

// Hub maintains the set of active WebSocket connections and broadcasts messages.
type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan WSMessage
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan WSMessage, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Run starts the event loop for broadcasting messages to WebSocket clients.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WebSocket] Client connected (%s)", client.RemoteAddr())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				log.Printf("[WebSocket] Client disconnected (%s)", client.RemoteAddr())
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			var deadClients []*websocket.Conn
			for client := range h.clients {
				if err := client.WriteJSON(msg); err != nil {
					log.Printf("[WebSocket] Error writing to client %s: %v", client.RemoteAddr(), err)
					client.Close()
					deadClients = append(deadClients, client)
				}
			}
			h.mu.RUnlock()
			if len(deadClients) > 0 {
				h.mu.Lock()
				for _, c := range deadClients {
					delete(h.clients, c)
				}
				h.mu.Unlock()
			}
		}
	}
}

// Broadcast dispatches a message to all connected WebSocket clients.
func (h *Hub) Broadcast(msg WSMessage) {
	select {
	case h.broadcast <- msg:
	default:
		log.Printf("[WebSocket] Broadcast channel full, dropping message")
	}
}

// HandleWS handles WebSocket handshake and upgrades HTTP connection.
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}

	h.register <- conn

	// Keep-alive / read loop
	go func() {
		defer func() {
			h.unregister <- conn
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}
