package api

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arrase/Raspiducky/pkg/gadget"
	"github.com/arrase/Raspiducky/pkg/hid"
	"github.com/gorilla/websocket"
)

func setupTestServer(t *testing.T, ledWatcher *hid.LEDWatcher) *Server {
	t.Helper()
	tmpDir := t.TempDir()

	udcDir := filepath.Join(tmpDir, "udc")
	if err := os.MkdirAll(udcDir, 0755); err != nil {
		t.Fatalf("mkdir udc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(udcDir, "dummy.udc"), []byte(""), 0644); err != nil {
		t.Fatalf("write dummy.udc: %v", err)
	}

	debugFSDir := filepath.Join(tmpDir, "debugfs")
	dummyDebugDir := filepath.Join(debugFSDir, "dummy.udc")
	if err := os.MkdirAll(dummyDebugDir, 0755); err != nil {
		t.Fatalf("mkdir debugfs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dummyDebugDir, "hw_params"), []byte("num_dev_ep: 16\n"), 0644); err != nil {
		t.Fatalf("write hw_params: %v", err)
	}

	gadgetBaseDir := filepath.Join(tmpDir, "usb_gadget")
	if err := os.MkdirAll(gadgetBaseDir, 0755); err != nil {
		t.Fatalf("mkdir usb_gadget: %v", err)
	}

	kbd, err := hid.NewKeyboard(filepath.Join(tmpDir, "hidg0"), "US")
	if err != nil {
		t.Fatalf("NewKeyboard: %v", err)
	}
	mouse := hid.NewMouse(filepath.Join(tmpDir, "hidg1"))

	server, err := NewServer(ServerOptions{
		StorageDir: filepath.Join(tmpDir, "storage"),
		Keyboard:   kbd,
		Mouse:      mouse,
		LEDWatcher: ledWatcher,
		GadgetOpts: []gadget.Option{
			gadget.WithBaseDir(gadgetBaseDir),
			gadget.WithUDCDir(udcDir),
			gadget.WithDebugFSDir(debugFSDir),
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize Server: %v", err)
	}
	return server
}

func TestServer_LEDStateBroadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ledWatcher := hid.NewLEDWatcher(ctx, "")
	server := setupTestServer(t, ledWatcher)

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

func TestGadgetManager_UpdateConfig_PreservesMaxEndpoints(t *testing.T) {
	server := setupTestServer(t, nil)

	status, err := server.gadgetManager.UpdateConfig(GadgetConfig{
		Keyboard:       true,
		Mouse:          false,
		VendorID:       "0x1d6b",
		ProductID:      "0x0104",
		Manufacturer:   "Raspiducky Labs",
		Product:        "Raspiducky HID",
		SerialNumber:   "RPD-2026-0001",
		KeyboardLayout: "US",
	})
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	if status.MaxEndpoints != 16 {
		t.Errorf("Expected MaxEndpoints=16 from UpdateConfig, got %d", status.MaxEndpoints)
	}
}

func TestScriptManager_SaveScript_NormalizesExtensionsAndSeedsDefaults(t *testing.T) {
	server := setupTestServer(t, nil)

	// Verify gadget_config.json was persisted on initial startup without preventing default scripts from seeding
	if _, err := os.Stat(filepath.Join(server.gadgetManager.storageDir, "gadget_config.json")); err != nil {
		t.Errorf("Expected gadget_config.json to be persisted on startup: %v", err)
	}

	initialScripts, err := server.scriptManager.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts failed: %v", err)
	}
	if len(initialScripts) < 3 {
		t.Errorf("Expected at least 3 default scripts seeded on fresh server startup, got %d (%+v)", len(initialScripts), initialScripts)
	}

	duckyName, err := server.scriptManager.SaveScript(Script{
		Name:    "my_payload",
		Type:    "duckyscript",
		Content: "DELAY 100\n",
	})
	if err != nil {
		t.Fatalf("SaveScript ducky failed: %v", err)
	}
	if duckyName != "my_payload.ducky" {
		t.Errorf("Expected normalized name 'my_payload.ducky', got %q", duckyName)
	}

	jsName, err := server.scriptManager.SaveScript(Script{
		Name:    "my_js",
		Type:    "javascript",
		Content: "delay(100);\n",
	})
	if err != nil {
		t.Fatalf("SaveScript js failed: %v", err)
	}
	if jsName != "my_js.js" {
		t.Errorf("Expected normalized name 'my_js.js', got %q", jsName)
	}

	if _, err := server.scriptManager.SaveScript(Script{
		Name:    ".ducky",
		Type:    "javascript",
		Content: "delay(100);\n",
	}); err == nil {
		t.Errorf("Expected error when saving script with empty stem '.ducky'")
	}

	scripts, err := server.scriptManager.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts failed: %v", err)
	}

	foundDucky := false
	foundJS := false
	for _, s := range scripts {
		if s.Name == "my_payload.ducky" && s.Type == "duckyscript" {
			foundDucky = true
		}
		if s.Name == "my_js.js" && s.Type == "javascript" {
			foundJS = true
		}
	}

	if !foundDucky {
		t.Errorf("Expected my_payload.ducky in ListScripts, got %+v", scripts)
	}
	if !foundJS {
		t.Errorf("Expected my_js.js in ListScripts, got %+v", scripts)
	}
}

func TestRunnerEngine_StopAndImmediateRestart_DoesNotOverwriteSecondJob(t *testing.T) {
	server := setupTestServer(t, nil)

	job1, err := server.runnerEngine.Run(RunRequest{
		Name:   "job1.ducky",
		Type:   "duckyscript",
		Script: "DELAY 2000\n",
	})
	if err != nil {
		t.Fatalf("Run job1 failed: %v", err)
	}

	if err := server.runnerEngine.Stop(); err != nil {
		t.Fatalf("Stop job1 failed: %v", err)
	}

	job2, err := server.runnerEngine.Run(RunRequest{
		Name:   "job2.ducky",
		Type:   "duckyscript",
		Script: "DELAY 500\n",
	})
	if err != nil {
		t.Fatalf("Run job2 failed: %v", err)
	}

	// Give job1's canceled goroutine time to exit while job2 is still running
	time.Sleep(100 * time.Millisecond)

	active := server.runnerEngine.GetActiveJob()
	if active == nil {
		t.Fatal("Expected active job, got nil")
	}
	if active.ID != job2.ID {
		t.Errorf("Expected active job ID %q (not overwritten by stopped job %q), got %q", job2.ID, job1.ID, active.ID)
	}
	if active.Status != "running" {
		t.Errorf("Expected job2 status 'running', got %q", active.Status)
	}

	_ = server.runnerEngine.Stop()
}

func TestGadgetManager_UpdateConfig_TypingDelayMs(t *testing.T) {
	server := setupTestServer(t, nil)

	status, err := server.gadgetManager.UpdateConfig(GadgetConfig{
		Keyboard:       true,
		Mouse:          false,
		VendorID:       "0x1d6b",
		ProductID:      "0x0104",
		Manufacturer:   "Raspiducky Labs",
		Product:        "Raspiducky HID",
		SerialNumber:   "RPD-2026-0001",
		KeyboardLayout: "US",
		TypingDelayMs:  35,
	})
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	if status.Config.TypingDelayMs != 35 {
		t.Errorf("Expected status.Config.TypingDelayMs=35, got %d", status.Config.TypingDelayMs)
	}

	delay, jitter := server.gadgetManager.keyboard.GetTypingSpeed()
	if delay != 35 || jitter != 0 {
		t.Errorf("Expected keyboard typing speed (35, 0), got (%d, %d)", delay, jitter)
	}
}

func TestRunnerEngine_Run_TypingDelayMs(t *testing.T) {
	server := setupTestServer(t, nil)

	delayMs := 45
	_, err := server.runnerEngine.Run(RunRequest{
		Name:          "typing_delay.ducky",
		Type:          "duckyscript",
		Script:        "DELAY 10\n",
		TypingDelayMs: &delayMs,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job := server.runnerEngine.GetActiveJob()
		if job != nil && job.Status == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	delay, jitter := server.runnerEngine.keyboard.GetTypingSpeed()
	if delay != 45 || jitter != 0 {
		t.Errorf("Expected keyboard typing speed (45, 0), got (%d, %d)", delay, jitter)
	}
}
