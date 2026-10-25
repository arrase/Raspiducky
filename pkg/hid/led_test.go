package hid

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLEDWatcher_GetStateAndSubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher := NewLEDWatcher(ctx, "")
	defer watcher.Close()

	initialState := watcher.GetState()
	if initialState.CapsLock || initialState.NumLock || initialState.ScrollLock {
		t.Errorf("Expected initial state to be all false, got %+v", initialState)
	}

	ch, unsubscribe := watcher.Subscribe()
	defer unsubscribe()

	// Update state to CapsLock ON (0x02)
	watcher.UpdateState(LEDCapsLock)

	select {
	case state := <-ch:
		if !state.CapsLock {
			t.Errorf("Expected CapsLock to be true, got false")
		}
		if state.NumLock || state.ScrollLock {
			t.Errorf("Expected NumLock and ScrollLock to be false, got %+v", state)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for LED state update channel")
	}

	currentState := watcher.GetState()
	if !currentState.CapsLock {
		t.Errorf("Expected GetState CapsLock to be true, got false")
	}
}

func TestLEDWatcher_MultiByteReadAndSetDevicePath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	dev1 := filepath.Join(tmpDir, "hidg0_1")
	dev2 := filepath.Join(tmpDir, "hidg0_2")

	// Write 2 consecutive LED state bytes into dev1 to verify buf[0:n] are all processed
	if err := os.WriteFile(dev1, []byte{LEDCapsLock, LEDCapsLock | LEDNumLock}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dev2, []byte{LEDScrollLock}, 0644); err != nil {
		t.Fatal(err)
	}

	watcher := NewLEDWatcher(ctx, "")
	defer watcher.Close()

	ch, unsubscribe := watcher.Subscribe()
	defer unsubscribe()

	watcher.SetDevicePath(dev1)

	var gotStates []LEDState
	for len(gotStates) < 2 {
		select {
		case st := <-ch:
			gotStates = append(gotStates, st)
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout waiting for multi-byte LED states, got %d: %+v", len(gotStates), gotStates)
		}
	}
	if !gotStates[0].CapsLock || gotStates[0].NumLock {
		t.Errorf("Expected 1st state CapsLock only, got %+v", gotStates[0])
	}
	if !gotStates[1].CapsLock || !gotStates[1].NumLock {
		t.Errorf("Expected 2nd state CapsLock+NumLock, got %+v", gotStates[1])
	}

	watcher.SetDevicePath(dev2)

	select {
	case state := <-ch:
		if !state.ScrollLock || state.CapsLock || state.NumLock {
			t.Errorf("Expected only ScrollLock to be true, got %+v", state)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for second device LED state update")
	}
}

func TestLEDWatcher_WaitLED_TransitionsAndStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher := NewLEDWatcher(ctx, "")

	// Turn CapsLock ON
	go func() {
		time.Sleep(20 * time.Millisecond)
		watcher.UpdateState(LEDCapsLock)
	}()

	state, err := watcher.WaitLED(ctx, LEDCapsLock, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitLED ON failed: %v", err)
	}
	if !state.CapsLock {
		t.Errorf("Expected CapsLock ON, got %+v", state)
	}

	// Turning CapsLock OFF should also notify a listener waiting on LEDCapsLock
	go func() {
		time.Sleep(20 * time.Millisecond)
		watcher.UpdateState(0)
	}()

	state, err = watcher.WaitLED(ctx, LEDCapsLock, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitLED OFF failed: %v", err)
	}
	if state.CapsLock {
		t.Errorf("Expected CapsLock OFF, got %+v", state)
	}

	// Closing watcher while WaitLED is blocked should unblock immediately with error
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = watcher.Close()
	}()

	_, err = watcher.WaitLED(ctx, LEDNumLock, 2*time.Second)
	if err == nil {
		t.Fatal("Expected error when watcher is closed during WaitLED")
	}
}

func TestLEDWatcher_DeviceFileClose(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "hidg0")
	if err := os.WriteFile(tmpFile, []byte{LEDNumLock}, 0644); err != nil {
		t.Fatalf("Failed to create temp device file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher := NewLEDWatcher(ctx, tmpFile)
	time.Sleep(50 * time.Millisecond)

	if !watcher.GetState().NumLock {
		t.Errorf("Expected NumLock to be read from device file")
	}

	if err := watcher.Close(); err != nil {
		t.Errorf("Unexpected error closing watcher: %v", err)
	}
}

func TestParseLEDMask(t *testing.T) {
	mask, err := ParseLEDMask("CAPS")
	if err != nil || mask != LEDCapsLock {
		t.Errorf("Expected LEDCapsLock, got %v, err=%v", mask, err)
	}

	mask, err = ParseLEDMask("NUM|SCROLL")
	if err != nil || mask != (LEDNumLock|LEDScrollLock) {
		t.Errorf("Expected LEDNumLock|LEDScrollLock, got %v, err=%v", mask, err)
	}

	if _, err := ParseLEDMask("INVALID"); err == nil {
		t.Error("Expected error for invalid LED mask")
	}
}
