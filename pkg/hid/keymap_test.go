package hid

import (
	"bytes"
	"context"
	"testing"
)

func TestLayoutTranslations(t *testing.T) {
	testCases := []struct {
		layout   string
		char     rune
		hasRune  bool
		expected uint8 // Expected main keycode
		mod      uint8 // Expected modifier
	}{
		// US Layout
		{"US", 'a', true, KeyA, 0},
		{"US", 'A', true, KeyA, ModLeftShift},
		{"US", '1', true, Key1, 0},
		{"US", '!', true, Key1, ModLeftShift},
		{"US", '@', true, Key2, ModLeftShift},

		// DE Layout (QWERTZ)
		{"DE", 'z', true, KeyY, 0},
		{"DE", 'y', true, KeyZ, 0},
		{"DE", 'ä', true, KeyApostrophe, 0},
		{"DE", 'ö', true, KeySemicolon, 0},
		{"DE", 'ü', true, KeyLeftBrace, 0},
		{"DE", 'ß', true, KeyMinus, 0},
		{"DE", '@', true, KeyQ, ModRightAlt},

		// ES Layout
		{"ES", 'ñ', true, KeySemicolon, 0},
		{"ES", 'Ñ', true, KeySemicolon, ModLeftShift},
		{"ES", 'ç', true, KeyRightBrace, 0},
		{"ES", '@', true, Key2, ModRightAlt},
		{"ES", '€', true, KeyE, ModRightAlt},

		// FR Layout (AZERTY)
		{"FR", 'a', true, KeyQ, 0},
		{"FR", 'z', true, KeyW, 0},
		{"FR", 'q', true, KeyA, 0},
		{"FR", 'w', true, KeyZ, 0},
		{"FR", 'm', true, KeySemicolon, 0},
		{"FR", '1', true, Key1, ModLeftShift},
		{"FR", '&', true, Key1, 0},
		{"FR", 'é', true, Key2, 0},
	}

	for _, tc := range testCases {
		l, err := GetLayout(tc.layout)
		if err != nil {
			t.Fatalf("Failed to get layout %s: %v", tc.layout, err)
		}

		reports, found := l.MapRune(tc.char)
		if found != tc.hasRune {
			t.Errorf("Layout %s MapRune(%q) found=%v, expected %v", tc.layout, tc.char, found, tc.hasRune)
			continue
		}

		if found && len(reports) > 0 {
			r := reports[0]
			if r.Modifiers() != tc.mod {
				t.Errorf("Layout %s MapRune(%q) modifier=0x%02x, expected 0x%02x", tc.layout, tc.char, r.Modifiers(), tc.mod)
			}
			keys := r.KeyCodes()
			if len(keys) == 0 || keys[0] != tc.expected {
				t.Errorf("Layout %s MapRune(%q) keycode=0x%02x, expected 0x%02x", tc.layout, tc.char, keys, tc.expected)
			}
		}
	}
}

func TestKeyComboParsing(t *testing.T) {
	l, err := GetLayout("US")
	if err != nil {
		t.Fatalf("GetLayout failed: %v", err)
	}

	combo, ok := l.MapKeyName("CTRL")
	if !ok || combo.Modifiers != ModLeftCtrl {
		t.Errorf("MapKeyName('CTRL') failed, got %+v", combo)
	}

	combo, ok = l.MapKeyName("ENTER")
	if !ok || combo.KeyCode != KeyEnter {
		t.Errorf("MapKeyName('ENTER') failed, got %+v", combo)
	}
}

func TestKeyboardWriting(t *testing.T) {
	buf := &bytes.Buffer{}
	kbd, err := NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	kbd.SetWriter(buf)

	err = kbd.TypeString(context.Background(), "hi")
	if err != nil {
		t.Fatalf("TypeString failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Errorf("Expected output written to buffer, got 0 bytes")
	}
}

func TestMouseWriting(t *testing.T) {
	buf := &bytes.Buffer{}
	mouse, err := NewMouse("")
	if err != nil {
		t.Fatalf("NewMouse failed: %v", err)
	}
	mouse.SetWriter(buf)

	err = mouse.Move(10, -5)
	if err != nil {
		t.Fatalf("Mouse.Move failed: %v", err)
	}

	if buf.Len() != 6 {
		t.Errorf("Expected 6 bytes mouse report, got %d bytes", buf.Len())
	}
}
