package hid

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	fastHoldDuration  = 2 * time.Millisecond
	minHoldDuration   = 5 * time.Millisecond
	maxHoldDuration   = 20 * time.Millisecond
	enterHoldDuration = 20 * time.Millisecond
	fastGapDuration   = 2 * time.Millisecond
	minEnterDelayMs   = 25
	releaseCleanupTTL = 100 * time.Millisecond
	maxHeldKeys       = 6
)

// Keyboard provides thread-safe access to a USB HID keyboard device (/dev/hidg0).
type Keyboard struct {
	hidWriter
	opMu             sync.Mutex
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

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (kbd *Keyboard) holdDuration() time.Duration {
	kbd.mu.Lock()
	delayMs := kbd.keyDelayMs
	kbd.mu.Unlock()

	if delayMs <= 0 {
		return fastHoldDuration
	}
	d := time.Duration(delayMs) * time.Millisecond
	if d < minHoldDuration {
		return minHoldDuration
	}
	if d > maxHoldDuration {
		return maxHoldDuration
	}
	return d
}

func (kbd *Keyboard) holdDurationForKey(key uint8) time.Duration {
	hold := kbd.holdDuration()
	if key == KeyEnter && hold > fastHoldDuration && hold < enterHoldDuration {
		return enterHoldDuration
	}
	return hold
}

func (kbd *Keyboard) waitKeyDelay(ctx context.Context) error {
	return kbd.waitKeyDelayForKey(ctx, 0)
}

func (kbd *Keyboard) waitKeyDelayForKey(ctx context.Context, key uint8) error {
	kbd.mu.Lock()
	delay := kbd.keyDelayMs
	jitter := kbd.keyDelayJitterMs
	kbd.mu.Unlock()

	if delay <= 0 && jitter <= 0 {
		return sleepContext(ctx, fastGapDuration)
	}
	if key == KeyEnter && delay > 0 && delay < minEnterDelayMs {
		delay = minEnterDelayMs
	}
	if jitter > 0 {
		delay += rand.IntN(jitter)
	}
	return sleepContext(ctx, time.Duration(delay)*time.Millisecond)
}

// WriteReport writes an 8-byte KeyboardReport to the device in a thread-safe manner.
func (kbd *Keyboard) WriteReport(report KeyboardReport) error {
	return kbd.writeReport(context.Background(), report[:])
}

func (kbd *Keyboard) writeReportContext(ctx context.Context, report KeyboardReport) error {
	return kbd.writeReport(ctx, report[:])
}

func (kbd *Keyboard) restoreHeldStateBestEffort() {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), releaseCleanupTTL)
	defer cancel()
	report := kbd.activeReport(0)
	_ = kbd.writeReportContext(cleanupCtx, report)
}

// ReleaseKeys clears all held keys and sends an empty report to release all currently pressed keys.
func (kbd *Keyboard) ReleaseKeys() error {
	kbd.opMu.Lock()
	defer kbd.opMu.Unlock()

	kbd.mu.Lock()
	kbd.heldModifiers = 0
	kbd.heldKeys = nil
	kbd.mu.Unlock()

	cleanupCtx, cancel := context.WithTimeout(context.Background(), releaseCleanupTTL)
	defer cancel()
	return kbd.writeReportContext(cleanupCtx, KeyboardReport{})
}

// ReleaseHeldKeys clears and releases keys only if any modifier or key is currently held.
func (kbd *Keyboard) ReleaseHeldKeys() error {
	kbd.opMu.Lock()
	defer kbd.opMu.Unlock()

	kbd.mu.Lock()
	wasHeld := kbd.heldModifiers != 0 || len(kbd.heldKeys) > 0
	kbd.heldModifiers = 0
	kbd.heldKeys = nil
	kbd.mu.Unlock()

	if !wasHeld {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), releaseCleanupTTL)
	defer cancel()
	return kbd.writeReportContext(cleanupCtx, KeyboardReport{})
}

