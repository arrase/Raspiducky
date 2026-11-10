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

func TestKeyboard_TypeStringAndCRLFNormalization(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer func() { _ = kbd.Close() }()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	ctx := context.Background()
	// Verify BOM (\ufeff), CRLF (\r\n), CR (\r), and unmapped emoji are handled cleanly:
	// "\ufeffa\r\nA\r🚀" -> 'a', '\n', 'A', '\n' (4 keystrokes = 8 reports = 64 bytes)
	if err := kbd.TypeString(ctx, "\ufeffa\r\nA\r🚀"); err != nil {
		t.Fatalf("TypeString failed: %v", err)
	}

	data := buf.Bytes()
	if len(data) != 64 {
		t.Fatalf("Expected 64 bytes written (4 chars, CRLF normalized), got %d", len(data))
	}

	assertReport(t, data[0:8], 0, KeyA)
	assertReport(t, data[8:16], 0, 0)
	assertReport(t, data[16:24], 0, KeyEnter)
	assertReport(t, data[24:32], 0, 0)
	assertReport(t, data[32:40], ModLeftShift, KeyA)
	assertReport(t, data[40:48], 0, 0)
	assertReport(t, data[48:56], 0, KeyEnter)
	assertReport(t, data[56:64], 0, 0)
}

func TestKeyboard_DeadKeyAndAccentedRunes(t *testing.T) {
	kbd, err := NewKeyboard("", "ES")
	if err != nil {
		t.Fatalf("NewKeyboard ES failed: %v", err)
	}
	defer func() { _ = kbd.Close() }()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	// In ES: 'á' = dead acute (KeyApostrophe) + 'a' (KeyA); '`' = dead grave (KeyLeftBrace) + Space (KeySpace)
	if err := kbd.TypeString(context.Background(), "á`"); err != nil {
		t.Fatalf("TypeString ES failed: %v", err)
	}

	data := buf.Bytes()
	// 2 two-step runes = 4 taps = 8 reports = 64 bytes
	if len(data) != 64 {
		t.Fatalf("Expected 64 bytes for 2 dead-key runes, got %d", len(data))
	}
	assertReport(t, data[0:8], 0, KeyApostrophe)
	assertReport(t, data[16:24], 0, KeyA)
	assertReport(t, data[32:40], 0, KeyLeftBrace)
	assertReport(t, data[48:56], 0, KeySpace)
}

func TestKeyboard_PressAndComboSeparators(t *testing.T) {
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	defer func() { _ = kbd.Close() }()

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(0, 0)

	ctx := context.Background()
	if err := kbd.Press(ctx, "GUI r"); err != nil {
		t.Fatalf("Press failed: %v", err)
	}
	data := buf.Bytes()
	if len(data) != 24 {
		t.Fatalf("Expected 24 bytes for modifier+key Press, got %d", len(data))
	}
	assertReport(t, data[0:8], ModLeftGUI, 0)
	assertReport(t, data[8:16], ModLeftGUI, KeyR)
	assertReport(t, data[16:24], 0, 0)

	buf.Reset()
	if err := kbd.Press(ctx, "CTRL-ALT-DELETE"); err != nil {
		t.Fatalf("Press CTRL-ALT-DELETE failed: %v", err)
	}
	assertReport(t, buf.Bytes()[8:16], ModLeftCtrl|ModLeftAlt, KeyDelete)

	buf.Reset()
	if err := kbd.Press(ctx, "CTRL +"); err != nil {
		t.Fatalf("Press CTRL + failed: %v", err)
	}
	assertReport(t, buf.Bytes()[8:16], ModLeftCtrl, KeyEqual)

	buf.Reset()
	if err := kbd.Press(ctx, "CTRL -"); err != nil {
		t.Fatalf("Press CTRL - failed: %v", err)
	}
	assertReport(t, buf.Bytes()[8:16], ModLeftCtrl, KeyMinus)

	buf.Reset()
	if err := kbd.Press(ctx, "ENTER"); err != nil {
		t.Fatalf("Press ENTER failed: %v", err)
	}
	if len(buf.Bytes()) != 16 {
		t.Fatalf("Expected 16 bytes for single-key Press, got %d", len(buf.Bytes()))
	}
	assertReport(t, buf.Bytes()[0:8], 0, KeyEnter)

	buf.Reset()
	if err := kbd.Press(ctx, "GUI"); err != nil {
		t.Fatalf("Press GUI failed: %v", err)
	}
	if len(buf.Bytes()) != 16 {
		t.Fatalf("Expected 16 bytes for modifier-only Press, got %d", len(buf.Bytes()))
	}
	assertReport(t, buf.Bytes()[0:8], ModLeftGUI, 0)
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

	// Verify SetWriter followed by SetDevicePath clears the custom writer and writes to dev2
	var tmpBuf bytes.Buffer
	kbd.SetWriter(&tmpBuf)
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

func TestKeyboard_TypingSpeedTimingAndCancel(t *testing.T) {
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
	if got := kbd.holdDuration(); got != fastHoldDuration {
		t.Errorf("Expected fastHoldDuration %v at 0ms delay, got %v", fastHoldDuration, got)
	}

	var buf bytes.Buffer
	kbd.SetWriter(&buf)
	kbd.SetTypingSpeed(3, 2)
	if got := kbd.holdDuration(); got != minHoldDuration {
		t.Errorf("Expected minHoldDuration %v for 3ms delay, got %v", minHoldDuration, got)
	}
	if got := kbd.holdDurationForKey(KeyEnter); got != enterHoldDuration {
		t.Errorf("Expected enterHoldDuration %v for KeyEnter when delay>0, got %v", enterHoldDuration, got)
	}
	if err := kbd.TypeString(context.Background(), "a\n"); err != nil {
		t.Fatalf("TypeString with delay and jitter failed: %v", err)
	}

	kbd.SetTypingSpeed(500, 0)
	if got := kbd.holdDuration(); got != maxHoldDuration {
		t.Errorf("Expected holdDuration capped at %v (never triggering OS key repeat), got %v", maxHoldDuration, got)
	}

	ctxHoldPress, cancelHoldPress := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancelHoldPress()
	if err := kbd.Press(ctxHoldPress, "ENTER"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in Press holdDuration, got %v", err)
	}

	ctxPress, cancelPress := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancelPress()
	if err := kbd.Press(ctxPress, "ENTER"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in Press waitKeyDelay, got %v", err)
	}

	ctxHoldType, cancelHoldType := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancelHoldType()
	if err := kbd.TypeString(ctxHoldType, "a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in TypeString holdDuration, got %v", err)
	}

	ctxType, cancelType := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancelType()
	if err := kbd.TypeString(ctxType, "a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected DeadlineExceeded in TypeString waitKeyDelay, got %v", err)
	}
}

