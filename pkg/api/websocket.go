package api

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsWriteTimeout  = 5 * time.Second
	wsPongWait      = 60 * time.Second
	wsPingPeriod    = 45 * time.Second
	wsMaxMessageLen = 64 * 1024
	wsClientBufSize = 64
)

func isSameOriginRequest(r *http.Request) bool {
	if secSite := r.Header.Get("Sec-Fetch-Site"); secSite != "" && secSite != "same-origin" && secSite != "none" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: isSameOriginRequest,
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
	wg         sync.WaitGroup
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

// Start launches the Hub event loop in a tracked background goroutine.
func (h *Hub) Start() {
	h.wg.Add(1)
	go h.run()
}

func (h *Hub) run() {
	defer h.wg.Done()

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
	h.wg.Wait()
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

func writePump(client *wsClient) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = client.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.send:
			if !ok {
				return
			}
			_ = client.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := client.conn.WriteJSON(msg); err != nil {
				log.Printf("[WebSocket] Error writing to client %s: %v", client.conn.RemoteAddr(), err)
				return
			}
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) readPump(client *wsClient) {
	defer func() {
		select {
		case h.unregister <- client:
		case <-h.stopCh:
		}
	}()

	client.conn.SetReadLimit(wsMaxMessageLen)
	_ = client.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	client.conn.SetPongHandler(func(string) error {
		_ = client.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			break
		}
	}
}

// Register registers an upgraded WebSocket connection with initial snapshot messages queued before live broadcasts.
func (h *Hub) Register(conn *websocket.Conn, initial ...WSMessage) {
	client := &wsClient{
		conn: conn,
		send: make(chan WSMessage, wsClientBufSize),
	}
	for _, msg := range initial {
		select {
		case client.send <- msg:
		default:
		}
	}

	select {
	case <-h.stopCh:
		_ = conn.Close()
		return
	case h.register <- client:
	}

	go writePump(client)
	go h.readPump(client)
}
