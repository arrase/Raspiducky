package hid

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestMouse_MoveAndButtons(t *testing.T) {
	m := NewMouse("")
	defer func() { _ = m.Close() }()

	var buf bytes.Buffer
	m.SetWriter(&buf)

	if err := m.Move(10, -5); err != nil {
		t.Fatalf("Move failed: %v", err)
	}
	rep := buf.Bytes()
	if len(rep) != 6 || rep[0] != 0x01 || rep[1] != 0 || int8(rep[2]) != 10 || int8(rep[3]) != -5 {
		t.Errorf("Unexpected relative move report: %v", rep)
	}

	buf.Reset()
	if err := m.MoveTo(1000, 2000); err != nil {
		t.Fatalf("MoveTo failed: %v", err)
	}
	rep = buf.Bytes()
	if len(rep) != 6 || rep[0] != 0x02 || binary.LittleEndian.Uint16(rep[2:4]) != 1000 || binary.LittleEndian.Uint16(rep[4:6]) != 2000 {
		t.Errorf("Unexpected absolute move report: %v", rep)
	}

	buf.Reset()
	if err := m.ButtonDown("left"); err != nil {
		t.Fatalf("ButtonDown failed: %v", err)
	}
	if buf.Bytes()[1] != MouseButtonLeft {
		t.Errorf("Expected left button bit set, got %#x", buf.Bytes()[1])
	}

	buf.Reset()
	if err := m.ButtonUp("left"); err != nil {
		t.Fatalf("ButtonUp failed: %v", err)
	}
	if buf.Bytes()[1] != 0 {
		t.Errorf("Expected left button bit cleared, got %#x", buf.Bytes()[1])
	}

	buf.Reset()
	if err := m.Click("right"); err != nil {
		t.Fatalf("Click failed: %v", err)
	}
	if len(buf.Bytes()) != 12 || buf.Bytes()[1] != MouseButtonRight || buf.Bytes()[7] != 0 {
		t.Errorf("Unexpected Click reports: %v", buf.Bytes())
	}
}

func TestMouse_SetDevicePath(t *testing.T) {
	tmpDir := t.TempDir()
	dev1 := filepath.Join(tmpDir, "hidg1_1")
	dev2 := filepath.Join(tmpDir, "hidg1_2")

	if err := os.WriteFile(dev1, nil, 0666); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dev2, nil, 0666); err != nil {
		t.Fatal(err)
	}

	m := NewMouse(dev1)
	defer func() { _ = m.Close() }()

	if err := m.Move(1, 2); err != nil {
		t.Fatalf("Move on dev1 failed: %v", err)
	}

	m.SetDevicePath(dev2)
	if err := m.Move(3, 4); err != nil {
		t.Fatalf("Move on dev2 failed: %v", err)
	}

	b1, _ := os.ReadFile(dev1)
	b2, _ := os.ReadFile(dev2)
	if len(b1) != 6 || b1[2] != 1 {
		t.Errorf("dev1 unexpected content: %v", b1)
	}
	if len(b2) != 6 || b2[2] != 3 {
		t.Errorf("dev2 unexpected content: %v", b2)
	}
}
