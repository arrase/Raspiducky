package hid

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func assertReport(t *testing.T, report []byte, mod, key byte) {
	t.Helper()
	if report[0] != mod || report[2] != key {
		t.Errorf("Unexpected report (expected mod=%#x, key=%#x): %v", mod, key, report[:8])
	}
}

func TestKeyboard_TypeString(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer kbd.Close()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	ctx := context.Background()
	start := time.Now()
	if err := kbd.TypeString(ctx, "aA"); err != nil {
		t.Fatalf("TypeString failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("Expected TypeString(\"aA\") to hold at least 40ms, took %v", elapsed)
	}

	// Each character writes 1 press report (8 bytes) + 1 release report (8 bytes) = 16 bytes
	data := buf.Bytes()
	if len(data) != 32 {
		t.Fatalf("Expected 32 bytes written, got %d", len(data))
	}

	assertReport(t, data[0:8], 0, KeyA)
	assertReport(t, data[8:16], 0, 0)
	assertReport(t, data[16:24], ModLeftShift, KeyA)
}

func TestKeyboard_Press(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer kbd.Close()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	ctx := context.Background()
	start := time.Now()
	if err := kbd.Press(ctx, "GUI r"); err != nil {
		t.Fatalf("Press failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("Expected Press(\"GUI r\") to hold at least 40ms, took %v", elapsed)
	}
	data := buf.Bytes()
	// Modifier-only report (8B) + modifier+key report (8B) + release report (8B) = 24 bytes
	if len(data) != 24 {
		t.Fatalf("Expected 24 bytes for modifier+key Press, got %d", len(data))
	}
	assertReport(t, data[0:8], ModLeftGUI, 0)
	assertReport(t, data[8:16], ModLeftGUI, KeyR)
	assertReport(t, data[16:24], 0, 0)

	buf.Reset()
	start = time.Now()
	if err := kbd.Press(ctx, "ENTER"); err != nil {
		t.Fatalf("Press ENTER failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Errorf("Expected Press(\"ENTER\") to hold at least 20ms, took %v", elapsed)
	}
	data = buf.Bytes()
	if len(data) != 16 {
		t.Fatalf("Expected 16 bytes for single-key Press, got %d", len(data))
	}
	assertReport(t, data[0:8], 0, KeyEnter)

	buf.Reset()
	if err := kbd.Press(ctx, "GUI"); err != nil {
		t.Fatalf("Press GUI failed: %v", err)
	}
	data = buf.Bytes()
	if len(data) != 16 {
		t.Fatalf("Expected 16 bytes for modifier-only Press, got %d", len(data))
	}
	assertReport(t, data[0:8], ModLeftGUI, 0)
}

func TestKeyboard_DevicePathAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	dev1 := filepath.Join(tmpDir, "hidg0_1")
	dev2 := filepath.Join(tmpDir, "hidg0_2")

	if err := os.WriteFile(dev1, nil, 0666); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dev2, nil, 0666); err != nil {
		t.Fatal(err)
	}

	kbd, err := NewKeyboard(dev1, "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer kbd.Close()

	if err := kbd.WriteReport(NewKeyboardReport(0, KeyA)); err != nil {
		t.Fatalf("WriteReport to dev1 failed: %v", err)
	}

	kbd.SetDevicePath(dev2)
	if err := kbd.WriteReport(NewKeyboardReport(0, KeyB)); err != nil {
		t.Fatalf("WriteReport to dev2 failed: %v", err)
	}

	b1, _ := os.ReadFile(dev1)
	b2, _ := os.ReadFile(dev2)
	if len(b1) != 8 || b1[2] != KeyA {
		t.Errorf("dev1 unexpected content: %v", b1)
	}
	if len(b2) != 8 || b2[2] != KeyB {
		t.Errorf("dev2 unexpected content: %v", b2)
	}
}

func TestKeyboard_TypingSpeedAndCancel(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer kbd.Close()

	delay, jitter := kbd.GetTypingSpeed()
	if delay != 10 || jitter != 0 {
		t.Errorf("Expected default (10, 0), got (%d, %d)", delay, jitter)
	}

	kbd.SetTypingSpeed(25, 5)
	delay, jitter = kbd.GetTypingSpeed()
	if delay != 25 || jitter != 5 {
		t.Errorf("Expected (25, 5), got (%d, %d)", delay, jitter)
	}

	kbd.SetTypingSpeed(-5, -10)
	delay, jitter = kbd.GetTypingSpeed()
	if delay != 0 || jitter != 0 {
		t.Errorf("Expected clamped (0, 0), got (%d, %d)", delay, jitter)
	}

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(5, 2)
	if err := kbd.TypeString(context.Background(), "a"); err != nil {
		t.Fatalf("TypeString with delay and jitter failed: %v", err)
	}

	kbd.SetTypingSpeed(500, 0)

	ctxPress, cancelPress := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelPress()
	if err := kbd.Press(ctxPress, "ENTER"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in Press waitKeyDelay, got %v", err)
	}

	ctxType, cancelType := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancelType()
	if err := kbd.TypeString(ctxType, "a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in TypeString waitKeyDelay, got %v", err)
	}
}