func appendUniqueKey(keys []uint8, key uint8) []uint8 {
	if slices.Contains(keys, key) {
		return keys
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

func (kbd *Keyboard) heldKeyPresent(key uint8) (KeyboardReport, bool) {
	if key == 0 {
		return KeyboardReport{}, false
	}
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	if !slices.Contains(kbd.heldKeys, key) {
		return KeyboardReport{}, false
	}
	return NewKeyboardReport(kbd.heldModifiers, removeKey(kbd.heldKeys, key)...), true
}

// SplitComboTokens splits a key combo string by whitespace, '-' or '+', preserving standalone '-' and '+' tokens.
func SplitComboTokens(comboStr string) []string {
	var tokens []string
	for _, word := range strings.Fields(comboStr) {
		if len(word) == 1 {
			tokens = append(tokens, word)
			continue
		}
		start := 0
		for i := 0; i < len(word); i++ {
			if word[i] != '-' && word[i] != '+' {
				continue
			}
			if i > start {
				tokens = append(tokens, word[start:i])
			}
			if i == len(word)-1 && i == start {
				tokens = append(tokens, word[i:])
			}
			start = i + 1
		}
		if start < len(word) {
			tokens = append(tokens, word[start:])
		}
	}
	return tokens
}

// ValidateCombo checks whether comboStr contains only valid key names for the given layout.
func ValidateCombo(layout *Layout, comboStr string) error {
	tokens := SplitComboTokens(comboStr)
	if len(tokens) == 0 {
		return errors.New("empty key combination")
	}
	for _, token := range tokens {
		if _, ok := layout.MapKeyName(token); !ok {
			return fmt.Errorf("unknown key token %q", token)
		}
	}
	return nil
}

func (kbd *Keyboard) parseCombo(comboStr, action string) (uint8, []uint8, bool, error) {
	tokens := SplitComboTokens(comboStr)
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
			keycodes = appendUniqueKey(keycodes, combo.KeyCode)
		}
	}

	return modifiers, keycodes, true, nil
}

// Hold presses and holds one or more keys until explicitly released via Release or ReleaseKeys.
func (kbd *Keyboard) Hold(ctx context.Context, comboStr string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	kbd.opMu.Lock()
	defer kbd.opMu.Unlock()

	modifiers, keycodes, ok, err := kbd.parseCombo(comboStr, "hold")
	if err != nil || !ok {
		return err
	}

	kbd.mu.Lock()
	nextMods := kbd.heldModifiers | modifiers
	nextKeys := append([]uint8(nil), kbd.heldKeys...)
	for _, kc := range keycodes {
		nextKeys = appendUniqueKey(nextKeys, kc)
	}
	kbd.mu.Unlock()

	if len(nextKeys) > maxHeldKeys {
		return fmt.Errorf("cannot hold more than %d keys simultaneously", maxHeldKeys)
	}

	if err := kbd.writeReportContext(ctx, NewKeyboardReport(nextMods, nextKeys...)); err != nil {
		return err
	}

	kbd.mu.Lock()
	kbd.heldModifiers = nextMods
	kbd.heldKeys = nextKeys
	kbd.mu.Unlock()

	return kbd.waitKeyDelay(ctx)
}

// Release releases one or more held keys while keeping any other held keys depressed.
func (kbd *Keyboard) Release(ctx context.Context, comboStr string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	kbd.opMu.Lock()
	defer kbd.opMu.Unlock()

	modifiers, keycodes, ok, err := kbd.parseCombo(comboStr, "release")
	if err != nil || !ok {
		return err
	}

	kbd.mu.Lock()
	nextMods := kbd.heldModifiers &^ modifiers
	nextKeys := append([]uint8(nil), kbd.heldKeys...)
	for _, kc := range keycodes {
		nextKeys = removeKey(nextKeys, kc)
	}
	kbd.mu.Unlock()

	if err := kbd.writeReportContext(ctx, NewKeyboardReport(nextMods, nextKeys...)); err != nil {
		return err
	}

	kbd.mu.Lock()
	kbd.heldModifiers = nextMods
	kbd.heldKeys = nextKeys
	kbd.mu.Unlock()

	return kbd.waitKeyDelay(ctx)
}

