package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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

	tmpScript := filepath.Join(t.TempDir(), "test.js")
	if err := os.WriteFile(tmpScript, []byte("delay(1);\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := runScriptCLI([]string{tmpScript, "-layout", "US", "-delay", "30"}); err != nil {
		t.Fatalf("runScriptCLI failed: %v", err)
	}
}
