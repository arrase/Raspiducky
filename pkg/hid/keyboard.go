package hid

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Keyboard provides thread-safe access to a USB HID keyboard device (/dev/hidg0).
type Keyboard struct {
	hidWriter
	mu               sync.Mutex
	layout           *Layout
	keyDelayMs       int
	keyDelayJitterMs int
	heldModifiers    uint8
	heldKeys         []uint8
}

// NewKeyboard initializes a new Keyboard connected to the given device path with the specified layout.
func NewKeyboard(devicePath, layoutName string) (*Keyboard, error) {
	layout, err := GetLayout(layoutName)
	if err != nil {
		return nil, fmt.Errorf("loading keyboard layout %q: %w", layoutName, err)
	}

	return &Keyboard{
		hidWriter:  hidWriter{devicePath: devicePath},
		layout:     layout,
		keyDelayMs: 10,
	}, nil
}

// SetLayout changes the active keyboard layout.
func (kbd *Keyboard) SetLayout(layoutName string) error {
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	layout, err := GetLayout(layoutName)
	if err != nil {
		return err
	}
	kbd.layout = layout
	return nil
}

// GetLayout returns the active keyboard layout name.
func (kbd *Keyboard) GetLayout() string {
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	return kbd.layout.Name
}

// SetTypingSpeed sets inter-keystroke delay and optional random jitter in milliseconds.
func (kbd *Keyboard) SetTypingSpeed(delayMs, jitterMs int) {
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	if delayMs < 0 {
		delayMs = 0
	}
	if jitterMs < 0 {
		jitterMs = 0
	}
	kbd.keyDelayMs = delayMs
	kbd.keyDelayJitterMs = jitterMs
}

// GetTypingSpeed returns the current inter-keystroke delay and random jitter in milliseconds.
func (kbd *Keyboard) GetTypingSpeed() (int, int) {
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	return kbd.keyDelayMs, kbd.keyDelayJitterMs
}

const minKeyHoldDuration = 20 * time.Millisecond

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func (kbd *Keyboard) waitKeyDelay(ctx context.Context) error {
	kbd.mu.Lock()
	delay := kbd.keyDelayMs
	jitter := kbd.keyDelayJitterMs
	kbd.mu.Unlock()

	totalDelay := delay
	if jitter > 0 {
		totalDelay += rand.Intn(jitter)
	}
	return sleepContext(ctx, time.Duration(totalDelay)*time.Millisecond)
}

// WriteReport writes an 8-byte KeyboardReport to the device in a thread-safe manner.
func (kbd *Keyboard) WriteReport(report KeyboardReport) error {
	return kbd.writeReport(report[:])
}

// ReleaseKeys clears all held keys and sends an empty report to release all currently pressed keys.
func (kbd *Keyboard) ReleaseKeys() error {
	kbd.mu.Lock()
	kbd.heldModifiers = 0
	kbd.heldKeys = nil
	kbd.mu.Unlock()
	return kbd.WriteReport(KeyboardReport{})
}

func appendUniqueKey(keys []uint8, key uint8) []uint8 {
	for _, k := range keys {
		if k == key {
			return keys
		}
	}
	return append(keys, key)
}

func removeKey(keys []uint8, key uint8) []uint8 {
	out := make([]uint8, 0, len(keys))
	for _, k := range keys {
		if k != key {
			out = append(out, k)
		}
	}
	return out
}

func (kbd *Keyboard) activeReport(modifiers uint8, keycodes ...uint8) KeyboardReport {
	kbd.mu.Lock()
	mods := kbd.heldModifiers | modifiers
	keys := append([]uint8(nil), kbd.heldKeys...)
	kbd.mu.Unlock()

	for _, kc := range keycodes {
		if kc != 0 {
			keys = appendUniqueKey(keys, kc)
		}
	}
	return NewKeyboardReport(mods, keys...)
}

