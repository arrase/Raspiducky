package hid

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"
)

// USB HID mouse button bitmasks and coordinate bounds.
const (
	MouseButtonLeft   uint8 = 0x01
	MouseButtonRight  uint8 = 0x02
	MouseButtonMiddle uint8 = 0x04
	MaxAbsCoord             = 32767
)

// Mouse provides thread-safe control over a USB HID mouse device (/dev/hidg1).
type Mouse struct {
	hidWriter
	mu      sync.Mutex
	buttons uint8
}

// NewMouse initializes a new Mouse connected to the specified device path.
func NewMouse(devicePath string) *Mouse {
	return &Mouse{
		hidWriter: hidWriter{devicePath: devicePath},
	}
}

// Move performs a relative mouse displacement.
func (m *Mouse) Move(x, y int8) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var report [6]byte
	report[0] = 0x01 // Relative report ID
	report[1] = m.buttons
	report[2] = uint8(x)
	report[3] = uint8(y)

	return m.writeReport(report[:])
}

// MoveTo sets an absolute mouse position (0..32767 on X and Y).
func (m *Mouse) MoveTo(x, y uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if x > MaxAbsCoord {
		x = MaxAbsCoord
	}
	if y > MaxAbsCoord {
		y = MaxAbsCoord
	}

	var report [6]byte
	report[0] = 0x02 // Absolute report ID
	report[1] = m.buttons
	binary.LittleEndian.PutUint16(report[2:4], x)
	binary.LittleEndian.PutUint16(report[4:6], y)

	return m.writeReport(report[:])
}

// parseButton converts a string button representation to its bitmask value.
func parseButton(b string) (uint8, error) {
	lower := strings.ToLower(strings.TrimSpace(b))
	switch lower {
	case "left", "1", "b1", "button1", "":
		return MouseButtonLeft, nil
	case "right", "2", "b2", "button2":
		return MouseButtonRight, nil
	case "middle", "3", "b3", "button3":
		return MouseButtonMiddle, nil
	default:
		return 0, fmt.Errorf("unrecognized mouse button %q", b)
	}
}

func (m *Mouse) setButton(button string, down bool) error {
	btn, err := parseButton(button)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if down {
		m.buttons |= btn
	} else {
		m.buttons &^= btn
	}

	var report [6]byte
	report[0] = 0x01
	report[1] = m.buttons

	return m.writeReport(report[:])
}

// ButtonDown depresses a mouse button without releasing it.
func (m *Mouse) ButtonDown(button string) error {
	return m.setButton(button, true)
}

// ButtonUp releases a mouse button.
func (m *Mouse) ButtonUp(button string) error {
	return m.setButton(button, false)
}

// ClickContext presses and releases the specified mouse button, respecting context cancellation.
func (m *Mouse) ClickContext(ctx context.Context, button string) error {
	if err := m.ButtonDown(button); err != nil {
		return err
	}
	if err := sleepContext(ctx, 20*time.Millisecond); err != nil {
		_ = m.ButtonUp(button)
		return err
	}
	return m.ButtonUp(button)
}

// Click presses and releases the specified mouse button.
func (m *Mouse) Click(button string) error {
	return m.ClickContext(context.Background(), button)
}
