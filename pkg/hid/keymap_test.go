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

func assertDeadKeyMapping(t *testing.T, layoutName string, l *Layout, ch rune, first, second KeyCombo) {
	t.Helper()
	ks, ok := l.Runes[ch]
	if !ok {
		t.Fatalf("%s missing dead-key rune %q", layoutName, ch)
	}
	if ks.First != first || ks.Second != second {
		t.Errorf("%s %q dead-key mismatch: got %+v, want %+v -> %+v",
			layoutName, ch, ks, first, second)
	}
}

func TestLayoutMappings(t *testing.T) {
	space := KeyCombo{KeyCode: KeySpace}

	de, _ := GetLayout("DE")
	assertRuneMapping(t, "DE", de, '?', ModLeftShift, KeyMinus)
	assertRuneMapping(t, "DE", de, '&', ModLeftShift, Key6)
	assertDeadKeyMapping(t, "DE", de, '^', KeyCombo{KeyCode: KeyGrave}, space)
	assertDeadKeyMapping(t, "DE", de, '`', KeyCombo{Modifiers: ModLeftShift, KeyCode: KeyEqual}, space)
	assertDeadKeyMapping(t, "DE", de, 'é', KeyCombo{KeyCode: KeyEqual}, KeyCombo{KeyCode: KeyE})

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
	assertDeadKeyMapping(t, "ES", es, '`', KeyCombo{KeyCode: KeyLeftBrace}, space)
	assertDeadKeyMapping(t, "ES", es, '^', KeyCombo{Modifiers: ModLeftShift, KeyCode: KeyLeftBrace}, space)
	assertDeadKeyMapping(t, "ES", es, '´', KeyCombo{KeyCode: KeyApostrophe}, space)
	assertDeadKeyMapping(t, "ES", es, '¨', KeyCombo{Modifiers: ModLeftShift, KeyCode: KeyApostrophe}, space)
	assertDeadKeyMapping(t, "ES", es, 'á', KeyCombo{KeyCode: KeyApostrophe}, KeyCombo{KeyCode: KeyA})
	assertDeadKeyMapping(t, "ES", es, 'Ó', KeyCombo{KeyCode: KeyApostrophe}, KeyCombo{Modifiers: ModLeftShift, KeyCode: KeyO})
	assertDeadKeyMapping(t, "ES", es, 'ü', KeyCombo{Modifiers: ModLeftShift, KeyCode: KeyApostrophe}, KeyCombo{KeyCode: KeyU})

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
	assertDeadKeyMapping(t, "FR", fr, '^', KeyCombo{KeyCode: KeyLeftBrace}, space)
	assertDeadKeyMapping(t, "FR", fr, '¨', KeyCombo{Modifiers: ModLeftShift, KeyCode: KeyLeftBrace}, space)
	assertDeadKeyMapping(t, "FR", fr, '~', KeyCombo{Modifiers: ModRightAlt, KeyCode: Key2}, space)
	assertDeadKeyMapping(t, "FR", fr, '`', KeyCombo{Modifiers: ModRightAlt, KeyCode: Key7}, space)
	assertDeadKeyMapping(t, "FR", fr, 'ê', KeyCombo{KeyCode: KeyLeftBrace}, KeyCombo{KeyCode: KeyE})
}

func TestMapKeyName(t *testing.T) {
	cases := []struct {
		layout string
		token  string
		want   KeyCombo
	}{
		{"ES", "CTRL", KeyCombo{Modifiers: ModLeftCtrl}},
		{"ES", "WIN", KeyCombo{Modifiers: ModLeftGUI}},
		{"ES", "BKSP", KeyCombo{KeyCode: KeyBackspace}},
		{"ES", "A", KeyCombo{KeyCode: KeyA}},
		{"ES", "ñ", KeyCombo{KeyCode: KeySemicolon}},
		{"ES", "Ñ", KeyCombo{KeyCode: KeySemicolon}},
		{"DE", "ß", KeyCombo{KeyCode: KeyMinus}},
		{"FR", "1", KeyCombo{KeyCode: Key1}},
		{"ES", "/", KeyCombo{KeyCode: Key7}},
	}

	for _, tc := range cases {
		l, _ := GetLayout(tc.layout)
		combo, ok := l.MapKeyName(tc.token)
		if !ok || combo != tc.want {
			t.Errorf("%s MapKeyName(%q) = (%+v, %v), want (%+v, true)", tc.layout, tc.token, combo, ok, tc.want)
		}
	}

	es, _ := GetLayout("ES")
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