func (kbd *Keyboard) parseCombo(comboStr, action string) (uint8, []uint8, bool, error) {
	tokens := strings.Fields(comboStr)
	if len(tokens) == 0 {
		return 0, nil, false, nil
	}

	kbd.mu.Lock()
	layout := kbd.layout
	kbd.mu.Unlock()

	var modifiers uint8
	var keycodes []uint8

	for _, token := range tokens {
		combo, ok := layout.MapKeyName(token)
		if !ok {
			return 0, nil, false, fmt.Errorf("unknown key token in %s '%s': %s", action, comboStr, token)
		}
		modifiers |= combo.Modifiers
		if combo.KeyCode != 0 {
			keycodes = append(keycodes, combo.KeyCode)
		}
	}

	return modifiers, keycodes, true, nil
}

// Hold presses and holds one or more keys until explicitly released via Release or ReleaseKeys.
func (kbd *Keyboard) Hold(ctx context.Context, comboStr string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	modifiers, keycodes, ok, err := kbd.parseCombo(comboStr, "hold")
	if err != nil || !ok {
		return err
	}

	kbd.mu.Lock()
	kbd.heldModifiers |= modifiers
	for _, kc := range keycodes {
		kbd.heldKeys = appendUniqueKey(kbd.heldKeys, kc)
	}
	report := NewKeyboardReport(kbd.heldModifiers, kbd.heldKeys...)
	kbd.mu.Unlock()

	if err := kbd.WriteReport(report); err != nil {
		return err
	}
	return kbd.waitKeyDelay(ctx)
}

// Release releases one or more held keys while keeping any other held keys depressed.
func (kbd *Keyboard) Release(ctx context.Context, comboStr string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	modifiers, keycodes, ok, err := kbd.parseCombo(comboStr, "release")
	if err != nil || !ok {
		return err
	}

	kbd.mu.Lock()
	kbd.heldModifiers &^= modifiers
	for _, kc := range keycodes {
		kbd.heldKeys = removeKey(kbd.heldKeys, kc)
	}
	report := NewKeyboardReport(kbd.heldModifiers, kbd.heldKeys...)
	kbd.mu.Unlock()

	if err := kbd.WriteReport(report); err != nil {
		return err
	}
	return kbd.waitKeyDelay(ctx)
}

// Press parses and executes a key combination string like "CTRL ALT DELETE" or "GUI R".
func (kbd *Keyboard) Press(ctx context.Context, comboStr string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	modifiers, keycodes, ok, err := kbd.parseCombo(comboStr, "press")
	if err != nil || !ok {
		return err
	}

	if modifiers != 0 && len(keycodes) > 0 {
		if err := kbd.WriteReport(kbd.activeReport(modifiers)); err != nil {
			return err
		}
		if err := sleepContext(ctx, minKeyHoldDuration); err != nil {
			_ = kbd.ReleaseKeys()
			return err
		}
	}

	if err := kbd.WriteReport(kbd.activeReport(modifiers, keycodes...)); err != nil {
		return err
	}

	if err := sleepContext(ctx, minKeyHoldDuration); err != nil {
		_ = kbd.ReleaseKeys()
		return err
	}
	if err := kbd.WriteReport(kbd.activeReport(0)); err != nil {
		return err
	}
	return kbd.waitKeyDelay(ctx)
}

// TypeString types out the given text string character by character according to the current layout and typing speed.
func (kbd *Keyboard) TypeString(ctx context.Context, text string) error {
	for _, r := range text {
		if err := ctx.Err(); err != nil {
			return err
		}

		kbd.mu.Lock()
		layout := kbd.layout
		kbd.mu.Unlock()

		report, ok := layout.Runes[r]
		if !ok {
			return fmt.Errorf("unmapped rune %q in layout %s", r, layout.Name)
		}

		if err := kbd.WriteReport(kbd.activeReport(report[0], report.KeyCodes()...)); err != nil {
			return err
		}
		if err := sleepContext(ctx, minKeyHoldDuration); err != nil {
			_ = kbd.ReleaseKeys()
			return err
		}
		if err := kbd.WriteReport(kbd.activeReport(0)); err != nil {
			return err
		}
		if err := kbd.waitKeyDelay(ctx); err != nil {
			return err
		}
	}
	return nil
}
