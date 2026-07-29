package hid

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestLEDWatcher_GetStateAndSubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := NewLEDWatcher(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create LEDWatcher: %v", err)
	}

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

func TestLEDWatcher_SetReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := NewLEDWatcher(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create LEDWatcher: %v", err)
	}

	ch, unsubscribe := watcher.Subscribe()
	defer unsubscribe()

	buf := []byte{LEDCapsLock | LEDNumLock, 0, 0, 0, 0, 0, 0, 0}
	r := bytes.NewReader(buf)
	watcher.SetReader(r)

	select {
	case state := <-ch:
		if !state.CapsLock || !state.NumLock {
			t.Errorf("Expected CapsLock and NumLock to be true, got %+v", state)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for LED state update via reader")
	}
}
