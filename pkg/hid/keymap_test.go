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

func assertRuneMapping(t *testing.T, layoutName string, l *Layout, ch rune, wantMod, wantKey uint8) {
	t.Helper()
	report, ok := l.Runes[ch]
	if !ok {
		t.Fatalf("%s missing rune %q", layoutName, ch)
	}
	if report[0] != wantMod || report[2] != wantKey {
		t.Errorf("%s %q mismatch: got mod=%#x key=%#x, want mod=%#x key=%#x",
			layoutName, ch, report[0], report[2], wantMod, wantKey)
	}
}

func TestLayoutMappings(t *testing.T) {
	de, _ := GetLayout("DE")
	assertRuneMapping(t, "DE", de, '?', ModLeftShift, KeyMinus)
	assertRuneMapping(t, "DE", de, '&', ModLeftShift, Key6)

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
		assertRuneMapping(t, "ES", es, tc.ch, tc.mod, tc.key)
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
		assertRuneMapping(t, "FR", fr, tc.ch, tc.mod, tc.key)
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

func TestSmartQuoteMappings(t *testing.T) {
	singleQuotes := []rune{'\u2018', '\u2019', '\u201a', '\u201b'}
	doubleQuotes := []rune{'\u201c', '\u201d', '\u201e', '\u201f', '\u00ab', '\u00bb'}

	for _, name := range []string{"US", "DE", "ES", "FR"} {
		l, err := GetLayout(name)
		if err != nil {
			t.Fatalf("GetLayout(%q) failed: %v", name, err)
		}

		wantSingle := l.Runes['\'']
		for _, q := range singleQuotes {
			assertRuneMapping(t, name, l, q, wantSingle[0], wantSingle[2])
		}

		wantDouble := l.Runes['"']
		for _, q := range doubleQuotes {
			assertRuneMapping(t, name, l, q, wantDouble[0], wantDouble[2])
		}
	}
}
