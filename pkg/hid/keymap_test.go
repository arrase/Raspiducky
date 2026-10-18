package hid

import (
	"testing"
)

func TestGetLayout(t *testing.T) {
	for _, name := range []string{"US", "us", "DE", "de", "ES", "es", "FR", "fr"} {
		l, err := GetLayout(name)
		if err != nil {
			t.Fatalf("GetLayout(%q) failed: %v", name, err)
		}
		if l == nil {
			t.Fatalf("GetLayout(%q) returned nil", name)
		}
	}

	if _, err := GetLayout("UNKNOWN"); err == nil {
		t.Error("Expected error for unsupported layout")
	}
}

func TestLayoutMappings(t *testing.T) {
	de, _ := GetLayout("DE")
	if r := de.Runes['?']; r[0] != ModLeftShift || r[2] != KeyMinus {
		t.Errorf("DE '?' mapping mismatch: got mod=%#x key=%#x", r[0], r[2])
	}
	if r := de.Runes['&']; r[0] != ModLeftShift || r[2] != Key6 {
		t.Errorf("DE '&' mapping mismatch: got mod=%#x key=%#x", r[0], r[2])
	}

	es, _ := GetLayout("ES")
	esCases := []struct {
		ch  rune
		mod uint8
		key uint8
	}{
		{'\'', 0, KeyMinus},
		{'?', ModLeftShift, KeyMinus},
		{'¡', 0, KeyEqual},
		{'¿', ModLeftShift, KeyEqual},
		{'{', ModRightAlt, KeyApostrophe},
		{'|', ModRightAlt, Key1},
	}
	for _, tc := range esCases {
		report, ok := es.Runes[tc.ch]
		if !ok {
			t.Fatalf("ES missing rune %q", tc.ch)
		}
		if report[0] != tc.mod || report[2] != tc.key {
			t.Errorf("ES %q mismatch: got mod=%#x key=%#x, want mod=%#x key=%#x",
				tc.ch, report[0], report[2], tc.mod, tc.key)
		}
	}

	fr, _ := GetLayout("FR")
	frCases := []struct {
		ch  rune
		mod uint8
		key uint8
	}{
		{']', ModRightAlt, KeyMinus},
		{'}', ModRightAlt, KeyEqual},
		{'=', 0, KeyEqual},
		{'+', ModLeftShift, KeyEqual},
	}
	for _, tc := range frCases {
		report, ok := fr.Runes[tc.ch]
		if !ok {
			t.Fatalf("FR missing rune %q", tc.ch)
		}
		if report[0] != tc.mod || report[2] != tc.key {
			t.Errorf("FR %q mismatch: got mod=%#x key=%#x, want mod=%#x key=%#x",
				tc.ch, report[0], report[2], tc.mod, tc.key)
		}
	}
}

func TestMapKeyName(t *testing.T) {
	es, _ := GetLayout("ES")

	if combo, ok := es.MapKeyName("CTRL"); !ok || combo.Modifiers != ModLeftCtrl || combo.KeyCode != 0 {
		t.Errorf("MapKeyName(CTRL) failed: %+v, ok=%v", combo, ok)
	}

	if combo, ok := es.MapKeyName("A"); !ok || combo.Modifiers != 0 || combo.KeyCode != KeyA {
		t.Errorf("MapKeyName(A) failed: %+v, ok=%v", combo, ok)
	}

	// Non-ASCII single rune in ES
	if combo, ok := es.MapKeyName("ñ"); !ok || combo.Modifiers != 0 || combo.KeyCode != KeySemicolon {
		t.Errorf("MapKeyName(ñ) failed: %+v, ok=%v", combo, ok)
	}
	if combo, ok := es.MapKeyName("Ñ"); !ok || combo.Modifiers != 0 || combo.KeyCode != KeySemicolon {
		t.Errorf("MapKeyName(Ñ) failed: %+v, ok=%v", combo, ok)
	}

	de, _ := GetLayout("DE")
	if combo, ok := de.MapKeyName("ß"); !ok || combo.Modifiers != 0 || combo.KeyCode != KeyMinus {
		t.Errorf("MapKeyName(ß) failed: %+v, ok=%v", combo, ok)
	}

	if _, ok := es.MapKeyName("NONEXISTENT_KEY"); ok {
		t.Error("Expected false for unknown key name")
	}
}
