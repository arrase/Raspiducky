package hid

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Keyboard provides thread-safe access to a USB HID keyboard device (/dev/hidg0).
type Keyboard struct {
	mu               sync.Mutex
	hw               hidWriter
	layout           *Layout
	keyDelayMs       int
	keyDelayJitterMs int
}

// NewKeyboard initializes a new Keyboard connected to the given device path with the specified layout.
func NewKeyboard(devicePath, layoutName string) (*Keyboard, error) {
	layout, err := GetLayout(layoutName)
	if err != nil {
		return nil, fmt.Errorf("loading keyboard layout %q: %w", layoutName, err)
	}

	return &Keyboard{
		hw:         hidWriter{devicePath: devicePath},
		layout:     layout,
		keyDelayMs: 10,
	}, nil
}

// SetWriter overrides the output writer (useful for testing or custom pipes).
func (kbd *Keyboard) SetWriter(w io.Writer) {
	kbd.hw.setWriter(w)
}

// SetDevicePath updates the target HID device path.
func (kbd *Keyboard) SetDevicePath(path string) {
	kbd.hw.setDevicePath(path)
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

// WriteReport writes an 8-byte KeyboardReport to the device in a thread-safe manner.
func (kbd *Keyboard) WriteReport(report KeyboardReport) error {
	return kbd.hw.writeReport(report[:])
}

// ReleaseKeys sends an empty report to release all currently pressed keys.
func (kbd *Keyboard) ReleaseKeys() error {
	return kbd.WriteReport(KeyboardReport{})
}

// Press parses and executes a key combination string like "CTRL ALT DELETE" or "GUI R".
func (kbd *Keyboard) Press(ctx context.Context, comboStr string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tokens := strings.Fields(comboStr)
	if len(tokens) == 0 {
		return nil
	}

	kbd.mu.Lock()
	layout := kbd.layout
	kbd.mu.Unlock()

	var modifiers uint8
	var keycodes []uint8

	for _, token := range tokens {
		combo, ok := layout.MapKeyName(token)
		if !ok {
			return fmt.Errorf("unknown key token in press '%s': %s", comboStr, token)
		}
		modifiers |= combo.Modifiers
		if combo.KeyCode != 0 {
			keycodes = append(keycodes, combo.KeyCode)
		}
	}

	report := NewKeyboardReport(modifiers, keycodes...)
	if err := kbd.WriteReport(report); err != nil {
		return err
	}

	time.Sleep(10 * time.Millisecond)
	return kbd.ReleaseKeys()
}

// TypeString types out the given text string character by character according to the current layout and typing speed.
func (kbd *Keyboard) TypeString(ctx context.Context, text string) error {
	for _, r := range text {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		kbd.mu.Lock()
		layout := kbd.layout
		delay := kbd.keyDelayMs
		jitter := kbd.keyDelayJitterMs
		kbd.mu.Unlock()

		report, ok := layout.Runes[r]
		if !ok {
			return fmt.Errorf("unmapped rune %q in layout %s", r, layout.Name)
		}

		if err := kbd.WriteReport(report); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
		if err := kbd.ReleaseKeys(); err != nil {
			return err
		}

		// Apply inter-keystroke delay
		totalDelay := delay
		if jitter > 0 {
			totalDelay += rand.Intn(jitter)
		}
		if totalDelay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(totalDelay) * time.Millisecond):
			}
		}
	}
	return nil
}

// Close closes the underlying device file if owned by Keyboard.
func (kbd *Keyboard) Close() error {
	return kbd.hw.close()
}
