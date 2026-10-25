package api

import (
	"bytes"
	"context"
	"net/http"
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

	gm, err := gadget.NewMockGadgetManager(tmpDir, "16")
	if err != nil {
		t.Fatalf("NewMockGadgetManager: %v", err)
	}

	kbd, err := hid.NewKeyboard(filepath.Join(tmpDir, "hidg0"), "US")
	if err != nil {
		t.Fatalf("NewKeyboard: %v", err)
	}
	mouse := hid.NewMouse(filepath.Join(tmpDir, "hidg1"))

	server, err := NewServer(ServerOptions{
		StorageDir:    filepath.Join(tmpDir, "storage"),
		Keyboard:      kbd,
		Mouse:         mouse,
		LEDWatcher:    ledWatcher,
		GadgetManager: gm,
	})
	if err != nil {
		t.Fatalf("Failed to initialize Server: %v", err)
	}
	t.Cleanup(server.Close)
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

	// First initial message upon connection should be led_state
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial WS led_state message: %v", err)
	}
	if msg.Type != "led_state" {
		t.Fatalf("Expected first initial WS message type 'led_state', got %q", msg.Type)
	}

	// Second initial message upon connection should be gadget_status
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read initial WS gadget_status message: %v", err)
	}
	if msg.Type != "gadget_status" {
		t.Fatalf("Expected second initial WS message type 'gadget_status', got %q", msg.Type)
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
	// EP0(1) + Keyboard(1) = 2
	if status.UsedEndpoints != 2 {
		t.Errorf("Expected UsedEndpoints=2, got %d", status.UsedEndpoints)
	}
}

