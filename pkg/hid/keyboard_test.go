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

func TestKeyboard_TypeStringAndPress(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer kbd.Close()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	ctx := context.Background()
	if err := kbd.TypeString(ctx, "aA"); err != nil {
		t.Fatalf("TypeString failed: %v", err)
	}

	// Each character writes 1 press report (8 bytes) + 1 release report (8 bytes) = 16 bytes
	data := buf.Bytes()
	if len(data) != 32 {
		t.Fatalf("Expected 32 bytes written, got %d", len(data))
	}

	// 'a' -> mod 0, KeyA
	if data[0] != 0 || data[2] != KeyA {
		t.Errorf("Unexpected report for 'a': %v", data[:8])
	}
	// release -> all zeros
	if data[8] != 0 || data[10] != 0 {
		t.Errorf("Unexpected release report: %v", data[8:16])
	}
	// 'A' -> ModLeftShift, KeyA
	if data[16] != ModLeftShift || data[18] != KeyA {
		t.Errorf("Unexpected report for 'A': %v", data[16:24])
	}

	buf.Reset()
	if err := kbd.Press(ctx, "GUI r"); err != nil {
		t.Fatalf("Press failed: %v", err)
	}
	data = buf.Bytes()
	if len(data) != 16 {
		t.Fatalf("Expected 16 bytes for Press, got %d", len(data))
	}
	if data[0] != ModLeftGUI || data[2] != KeyR {
		t.Errorf("Unexpected report for 'GUI r': %v", data[:8])
	}
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
