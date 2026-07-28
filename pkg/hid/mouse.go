package hid

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	MouseButtonLeft   uint8 = 0x01
	MouseButtonRight  uint8 = 0x02
	MouseButtonMiddle uint8 = 0x04
)

// Mouse provides thread-safe control over a USB HID mouse device (/dev/hidg1).
type Mouse struct {
	mu         sync.Mutex
	devicePath string
	writer     io.Writer
	ownsWriter bool
	buttons    uint8
}

// NewMouse initializes a new Mouse connected to the specified device path.
func NewMouse(devicePath string) (*Mouse, error) {
	var writer io.Writer
	var ownsWriter bool
	if devicePath != "" {
		f, err := os.OpenFile(devicePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err == nil {
			writer = f
			ownsWriter = true
		}
	}

	return &Mouse{
		devicePath: devicePath,
		writer:     writer,
		ownsWriter: ownsWriter,
	}, nil
}

// SetWriter overrides the output writer (useful for testing or custom pipes).
func (m *Mouse) SetWriter(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ownsWriter && m.writer != nil {
		if closer, ok := m.writer.(io.Closer); ok {
			_ = closer.Close()
		}
	}
	m.writer = w
	m.ownsWriter = false
}

func (m *Mouse) writeReport(report []byte) error {
	if m.writer == nil {
		return nil
	}
	n, err := m.writer.Write(report)
	if err != nil {
		return fmt.Errorf("hid mouse write error: %w", err)
	}
	if n < len(report) {
		return io.ErrShortWrite
	}
	return nil
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
	report[4] = 0
	report[5] = 0

	return m.writeReport(report[:])
}

// MoveTo sets an absolute mouse position (0..32767 on X and Y).
func (m *Mouse) MoveTo(x, y uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var report [6]byte
	report[0] = 0x02 // Absolute report ID
	report[1] = m.buttons
	binary.LittleEndian.PutUint16(report[2:4], x)
	binary.LittleEndian.PutUint16(report[4:6], y)

	return m.writeReport(report[:])
}

// parseButton converts a string button representation to its bitmask value.
func parseButton(b string) uint8 {
	lower := strings.ToLower(strings.TrimSpace(b))
	switch lower {
	case "left", "1", "b1", "button1", "":
		return MouseButtonLeft
	case "right", "2", "b2", "button2":
		return MouseButtonRight
	case "middle", "3", "b3", "button3":
		return MouseButtonMiddle
	}
	return MouseButtonLeft
}

// ButtonDown depresses a mouse button without releasing it.
func (m *Mouse) ButtonDown(button string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	btn := parseButton(button)
	m.buttons |= btn

	var report [6]byte
	report[0] = 0x01
	report[1] = m.buttons

	return m.writeReport(report[:])
}

// ButtonUp releases a mouse button.
func (m *Mouse) ButtonUp(button string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	btn := parseButton(button)
	m.buttons &= ^btn

	var report [6]byte
	report[0] = 0x01
	report[1] = m.buttons

	return m.writeReport(report[:])
}

// Click presses and releases the specified mouse button.
func (m *Mouse) Click(button string) error {
	if err := m.ButtonDown(button); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	return m.ButtonUp(button)
}

// Close closes the underlying device file if owned by Mouse.
func (m *Mouse) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ownsWriter && m.writer != nil {
		if closer, ok := m.writer.(io.Closer); ok {
			err := closer.Close()
			m.writer = nil
			return err
		}
	}
	return nil
}