func TestHIDWriter_BlockedPipeCancelAndTimeout(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	// Fill the OS pipe buffer completely so the next write blocks in epoll
	chunk := bytes.Repeat([]byte{0xAA}, 4096)
	_ = w.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
	for {
		if _, writeErr := w.Write(chunk); writeErr != nil {
			break
		}
	}
	_ = w.SetWriteDeadline(time.Time{})

	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatal(err)
	}
	kbd.setActiveFile(w)
	kbd.writer = w

	// Verify context cancellation unblocks a stalled write quickly without deadlocking
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = kbd.writeReportContext(ctx, NewKeyboardReport(0, KeyA))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected context.DeadlineExceeded on blocked pipe, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Expected blocked write to abort promptly on ctx cancel, took %v", elapsed)
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

func TestKeyboard_HoldAndReleaseComboAndRolloverLimit(t *testing.T) {
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
	if err := kbd.Hold(ctx, "a b c d e f g"); err == nil {
		t.Errorf("Expected error when holding >6 non-modifier keys")
	}
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

	if err := kbd.Hold(ctx, "SHIFT"); err != nil {
		t.Fatalf("Hold SHIFT failed: %v", err)
	}
	if err := kbd.Press(ctx, "DOWNARROW"); err != nil {
		t.Fatalf("Press DOWNARROW failed: %v", err)
	}
	if err := kbd.TypeString(ctx, "b"); err != nil {
		t.Fatalf("TypeString failed: %v", err)
	}
	if err := kbd.ReleaseKeys(); err != nil {
		t.Fatalf("ReleaseKeys failed: %v", err)
	}

	data := buf.Bytes()
	if len(data) != 6*8 {
		t.Fatalf("Expected 48 bytes (6 reports), got %d", len(data))
	}

	assertReport(t, data[0:8], ModLeftShift, 0)
	assertReport(t, data[8:16], ModLeftShift, KeyDown)
	assertReport(t, data[16:24], ModLeftShift, 0)
	assertReport(t, data[24:32], ModLeftShift, KeyB)
	assertReport(t, data[32:40], ModLeftShift, 0)
	assertReport(t, data[40:48], 0, 0)

	// Verify typing a key that is currently held briefly releases it first to generate an edge
	buf.Reset()
	mustHold(t, kbd, "a")
	if err := kbd.TypeString(ctx, "a"); err != nil {
		t.Fatalf("TypeString held 'a' failed: %v", err)
	}
	if len(buf.Bytes()) != 4*8 {
		t.Fatalf("Expected 4 reports (hold a, pre-release, press a, restore hold a), got %d bytes", len(buf.Bytes()))
	}
	assertReport(t, buf.Bytes()[8:16], 0, 0)
	assertReport(t, buf.Bytes()[16:24], 0, KeyA)
}
