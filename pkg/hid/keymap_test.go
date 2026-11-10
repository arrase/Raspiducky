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
	ks, ok := l.Runes[ch]
	if !ok {
		t.Fatalf("%s missing rune %q", layoutName, ch)
	}
	if ks.First.Modifiers != wantMod || ks.First.KeyCode != wantKey {
		t.Errorf("%s %q mismatch: got mod=%#x key=%#x, want mod=%#x key=%#x",
			layoutName, ch, ks.First.Modifiers, ks.First.KeyCode, wantMod, wantKey)
	}
}

func assertDeadKeyMapping(t *testing.T, layoutName string, l *Layout, ch rune, deadMod, deadKey, nextMod, nextKey uint8) {
	t.Helper()
	ks, ok := l.Runes[ch]
	if !ok {
		t.Fatalf("%s missing dead-key rune %q", layoutName, ch)
	}
	if ks.First.Modifiers != deadMod || ks.First.KeyCode != deadKey || ks.Second.Modifiers != nextMod || ks.Second.KeyCode != nextKey {
		t.Errorf("%s %q dead-key mismatch: got %+v, want (%#x,%#x)->(%#x,%#x)",
			layoutName, ch, ks, deadMod, deadKey, nextMod, nextKey)
	}
}

func TestLayoutMappings(t *testing.T) {
	de, _ := GetLayout("DE")
	assertRuneMapping(t, "DE", de, '?', ModLeftShift, KeyMinus)
	assertRuneMapping(t, "DE", de, '&', ModLeftShift, Key6)
	assertDeadKeyMapping(t, "DE", de, '^', 0, KeyGrave, 0, KeySpace)
	assertDeadKeyMapping(t, "DE", de, '`', ModLeftShift, KeyEqual, 0, KeySpace)
	assertDeadKeyMapping(t, "DE", de, 'é', 0, KeyEqual, 0, KeyE)

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
		{'º', 0, KeyGrave},
		{'ª', ModLeftShift, KeyGrave},
		{'[', ModRightAlt, KeyLeftBrace},
		{'+', 0, KeyRightBrace},
		{'*', ModLeftShift, KeyRightBrace},
		{']', ModRightAlt, KeyRightBrace},
		{'ç', 0, KeyHashtilde},
		{'Ç', ModLeftShift, KeyHashtilde},
		{'{', ModRightAlt, KeyApostrophe},
		{'}', ModRightAlt, KeyHashtilde},
		{'|', ModRightAlt, Key1},
	}
	for _, tc := range esCases {
		assertRuneMapping(t, "ES", es, tc.ch, tc.mod, tc.key)
	}
	assertDeadKeyMapping(t, "ES", es, '`', 0, KeyLeftBrace, 0, KeySpace)
	assertDeadKeyMapping(t, "ES", es, '^', ModLeftShift, KeyLeftBrace, 0, KeySpace)
	assertDeadKeyMapping(t, "ES", es, '´', 0, KeyApostrophe, 0, KeySpace)
	assertDeadKeyMapping(t, "ES", es, '¨', ModLeftShift, KeyApostrophe, 0, KeySpace)
	assertDeadKeyMapping(t, "ES", es, 'á', 0, KeyApostrophe, 0, KeyA)
	assertDeadKeyMapping(t, "ES", es, 'Ó', 0, KeyApostrophe, ModLeftShift, KeyO)
	assertDeadKeyMapping(t, "ES", es, 'ü', ModLeftShift, KeyApostrophe, 0, KeyU)

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
	assertDeadKeyMapping(t, "FR", fr, '^', 0, KeyLeftBrace, 0, KeySpace)
	assertDeadKeyMapping(t, "FR", fr, '¨', ModLeftShift, KeyLeftBrace, 0, KeySpace)
	assertDeadKeyMapping(t, "FR", fr, '~', ModRightAlt, Key2, 0, KeySpace)
	assertDeadKeyMapping(t, "FR", fr, '`', ModRightAlt, Key7, 0, KeySpace)
	assertDeadKeyMapping(t, "FR", fr, 'ê', 0, KeyLeftBrace, 0, KeyE)
}

func TestMapKeyName(t *testing.T) {
	es, _ := GetLayout("ES")

	if combo, ok := es.MapKeyName("CTRL"); !ok || combo.Modifiers != ModLeftCtrl || combo.KeyCode != 0 {
		t.Errorf("MapKeyName(CTRL) failed: %+v, ok=%v", combo, ok)
	}
	if combo, ok := es.MapKeyName("WIN"); !ok || combo.Modifiers != ModLeftGUI || combo.KeyCode != 0 {
		t.Errorf("MapKeyName(WIN) failed: %+v, ok=%v", combo, ok)
	}
	if combo, ok := es.MapKeyName("BKSP"); !ok || combo.KeyCode != KeyBackspace {
		t.Errorf("MapKeyName(BKSP) failed: %+v, ok=%v", combo, ok)
	}

	if combo, ok := es.MapKeyName("A"); !ok || combo.Modifiers != 0 || combo.KeyCode != KeyA {
		t.Errorf("MapKeyName(A) failed: %+v, ok=%v", combo, ok)
	}

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

	fr, _ := GetLayout("FR")
	if combo, ok := fr.MapKeyName("1"); !ok || combo.Modifiers != 0 || combo.KeyCode != Key1 {
		t.Errorf("FR MapKeyName(1) should not inject Shift: %+v, ok=%v", combo, ok)
	}
	if combo, ok := es.MapKeyName("/"); !ok || combo.Modifiers != 0 || combo.KeyCode != Key7 {
		t.Errorf("ES MapKeyName(/) should not inject Shift: %+v, ok=%v", combo, ok)
	}

	if _, ok := es.MapKeyName("NONEXISTENT_KEY"); ok {
		t.Error("Expected false for unknown key name")
	}
}

func TestSmartQuoteAndUnicodeAliases(t *testing.T) {
	singleQuotes := []rune{'\u2018', '\u2019', '\u201a', '\u201b'}
	doubleQuotes := []rune{'\u201c', '\u201d', '\u201e', '\u201f', '\u00ab', '\u00bb'}
	dashes := []rune{'\u2013', '\u2014', '\u2212'}

	for _, name := range []string{"US", "DE", "ES", "FR"} {
		l, err := GetLayout(name)
		if err != nil {
			t.Fatalf("GetLayout(%q) failed: %v", name, err)
		}

		wantSingle := l.Runes['\'']
		for _, q := range singleQuotes {
			assertRuneMapping(t, name, l, q, wantSingle.First.Modifiers, wantSingle.First.KeyCode)
		}

		wantDouble := l.Runes['"']
		for _, q := range doubleQuotes {
			assertRuneMapping(t, name, l, q, wantDouble.First.Modifiers, wantDouble.First.KeyCode)
		}

		wantDash := l.Runes['-']
		for _, d := range dashes {
			assertRuneMapping(t, name, l, d, wantDash.First.Modifiers, wantDash.First.KeyCode)
		}

		assertRuneMapping(t, name, l, '\u00a0', 0, KeySpace)
	}
}