func (kbd *Keyboard) tapCombo(ctx context.Context, combo KeyCombo) error {
	if preRelease, held := kbd.heldKeyPresent(combo.KeyCode); held {
		if err := kbd.writeReportContext(ctx, preRelease); err != nil {
			return err
		}
		if err := sleepContext(ctx, kbd.holdDuration()); err != nil {
			kbd.restoreHeldStateBestEffort()
			return err
		}
	}

	if err := kbd.writeReportContext(ctx, kbd.activeReport(combo.Modifiers, combo.KeyCode)); err != nil {
		return err
	}
	if err := sleepContext(ctx, kbd.holdDurationForKey(combo.KeyCode)); err != nil {
		kbd.restoreHeldStateBestEffort()
		return err
	}
	if err := kbd.writeReportContext(ctx, kbd.activeReport(0)); err != nil {
		return err
	}
	return kbd.waitKeyDelayForKey(ctx, combo.KeyCode)
}

// Press parses and executes a key combination string like "CTRL ALT DELETE" or "GUI R".
func (kbd *Keyboard) Press(ctx context.Context, comboStr string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	kbd.opMu.Lock()
	defer kbd.opMu.Unlock()

	modifiers, keycodes, ok, err := kbd.parseCombo(comboStr, "press")
	if err != nil || !ok {
		return err
	}

	var primaryKey uint8
	if len(keycodes) == 1 {
		primaryKey = keycodes[0]
	}

	hold := kbd.holdDurationForKey(primaryKey)

	if modifiers != 0 && len(keycodes) > 0 {
		if err := kbd.writeReportContext(ctx, kbd.activeReport(modifiers)); err != nil {
			return err
		}
		if err := sleepContext(ctx, kbd.holdDuration()); err != nil {
			kbd.restoreHeldStateBestEffort()
			return err
		}
	}

	if len(keycodes) == 1 {
		if preRelease, held := kbd.heldKeyPresent(primaryKey); held {
			if err := kbd.writeReportContext(ctx, preRelease); err != nil {
				return err
			}
			if err := sleepContext(ctx, kbd.holdDuration()); err != nil {
				kbd.restoreHeldStateBestEffort()
				return err
			}
		}
	}

	if err := kbd.writeReportContext(ctx, kbd.activeReport(modifiers, keycodes...)); err != nil {
		return err
	}

	if err := sleepContext(ctx, hold); err != nil {
		kbd.restoreHeldStateBestEffort()
		return err
	}
	if err := kbd.writeReportContext(ctx, kbd.activeReport(0)); err != nil {
		return err
	}
	return kbd.waitKeyDelayForKey(ctx, primaryKey)
}

func isIgnoredFormatRune(r rune) bool {
	return r == '\ufeff' || r == '\u200b' || r == '\u200c' || r == '\u200d'
}

// TypeString types out the given text string character by character according to the current layout and typing speed.
func (kbd *Keyboard) TypeString(ctx context.Context, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	kbd.opMu.Lock()
	defer kbd.opMu.Unlock()

	for _, r := range normalized {
		if err := ctx.Err(); err != nil {
			return err
		}
		if isIgnoredFormatRune(r) {
			continue
		}

		kbd.mu.Lock()
		layout := kbd.layout
		kbd.mu.Unlock()

		ks, ok := layout.Runes[r]
		if !ok {
			continue
		}

		if err := kbd.tapCombo(ctx, ks.First); err != nil {
			return err
		}
		if ks.Second.KeyCode != 0 {
			if err := kbd.tapCombo(ctx, ks.Second); err != nil {
				return err
			}
		}
	}
	return nil
}
