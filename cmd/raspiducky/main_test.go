package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/arrase/Raspiducky/pkg/api"
	"github.com/arrase/Raspiducky/pkg/gadget"
	"github.com/arrase/Raspiducky/pkg/hid"
)

func TestInitHIDDevicesAndRunScriptCLI(t *testing.T) {
	printUsage()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kbd, mouse, ledWatcher, err := initHIDDevices(ctx, "", -1)
	if err != nil {
		t.Fatalf("initHIDDevices default failed: %v", err)
	}
	defer func() { _ = kbd.Close() }()
	defer func() { _ = mouse.Close() }()
	defer func() { _ = ledWatcher.Close() }()

	if kbd.GetLayout() != "US" {
		t.Errorf("Expected default layout US, got %s", kbd.GetLayout())
	}
	delay, jitter := kbd.GetTypingSpeed()
	if delay != 10 || jitter != 0 {
		t.Errorf("Expected default typing speed (10, 0), got (%d, %d)", delay, jitter)
	}

	tmpDir := t.TempDir()
	tmpScript := filepath.Join(tmpDir, "test.js")
	if err := os.WriteFile(tmpScript, []byte("delay(1);\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Local execution fallback when daemon is disabled
	if err := runScriptCLI([]string{tmpScript, "-layout", "US", "-delay", "0", "-server", ""}); err != nil {
		t.Fatalf("runScriptCLI fallback failed: %v", err)
	}

	// Daemon delegation when daemon API is reachable
	gm, err := gadget.NewMockGadgetManager(tmpDir, "16")
	if err != nil {
		t.Fatalf("NewMockGadgetManager: %v", err)
	}
	daemonKbd, err := hid.NewKeyboard(filepath.Join(tmpDir, "hidg0"), "US")
	if err != nil {
		t.Fatalf("NewKeyboard: %v", err)
	}
	var hidOut bytes.Buffer
	daemonKbd.SetWriter(&hidOut)
	daemonMouse := hid.NewMouse(filepath.Join(tmpDir, "hidg1"))

	initialDelay := 50
	server, err := api.NewServer(api.ServerOptions{
		StorageDir:     filepath.Join(tmpDir, "storage"),
		Keyboard:       daemonKbd,
		Mouse:          daemonMouse,
		KeyboardLayout: "ES",
		TypingDelayMs:  &initialDelay,
		GadgetManager:  gm,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer server.Close()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	duckyScript := filepath.Join(tmpDir, "payload.ducky")
	if err := os.WriteFile(duckyScript, []byte("STRING ñ\n"), 0644); err != nil {
		t.Fatalf("WriteFile duckyScript: %v", err)
	}

	// Without -layout flag and with explicit -delay 0, delegated run must use ES layout and 0ms delay
	if err := runScriptCLI([]string{duckyScript, "-delay", "0", "-server", ts.URL}); err != nil {
		t.Fatalf("runScriptCLI via daemon failed: %v", err)
	}
	if hidOut.Len() < 8 || hidOut.Bytes()[2] != hid.KeySemicolon {
		t.Errorf("Expected daemon keyboard to type ES 'ñ' (KeySemicolon), got %v", hidOut.Bytes())
	}
	restoredDelay, _ := daemonKbd.GetTypingSpeed()
	if restoredDelay != 50 {
		t.Errorf("Expected daemon typing delay to restore to 50ms after -delay 0 override, got %d", restoredDelay)
	}
}

func TestRunViaDaemon_DoesNotFallbackOnDaemonError(t *testing.T) {
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()

	handled, err := runViaDaemon(context.Background(), errServer.URL, "p.ducky", "ENTER\n", "", nil)
	if !handled || err == nil {
		t.Fatalf("Expected daemon 500 to be treated as handled error without local fallback, got handled=%v err=%v", handled, err)
	}
}

func TestCheckDaemonJobStatus_SupersededJobReturnsError(t *testing.T) {
	nullJobServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("null"))
	}))
	defer nullJobServer.Close()

	done, err := checkDaemonJobStatus(context.Background(), nullJobServer.Client(), nullJobServer.URL, "job-123")
	if done || err == nil {
		t.Fatalf("Expected error when daemon returns null job, got done=%v err=%v", done, err)
	}
}

func TestRunDaemon_GracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	storageDir := filepath.Join(t.TempDir(), "storage")
	errCh := make(chan error, 1)
	go func() {
		errCh <- runDaemon([]string{"-port", addr, "-storage", storageDir, "-layout", "ES", "-delay", "0"})
	}()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, getErr := http.Get("http://" + addr + "/api/job")
		if getErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)

	select {
	case daemonErr := <-errCh:
		if daemonErr != nil {
			t.Fatalf("runDaemon returned error: %v", daemonErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runDaemon did not shut down within 5s")
	}
}
