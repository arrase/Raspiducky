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
	defer func() { _ = kbd.Close() }()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	ctx := context.Background()
	start := time.Now()
	if err := kbd.TypeString(ctx, "aA"); err != nil {
		t.Fatalf("TypeString failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 140*time.Millisecond {
		t.Errorf("Expected TypeString(\"aA\") to hold and pause at least 140ms, took %v", elapsed)
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
	defer func() { _ = kbd.Close() }()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	ctx := context.Background()
	start := time.Now()
	if err := kbd.Press(ctx, "GUI r"); err != nil {
		t.Fatalf("Press failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 105*time.Millisecond {
		t.Errorf("Expected Press(\"GUI r\") to hold and pause at least 105ms, took %v", elapsed)
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
	if elapsed := time.Since(start); elapsed < 70*time.Millisecond {
		t.Errorf("Expected Press(\"ENTER\") to hold and pause at least 70ms, took %v", elapsed)
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
	defer func() { _ = kbd.Close() }()

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
	defer func() { _ = kbd.Close() }()

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
	if got := kbd.holdDuration(); got != minKeyHoldDuration {
		t.Errorf("Expected holdDuration %v for 5ms delay, got %v", minKeyHoldDuration, got)
	}
	if err := kbd.TypeString(context.Background(), "a"); err != nil {
		t.Fatalf("TypeString with delay and jitter failed: %v", err)
	}

	kbd.SetTypingSpeed(50, 0)
	if got := kbd.holdDuration(); got != 50*time.Millisecond {
		t.Errorf("Expected holdDuration 50ms, got %v", got)
	}

	kbd.SetTypingSpeed(500, 0)
	if got := kbd.holdDuration(); got != maxKeyHoldDuration {
		t.Errorf("Expected holdDuration capped at %v, got %v", maxKeyHoldDuration, got)
	}

	ctxHoldPress, cancelHoldPress := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelHoldPress()
	if err := kbd.Press(ctxHoldPress, "ENTER"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in Press holdDuration, got %v", err)
	}

	ctxPress, cancelPress := context.WithTimeout(context.Background(), 130*time.Millisecond)
	defer cancelPress()
	if err := kbd.Press(ctxPress, "ENTER"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in Press waitKeyDelay, got %v", err)
	}

	ctxHoldType, cancelHoldType := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelHoldType()
	if err := kbd.TypeString(ctxHoldType, "a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in TypeString holdDuration, got %v", err)
	}

	ctxType, cancelType := context.WithTimeout(context.Background(), 130*time.Millisecond)
	defer cancelType()
	if err := kbd.TypeString(ctxType, "a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in TypeString waitKeyDelay, got %v", err)
	}
}

func mustHold(t *testing.T, kbd *Keyboard, keys string) {
	t.Helper()
	if err := kbd.Hold(context.Background(), keys); err != nil {
		t.Fatalf("Hold %q failed: %v", keys, err)
	}
}

func mustRelease(t *testing.T, kbd *Keyboard, keys string) {
	t.Helper()
	if err := kbd.Release(context.Background(), keys); err != nil {
		t.Fatalf("Release %q failed: %v", keys, err)
	}
}

func TestKeyboard_HoldAndRelease(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer func() { _ = kbd.Close() }()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	mustHold(t, kbd, "SHIFT")
	mustHold(t, kbd, "DOWNARROW")
	mustHold(t, kbd, "a")
	mustHold(t, kbd, "DOWNARROW") // Duplicate hold should not duplicate keycode
	mustRelease(t, kbd, "DOWNARROW")
	mustRelease(t, kbd, "a")
	mustRelease(t, kbd, "SHIFT")

	data := buf.Bytes()
	if len(data) != 7*8 {
		t.Fatalf("Expected 56 bytes (7 reports), got %d", len(data))
	}

	assertReport(t, data[0:8], ModLeftShift, 0)
	assertReport(t, data[8:16], ModLeftShift, KeyDown)
	if data[16] != ModLeftShift || data[18] != KeyDown || data[19] != KeyA {
		t.Errorf("Unexpected 3rd report: %v", data[16:24])
	}
	if data[24] != ModLeftShift || data[26] != KeyDown || data[27] != KeyA || data[28] != 0 {
		t.Errorf("Unexpected 4th report (duplicate hold): %v", data[24:32])
	}
	assertReport(t, data[32:40], ModLeftShift, KeyA)
	assertReport(t, data[40:48], ModLeftShift, 0)
	assertReport(t, data[48:56], 0, 0)
}

func TestKeyboard_HoldAndReleaseCombo(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer func() { _ = kbd.Close() }()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	mustHold(t, kbd, "CTRL LEFTARROW UPARROW")
	mustRelease(t, kbd, "LEFTARROW")
	mustRelease(t, kbd, "CTRL")
	mustRelease(t, kbd, "UPARROW")

	data := buf.Bytes()
	if len(data) != 4*8 {
		t.Fatalf("Expected 32 bytes (4 reports), got %d", len(data))
	}
	if data[0] != ModLeftCtrl || data[2] != KeyLeft || data[3] != KeyUp {
		t.Errorf("Unexpected multi-hold report: %v", data[0:8])
	}
	assertReport(t, data[8:16], ModLeftCtrl, KeyUp)
	assertReport(t, data[16:24], 0, KeyUp)
	assertReport(t, data[24:32], 0, 0)

	ctx := context.Background()
	if err := kbd.Hold(ctx, "NONEXISTENT_KEY"); err == nil {
		t.Errorf("Expected error for unknown key token in Hold")
	}
	if err := kbd.Release(ctx, "NONEXISTENT_KEY"); err == nil {
		t.Errorf("Expected error for unknown key token in Release")
	}
}

func TestKeyboard_HoldWithPressAndTypeString(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer func() { _ = kbd.Close() }()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	ctx := context.Background()

	// Hold SHIFT
	if err := kbd.Hold(ctx, "SHIFT"); err != nil {
		t.Fatalf("Hold SHIFT failed: %v", err)
	}

	// Press DOWNARROW while SHIFT is held
	if err := kbd.Press(ctx, "DOWNARROW"); err != nil {
		t.Fatalf("Press DOWNARROW failed: %v", err)
	}

	// Type "b" while SHIFT is held
	if err := kbd.TypeString(ctx, "b"); err != nil {
		t.Fatalf("TypeString failed: %v", err)
	}

	// ReleaseKeys clears all held keys
	if err := kbd.ReleaseKeys(); err != nil {
		t.Fatalf("ReleaseKeys failed: %v", err)
	}

	data := buf.Bytes()
	// 1 (Hold SHIFT) + 2 (Press DOWNARROW: down + release to held state) + 2 (TypeString "b": down + release to held state) + 1 (ReleaseKeys) = 6 reports = 48 bytes
	if len(data) != 6*8 {
		t.Fatalf("Expected 48 bytes (6 reports), got %d", len(data))
	}

	assertReport(t, data[0:8], ModLeftShift, 0)
	assertReport(t, data[8:16], ModLeftShift, KeyDown)
	assertReport(t, data[16:24], ModLeftShift, 0)
	assertReport(t, data[24:32], ModLeftShift, KeyB)
	assertReport(t, data[32:40], ModLeftShift, 0)
	assertReport(t, data[40:48], 0, 0)
}
