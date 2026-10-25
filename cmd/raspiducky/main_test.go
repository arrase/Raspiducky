package main

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/arrase/Raspiducky/pkg/api"
	"github.com/arrase/Raspiducky/pkg/gadget"
	"github.com/arrase/Raspiducky/pkg/hid"
)

func TestInitHIDDevicesAndRunScriptCLI(t *testing.T) {
	printUsage()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kbd, mouse, ledWatcher, err := initHIDDevices(ctx, "US", 25)
	if err != nil {
		t.Fatalf("initHIDDevices failed: %v", err)
	}
	defer kbd.Close()
	defer mouse.Close()
	defer ledWatcher.Close()

	delay, jitter := kbd.GetTypingSpeed()
	if delay != 25 || jitter != 0 {
		t.Errorf("Expected typing speed (25, 0), got (%d, %d)", delay, jitter)
	}

	tmpDir := t.TempDir()
	tmpScript := filepath.Join(tmpDir, "test.js")
	if err := os.WriteFile(tmpScript, []byte("delay(1);\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Local execution fallback when daemon is not reachable
	if err := runScriptCLI([]string{tmpScript, "-layout", "US", "-delay", "30", "-server", ""}); err != nil {
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

	server, err := api.NewServer(api.ServerOptions{
		StorageDir:     filepath.Join(tmpDir, "storage"),
		Keyboard:       daemonKbd,
		Mouse:          daemonMouse,
		KeyboardLayout: "ES",
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

	// Without -layout flag, delegated run must use the daemon's configured ES layout
	if err := runScriptCLI([]string{duckyScript, "-delay", "1", "-server", ts.URL}); err != nil {
		t.Fatalf("runScriptCLI via daemon failed: %v", err)
	}
	if hidOut.Len() < 8 || hidOut.Bytes()[2] != hid.KeySemicolon {
		t.Errorf("Expected daemon keyboard to type ES 'ñ' (KeySemicolon), got %v", hidOut.Bytes())
	}
}