func TestScriptManager_SaveScript_NormalizesExtensionsAndSeedsDefaults(t *testing.T) {
	server := setupTestServer(t, nil)

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
		Name:        "my_payload..v1",
		Type:        "duckyscript",
		Content:     "DELAY 100\n",
		Description: "Custom Ducky Description",
	})
	if err != nil {
		t.Fatalf("SaveScript ducky failed: %v", err)
	}
	if duckyName != "my_payload..v1.ducky" {
		t.Errorf("Expected normalized name 'my_payload..v1.ducky', got %q", duckyName)
	}

	jsName, err := server.scriptManager.SaveScript(Script{
		Name:    "my_js",
		Type:    "javascript",
		Content: "// Custom JS Comment\ndelay(100);\n",
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
		if s.Name == "my_payload..v1.ducky" && s.Type == "duckyscript" && s.Description == "Custom Ducky Description" {
			foundDucky = true
		}
		if s.Name == "my_js.js" && s.Type == "javascript" && s.Description == "Custom JS Comment" {
			foundJS = true
		}
	}

	if !foundDucky {
		t.Errorf("Expected my_payload..v1.ducky with custom description in ListScripts, got %+v", scripts)
	}
	if !foundJS {
		t.Errorf("Expected my_js.js with comment description in ListScripts, got %+v", scripts)
	}

	// Verify DELETE status codes: 404 for missing script, 400 for invalid name
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	req404, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/scripts/nonexistent.ducky", nil)
	resp404, err := http.DefaultClient.Do(req404)
	if err != nil {
		t.Fatalf("DELETE nonexistent failed: %v", err)
	}
	_ = resp404.Body.Close()
	if resp404.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found for missing script, got %d", resp404.StatusCode)
	}

	req400, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/scripts/..", nil)
	resp400, err := http.DefaultClient.Do(req400)
	if err != nil {
		t.Fatalf("DELETE '..' failed: %v", err)
	}
	_ = resp400.Body.Close()
	if resp400.StatusCode == http.StatusNoContent {
		t.Errorf("Expected error status for invalid script name '..', got 204")
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

func TestGadgetManager_UpdateConfig_TypingDelayMsAndCLIFlags(t *testing.T) {
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

	// Verify reloading from existing gadget_config.json without typingDelayMs preserves default 10ms
	legacyJSON := `{"keyboard":true,"mouse":false,"vendorId":"0x1d6b","productId":"0x0104","manufacturer":"Raspiducky","product":"HID","serialNumber":"RPD-1","keyboardLayout":"US"}`
	if err := os.WriteFile(filepath.Join(server.gadgetManager.storageDir, "gadget_config.json"), []byte(legacyJSON), 0644); err != nil {
		t.Fatalf("WriteFile legacy gadget_config.json: %v", err)
	}
	reloadedGM := NewGadgetManager(server.hub, server.runnerEngine, ServerOptions{
		StorageDir:    server.gadgetManager.storageDir,
		Keyboard:      server.gadgetManager.keyboard,
		Mouse:         server.gadgetManager.mouse,
		GadgetManager: server.gadgetManager.GadgetManager,
	})
	if reloadedGM.currentStatus.Config.TypingDelayMs != 10 {
		t.Errorf("Expected default TypingDelayMs=10 when missing from saved config, got %d", reloadedGM.currentStatus.Config.TypingDelayMs)
	}

	// Verify CLI flags (-layout ES -delay 55) passed via ServerOptions override saved config
	cliGM := NewGadgetManager(server.hub, server.runnerEngine, ServerOptions{
		StorageDir:     server.gadgetManager.storageDir,
		Keyboard:       server.gadgetManager.keyboard,
		Mouse:          server.gadgetManager.mouse,
		KeyboardLayout: "ES",
		TypingDelayMs:  55,
		GadgetManager:  server.gadgetManager.GadgetManager,
	})
	if cliGM.currentStatus.Config.KeyboardLayout != "ES" || cliGM.currentStatus.Config.TypingDelayMs != 55 {
		t.Errorf("Expected CLI flags (ES, 55) to be honored, got (%q, %d)", cliGM.currentStatus.Config.KeyboardLayout, cliGM.currentStatus.Config.TypingDelayMs)
	}
	if server.gadgetManager.keyboard.GetLayout() != "ES" {
		t.Errorf("Expected keyboard layout 'ES', got %q", server.gadgetManager.keyboard.GetLayout())
	}
}

func TestRunnerEngine_Run_TypingDelayMsAndStateRestoration(t *testing.T) {
	server := setupTestServer(t, nil)
	var hidBuf bytes.Buffer
	server.runnerEngine.keyboard.SetWriter(&hidBuf)

	_, err := server.runnerEngine.Run(RunRequest{
		Name:          "typing_delay.ducky",
		Type:          "duckyscript",
		Script:        "LAYOUT ES\nTYPING_SPEED 100 50\nDELAY 10\n",
		TypingDelayMs: 45,
		Layout:        "DE",
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

	// Verify base typing speed (10, 0) and base layout ("US") were restored after RunRequest + in-script mutations
	delay, jitter := server.runnerEngine.keyboard.GetTypingSpeed()
	if delay != 10 || jitter != 0 {
		t.Errorf("Expected base keyboard typing speed (10, 0) restored after script, got (%d, %d)", delay, jitter)
	}
	if gotLayout := server.runnerEngine.keyboard.GetLayout(); gotLayout != "US" {
		t.Errorf("Expected base keyboard layout 'US' restored after script, got %q", gotLayout)
	}

	// Verify concurrent UpdateConfig while a job is running stops the job and applies the new config without race
	_, err = server.runnerEngine.Run(RunRequest{
		Name:          "long_running.ducky",
		Type:          "duckyscript",
		Script:        "DELAY 5000\n",
		TypingDelayMs: 99,
		Layout:        "FR",
	})
	if err != nil {
		t.Fatalf("Run long_running failed: %v", err)
	}

	_, err = server.gadgetManager.UpdateConfig(GadgetConfig{
		Keyboard:       true,
		Mouse:          true,
		VendorID:       "0x1d6b",
		ProductID:      "0x0104",
		Manufacturer:   "Raspiducky",
		Product:        "HID",
		SerialNumber:   "RPD-1",
		KeyboardLayout: "ES",
		TypingDelayMs:  60,
	})
	if err != nil {
		t.Fatalf("Concurrent UpdateConfig failed: %v", err)
	}

	delay, jitter = server.runnerEngine.keyboard.GetTypingSpeed()
	if delay != 60 || jitter != 0 {
		t.Errorf("Expected UpdateConfig typing speed (60, 0) not overwritten by stopped job, got (%d, %d)", delay, jitter)
	}
	if gotLayout := server.runnerEngine.keyboard.GetLayout(); gotLayout != "ES" {
		t.Errorf("Expected UpdateConfig layout 'ES' not overwritten by stopped job, got %q", gotLayout)
	}
}

func TestGadgetManager_UpdateConfig_ValidationAndHIDDevicePaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ledWatcher := hid.NewLEDWatcher(ctx, "")
	server := setupTestServer(t, ledWatcher)

	// Invalid VID must fail before mutating keyboard layout or creating disk.img
	_, err := server.gadgetManager.UpdateConfig(GadgetConfig{
		Keyboard:       true,
		Storage:        true,
		StorageSizeMB:  10,
		VendorID:       "invalid",
		ProductID:      "0x0104",
		Manufacturer:   "Raspiducky",
		Product:        "HID",
		SerialNumber:   "RPD-1",
		KeyboardLayout: "DE",
	})
	if err == nil {
		t.Fatal("Expected error for invalid VendorID")
	}
	if server.gadgetManager.keyboard.GetLayout() != "US" {
		t.Errorf("Expected keyboard layout to remain 'US' on failed UpdateConfig, got %q", server.gadgetManager.keyboard.GetLayout())
	}
	if _, statErr := os.Stat(filepath.Join(server.gadgetManager.storageDir, "disk.img")); !os.IsNotExist(statErr) {
		t.Errorf("Expected disk.img not to be created when validation fails")
	}

	// Disabling Keyboard while Mouse is enabled must unbind keyboard/ledWatcher from hidg0 and assign hidg0 to mouse
	status, err := server.gadgetManager.UpdateConfig(GadgetConfig{
		Keyboard:       false,
		Mouse:          true,
		RNDIS:          true,
		ECM:            false,
		VendorID:       "0x1d6b",
		ProductID:      "0x0104",
		Manufacturer:   "Raspiducky",
		Product:        "HID",
		SerialNumber:   "RPD-1",
		KeyboardLayout: "US",
	})
	if err != nil {
		t.Fatalf("UpdateConfig (Mouse + RNDIS) failed: %v", err)
	}
	if status.Config.StorageSizeMB != 100 {
		t.Errorf("Expected StorageSizeMB to default to 100 even when Storage is false, got %d", status.Config.StorageSizeMB)
	}
	if server.gadgetManager.keyboard.DevicePath() != "" {
		t.Errorf("Expected keyboard DevicePath to be empty when Keyboard is disabled, got %q", server.gadgetManager.keyboard.DevicePath())
	}
	if ledWatcher.DevicePath() != "" {
		t.Errorf("Expected ledWatcher DevicePath to be empty when Keyboard is disabled, got %q", ledWatcher.DevicePath())
	}
	if !strings.HasSuffix(server.gadgetManager.mouse.DevicePath(), "hidg0") {
		t.Errorf("Expected mouse DevicePath to end with 'hidg0' when Keyboard is disabled, got %q", server.gadgetManager.mouse.DevicePath())
	}

	// Re-enabling Keyboard must point both keyboard and ledWatcher back to hidg0
	_, err = server.gadgetManager.UpdateConfig(GadgetConfig{
		Keyboard:       true,
		Mouse:          true,
		VendorID:       "0x1d6b",
		ProductID:      "0x0104",
		Manufacturer:   "Raspiducky",
		Product:        "HID",
		SerialNumber:   "RPD-1",
		KeyboardLayout: "US",
	})
	if err != nil {
		t.Fatalf("UpdateConfig (re-enable Keyboard) failed: %v", err)
	}
	if !strings.HasSuffix(ledWatcher.DevicePath(), "hidg0") {
		t.Errorf("Expected ledWatcher DevicePath to end with 'hidg0' when Keyboard is re-enabled, got %q", ledWatcher.DevicePath())
	}

	// Verify existing non-empty disk.img is preserved by ensureBackingFile even if requested sizeMB differs
	diskPath := filepath.Join(server.gadgetManager.storageDir, "disk.img")
	sentinel := []byte("USER_FILESYSTEM_DATA")
	if err := os.WriteFile(diskPath, sentinel, 0644); err != nil {
		t.Fatalf("WriteFile sentinel disk.img: %v", err)
	}
	if err := ensureBackingFile(diskPath, 50); err != nil {
		t.Fatalf("ensureBackingFile on existing non-empty disk.img failed: %v", err)
	}
	gotDisk, err := os.ReadFile(diskPath)
	if err != nil || string(gotDisk) != string(sentinel) {
		t.Errorf("Expected existing disk.img content to be preserved, got %q (err=%v)", string(gotDisk), err)
	}
}
