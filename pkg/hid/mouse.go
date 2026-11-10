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

func stepDelta(v int) int8 {
	if v > 127 {
		return 127
	}
	if v < -127 {
		return -127
	}
	return int8(v)
}

// MoveContext performs a relative mouse displacement, splitting steps larger than ±127 across multiple reports.
func (m *Mouse) MoveContext(ctx context.Context, dx, dy int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if dx == 0 && dy == 0 {
		report := [6]byte{0x01, m.buttons, 0, 0, 0, 0}
		return m.writeReport(ctx, report[:])
	}

	for dx != 0 || dy != 0 {
		sx := stepDelta(dx)
		sy := stepDelta(dy)
		report := [6]byte{0x01, m.buttons, uint8(sx), uint8(sy), 0, 0}
		if err := m.writeReport(ctx, report[:]); err != nil {
			return err
		}
		dx -= int(sx)
		dy -= int(sy)
	}
	return nil
}

// Move performs a single relative mouse displacement.
func (m *Mouse) Move(x, y int8) error {
	return m.MoveContext(context.Background(), int(x), int(y))
}

// MoveToContext sets an absolute mouse position (0..32767 on X and Y).
func (m *Mouse) MoveToContext(ctx context.Context, x, y uint16) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if x > MaxAbsCoord {
		x = MaxAbsCoord
	}
	if y > MaxAbsCoord {
		y = MaxAbsCoord
	}

	var report [6]byte
	report[0] = 0x02
	report[1] = m.buttons
	binary.LittleEndian.PutUint16(report[2:4], x)
	binary.LittleEndian.PutUint16(report[4:6], y)

	return m.writeReport(ctx, report[:])
}

// MoveTo sets an absolute mouse position (0..32767 on X and Y).
func (m *Mouse) MoveTo(x, y uint16) error {
	return m.MoveToContext(context.Background(), x, y)
}

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

func (m *Mouse) setButtonContext(ctx context.Context, button string, down bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	btn, err := parseButton(button)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	next := m.buttons
	if down {
		next |= btn
	} else {
		next &^= btn
	}

	report := [6]byte{0x01, next, 0, 0, 0, 0}
	if err := m.writeReport(ctx, report[:]); err != nil {
		return err
	}
	m.buttons = next
	return nil
}

// ButtonDownContext depresses a mouse button without releasing it.
func (m *Mouse) ButtonDownContext(ctx context.Context, button string) error {
	return m.setButtonContext(ctx, button, true)
}

// ButtonDown depresses a mouse button without releasing it.
func (m *Mouse) ButtonDown(button string) error {
	return m.setButtonContext(context.Background(), button, true)
}

// ButtonUpContext releases a mouse button.
func (m *Mouse) ButtonUpContext(ctx context.Context, button string) error {
	return m.setButtonContext(ctx, button, false)
}

// ButtonUp releases a mouse button.
func (m *Mouse) ButtonUp(button string) error {
	return m.setButtonContext(context.Background(), button, false)
}

// ReleaseButtons clears all depressed mouse buttons and emits a release report if any button was held.
func (m *Mouse) ReleaseButtons() error {
	m.mu.Lock()
	wasHeld := m.buttons != 0
	m.buttons = 0
	m.mu.Unlock()

	if !wasHeld {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), releaseCleanupTTL)
	defer cancel()
	report := [6]byte{0x01, 0, 0, 0, 0, 0}
	return m.writeReport(cleanupCtx, report[:])
}

// ClickContext presses and releases the specified mouse button, respecting context cancellation.
func (m *Mouse) ClickContext(ctx context.Context, button string) error {
	if err := m.ButtonDownContext(ctx, button); err != nil {
		return err
	}
	if err := sleepContext(ctx, 20*time.Millisecond); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), releaseCleanupTTL)
		defer cancel()
		_ = m.ButtonUpContext(cleanupCtx, button)
		return err
	}
	return m.ButtonUpContext(ctx, button)
}

// Click presses and releases the specified mouse button.
func (m *Mouse) Click(button string) error {
	return m.ClickContext(context.Background(), button)
}
