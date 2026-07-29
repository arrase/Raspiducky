package hid

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

// Keyboard provides thread-safe access to a USB HID keyboard device (/dev/hidg0).
type Keyboard struct {
	mu               sync.Mutex
	devicePath       string
	writer           io.Writer
	ownsWriter       bool
	activeLayout     *Layout
	keyDelayMs       int
	keyDelayJitterMs int
}

// NewKeyboard initializes a new Keyboard connected to the given device path with the specified layout.
func NewKeyboard(devicePath string, layoutName string) (*Keyboard, error) {
	layout, err := GetLayout(layoutName)
	if err != nil {
		return nil, fmt.Errorf("loading keyboard layout %q: %w", layoutName, err)
	}

	var writer io.Writer
	ownsWriter := false
	if devicePath != "" {
		ownsWriter = true
	}

	kbd := &Keyboard{
		devicePath:       devicePath,
		writer:           writer,
		ownsWriter:       ownsWriter,
		activeLayout:     layout,
		keyDelayMs:       10,
		keyDelayJitterMs: 0,
	}

	return kbd, nil
}

// SetWriter overrides the output writer (useful for testing or custom pipes).
func (kbd *Keyboard) SetWriter(w io.Writer) {
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	if kbd.ownsWriter && kbd.writer != nil {
		if closer, ok := kbd.writer.(io.Closer); ok {
			_ = closer.Close()
		}
	}
	kbd.writer = w
	kbd.ownsWriter = false
}

// SetLayout changes the active keyboard layout.
func (kbd *Keyboard) SetLayout(layoutName string) error {
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	layout, err := GetLayout(layoutName)
	if err != nil {
		return err
	}
	kbd.activeLayout = layout
	return nil
}

// GetLayoutName returns the name of the current active layout.
func (kbd *Keyboard) GetLayoutName() string {
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	return kbd.activeLayout.Name
}

// SetTypingSpeed sets inter-keystroke delay and optional random jitter in milliseconds.
func (kbd *Keyboard) SetTypingSpeed(delayMs int, jitterMs int) {
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
	kbd.mu.Lock()
	defer kbd.mu.Unlock()

	if kbd.writer == nil && kbd.ownsWriter && kbd.devicePath != "" {
		f, err := os.OpenFile(kbd.devicePath, os.O_WRONLY|os.O_SYNC, 0666)
		if err != nil {
			return fmt.Errorf("opening device %q: %w", kbd.devicePath, err)
		}
		kbd.writer = f
	}

	if kbd.writer == nil {
		return errors.New("keyboard device not connected")
	}

	data := report[:]
	n, err := kbd.writer.Write(data)
	if err != nil {
		if kbd.ownsWriter {
			if closer, ok := kbd.writer.(io.Closer); ok {
				_ = closer.Close()
			}
			kbd.writer = nil // Force reopen on next write
		}
		return fmt.Errorf("hid keyboard write error: %w", err)
	}
	if n < len(data) {
		return io.ErrShortWrite
	}
	return nil
}

// ReleaseKeys sends an empty report to release all currently pressed keys.
func (kbd *Keyboard) ReleaseKeys() error {
	return kbd.WriteReport(EmptyKeyboardReport())
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
	layout := kbd.activeLayout
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
		layout := kbd.activeLayout
		delay := kbd.keyDelayMs
		jitter := kbd.keyDelayJitterMs
		kbd.mu.Unlock()

		reports, ok := layout.MapRune(r)
		if !ok {
			return fmt.Errorf("unmapped rune %q in layout %s", r, layout.Name)
		}

		for _, report := range reports {
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
	}
	return nil
}

// Close closes the underlying device file if owned by Keyboard.
func (kbd *Keyboard) Close() error {
	kbd.mu.Lock()
	defer kbd.mu.Unlock()
	if kbd.ownsWriter && kbd.writer != nil {
		if closer, ok := kbd.writer.(io.Closer); ok {
			err := closer.Close()
			kbd.writer = nil
			return err
		}
	}
	return nil
}
