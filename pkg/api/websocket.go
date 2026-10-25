package api

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsWriteTimeout  = 5 * time.Second
	wsClientBufSize = 64
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from web dashboard
	},
}

type wsClient struct {
	conn   *websocket.Conn
	send   chan WSMessage
	closed bool
}

// Hub maintains the set of active WebSocket connections and broadcasts messages.
type Hub struct {
	clients    map[*wsClient]struct{}
	broadcast  chan WSMessage
	register   chan *wsClient
	unregister chan *wsClient
	stopCh     chan struct{}
	closeOnce  sync.Once
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*wsClient]struct{}),
		broadcast:  make(chan WSMessage, 256),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
		stopCh:     make(chan struct{}),
	}
}

func (h *Hub) removeClient(client *wsClient) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)
	if !client.closed {
		client.closed = true
		close(client.send)
	}
	_ = client.conn.Close()
}

// Run starts the event loop for broadcasting messages to WebSocket clients.
func (h *Hub) Run() {
	for {
		select {
		case <-h.stopCh:
			for client := range h.clients {
				h.removeClient(client)
			}
			return

		case client := <-h.register:
			h.clients[client] = struct{}{}
			log.Printf("[WebSocket] Client connected (%s)", client.conn.RemoteAddr())

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				h.removeClient(client)
				log.Printf("[WebSocket] Client disconnected (%s)", client.conn.RemoteAddr())
			}

		case msg := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					log.Printf("[WebSocket] Client %s send buffer full, disconnecting", client.conn.RemoteAddr())
					h.removeClient(client)
				}
			}
		}
	}
}

// Close stops the hub and disconnects all active WebSocket clients.
func (h *Hub) Close() {
	h.closeOnce.Do(func() {
		close(h.stopCh)
	})
}

// Broadcast dispatches a message to all connected WebSocket clients.
func (h *Hub) Broadcast(msg WSMessage) {
	select {
	case <-h.stopCh:
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

// Register registers an upgraded WebSocket connection with the Hub and starts its read/write loops.
func (h *Hub) Register(conn *websocket.Conn) {
	client := &wsClient{
		conn: conn,
		send: make(chan WSMessage, wsClientBufSize),
	}

	select {
	case <-h.stopCh:
		_ = conn.Close()
		return
	case h.register <- client:
	}

	go func() {
		defer func() {
			_ = conn.Close()
		}()
		for msg := range client.send {
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("[WebSocket] Error writing to client %s: %v", conn.RemoteAddr(), err)
				return
			}
		}
	}()

	go func() {
		defer func() {
			select {
			case h.unregister <- client:
			case <-h.stopCh:
			}
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}
