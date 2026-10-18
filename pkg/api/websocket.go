package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const wsWriteTimeout = 5 * time.Second

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
			h.clients[client] = true
			log.Printf("[WebSocket] Client connected (%s)", client.RemoteAddr())

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				log.Printf("[WebSocket] Client disconnected (%s)", client.RemoteAddr())
			}

		case msg := <-h.broadcast:
			for client := range h.clients {
				_ = client.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
				if err := client.WriteJSON(msg); err != nil {
					log.Printf("[WebSocket] Error writing to client %s: %v", client.RemoteAddr(), err)
					client.Close()
					delete(h.clients, client)
				}
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

// Upgrade upgrades an HTTP connection to WebSocket.
func (h *Hub) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return nil, err
	}
	return conn, nil
}

// Register registers an upgraded WebSocket connection with the Hub and starts its read loop.
func (h *Hub) Register(conn *websocket.Conn) {
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
