package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
	"github.com/gorilla/websocket"
)

func TestServer_LEDStateBroadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ledWatcher, err := hid.NewLEDWatcher(ctx, "")
	if err != nil {
		t.Fatalf("Failed to initialize LEDWatcher: %v", err)
	}

	server, err := NewServer(ServerOptions{
		StorageDir: t.TempDir(),
		LEDWatcher: ledWatcher,
	})
	if err != nil {
		t.Fatalf("Failed to initialize Server: %v", err)
	}

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Initial message upon connection should be initial led_state
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial WS message: %v", err)
	}

	if msg.Type != "led_state" {
		t.Fatalf("Expected initial WS message type 'led_state', got %q", msg.Type)
	}

	// Now update LED state (simulate host pressing CapsLock)
	ledWatcher.UpdateState(hid.LEDCapsLock)

	// Read broadcasted WS message
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read broadcasted WS message: %v", err)
	}

	if msg.Type != "led_state" {
		t.Fatalf("Expected broadcast WS message type 'led_state', got %q", msg.Type)
	}

	payloadMap, ok := msg.Payload.(map[string]any)
	if !ok {
		t.Fatalf("Expected payload map, got %T", msg.Payload)
	}

	if caps, ok := payloadMap["capsLock"].(bool); !ok || !caps {
		t.Errorf("Expected capsLock to be true in WS payload, got %v", payloadMap["capsLock"])
	}
}
