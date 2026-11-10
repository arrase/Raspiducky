package hid

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// KeyCombo represents a modifier bitmask and a main keycode.
type KeyCombo struct {
	Modifiers uint8
	KeyCode   uint8
}

// KeyStroke represents up to two sequential key combinations (supporting dead-key sequences).
type KeyStroke struct {
	First  KeyCombo
	Second KeyCombo
}

func stroke(mod, key uint8) KeyStroke {
	return KeyStroke{First: KeyCombo{Modifiers: mod, KeyCode: key}}
}

func deadStroke(deadMod, deadKey, nextMod, nextKey uint8) KeyStroke {
	return KeyStroke{
		First:  KeyCombo{Modifiers: deadMod, KeyCode: deadKey},
		Second: KeyCombo{Modifiers: nextMod, KeyCode: nextKey},
	}
}

// Layout defines mapping rules for translating runes and key names into HID keystrokes.
type Layout struct {
	Name  string
	Runes map[rune]KeyStroke
}

var commonKeyNames = map[string]KeyCombo{
	"CTRL":        {Modifiers: ModLeftCtrl, KeyCode: 0},
	"CONTROL":     {Modifiers: ModLeftCtrl, KeyCode: 0},
	"LEFTCTRL":    {Modifiers: ModLeftCtrl, KeyCode: 0},
	"RIGHTCTRL":   {Modifiers: ModRightCtrl, KeyCode: 0},
	"SHIFT":       {Modifiers: ModLeftShift, KeyCode: 0},
	"LEFTSHIFT":   {Modifiers: ModLeftShift, KeyCode: 0},
	"RIGHTSHIFT":  {Modifiers: ModRightShift, KeyCode: 0},
	"ALT":         {Modifiers: ModLeftAlt, KeyCode: 0},
	"OPTION":      {Modifiers: ModLeftAlt, KeyCode: 0},
	"LEFTALT":     {Modifiers: ModLeftAlt, KeyCode: 0},
	"RIGHTALT":    {Modifiers: ModRightAlt, KeyCode: 0},
	"ALTGR":       {Modifiers: ModRightAlt, KeyCode: 0},
	"GUI":         {Modifiers: ModLeftGUI, KeyCode: 0},
	"WIN":         {Modifiers: ModLeftGUI, KeyCode: 0},
	"WINDOWS":     {Modifiers: ModLeftGUI, KeyCode: 0},
	"CMD":         {Modifiers: ModLeftGUI, KeyCode: 0},
	"COMMAND":     {Modifiers: ModLeftGUI, KeyCode: 0},
	"META":        {Modifiers: ModLeftGUI, KeyCode: 0},
	"SUPER":       {Modifiers: ModLeftGUI, KeyCode: 0},
	"LEFTGUI":     {Modifiers: ModLeftGUI, KeyCode: 0},
	"RIGHTGUI":    {Modifiers: ModRightGUI, KeyCode: 0},
	"ENTER":       {Modifiers: 0, KeyCode: KeyEnter},
	"RETURN":      {Modifiers: 0, KeyCode: KeyEnter},
	"KP_ENTER":    {Modifiers: 0, KeyCode: KeyKpEnter},
	"ESC":         {Modifiers: 0, KeyCode: KeyEsc},
	"ESCAPE":      {Modifiers: 0, KeyCode: KeyEsc},
	"BACKSPACE":   {Modifiers: 0, KeyCode: KeyBackspace},
	"BKSP":        {Modifiers: 0, KeyCode: KeyBackspace},
	"TAB":         {Modifiers: 0, KeyCode: KeyTab},
	"SPACE":       {Modifiers: 0, KeyCode: KeySpace},
	"DELETE":      {Modifiers: 0, KeyCode: KeyDelete},
	"DEL":         {Modifiers: 0, KeyCode: KeyDelete},
	"INSERT":      {Modifiers: 0, KeyCode: KeyInsert},
	"INS":         {Modifiers: 0, KeyCode: KeyInsert},
	"HOME":        {Modifiers: 0, KeyCode: KeyHome},
	"END":         {Modifiers: 0, KeyCode: KeyEnd},
	"PAGEUP":      {Modifiers: 0, KeyCode: KeyPageUp},
	"PGUP":        {Modifiers: 0, KeyCode: KeyPageUp},
	"PAGEDOWN":    {Modifiers: 0, KeyCode: KeyPageDown},
	"PGDN":        {Modifiers: 0, KeyCode: KeyPageDown},
	"UP":          {Modifiers: 0, KeyCode: KeyUp},
	"UPARROW":     {Modifiers: 0, KeyCode: KeyUp},
	"DOWN":        {Modifiers: 0, KeyCode: KeyDown},
	"DOWNARROW":   {Modifiers: 0, KeyCode: KeyDown},
	"LEFT":        {Modifiers: 0, KeyCode: KeyLeft},
	"LEFTARROW":   {Modifiers: 0, KeyCode: KeyLeft},
	"RIGHT":       {Modifiers: 0, KeyCode: KeyRight},
	"RIGHTARROW":  {Modifiers: 0, KeyCode: KeyRight},
	"CAPSLOCK":    {Modifiers: 0, KeyCode: KeyCapsLock},
	"CAPS":        {Modifiers: 0, KeyCode: KeyCapsLock},
	"NUMLOCK":     {Modifiers: 0, KeyCode: KeyNumLock},
	"SCROLLLOCK":  {Modifiers: 0, KeyCode: KeyScrollLock},
	"PRINTSCREEN": {Modifiers: 0, KeyCode: KeyPrintScreen},
	"PRINTSCRN":   {Modifiers: 0, KeyCode: KeyPrintScreen},
	"PRTSC":       {Modifiers: 0, KeyCode: KeyPrintScreen},
	"SYSRQ":       {Modifiers: 0, KeyCode: KeyPrintScreen},
	"PAUSE":       {Modifiers: 0, KeyCode: KeyPause},
	"BREAK":       {Modifiers: 0, KeyCode: KeyPause},
	"MENU":        {Modifiers: 0, KeyCode: KeyApplication},
	"APP":         {Modifiers: 0, KeyCode: KeyApplication},
	"F1":          {Modifiers: 0, KeyCode: KeyF1},
	"F2":          {Modifiers: 0, KeyCode: KeyF2},
	"F3":          {Modifiers: 0, KeyCode: KeyF3},
	"F4":          {Modifiers: 0, KeyCode: KeyF4},
	"F5":          {Modifiers: 0, KeyCode: KeyF5},
	"F6":          {Modifiers: 0, KeyCode: KeyF6},
	"F7":          {Modifiers: 0, KeyCode: KeyF7},
	"F8":          {Modifiers: 0, KeyCode: KeyF8},
	"F9":          {Modifiers: 0, KeyCode: KeyF9},
	"F10":         {Modifiers: 0, KeyCode: KeyF10},
	"F11":         {Modifiers: 0, KeyCode: KeyF11},
	"F12":         {Modifiers: 0, KeyCode: KeyF12},
}

var layouts = map[string]*Layout{}

func init() {
	layouts["US"] = buildUSLayout()
	layouts["DE"] = buildDELayout()
	layouts["ES"] = buildESLayout()
	layouts["FR"] = buildFRLayout()
	for _, l := range layouts {
		aliasCommonUnicode(l)
	}
}

func aliasCommonUnicode(l *Layout) {
	for _, q := range []rune{'\u2018', '\u2019', '\u201a', '\u201b'} {
		l.Runes[q] = l.Runes['\'']
	}
	for _, q := range []rune{'\u201c', '\u201d', '\u201e', '\u201f', '\u00ab', '\u00bb'} {
		l.Runes[q] = l.Runes['"']
	}
	for _, d := range []rune{'\u2013', '\u2014', '\u2212'} {
		l.Runes[d] = l.Runes['-']
	}
	l.Runes['\u00a0'] = l.Runes[' ']
}

func addDeadAccentVowels(l *Layout, deadMod, deadKey uint8, lower [5]rune, upper [5]rune) {
	vowels := [5]rune{'a', 'e', 'i', 'o', 'u'}
	upperVowels := [5]rune{'A', 'E', 'I', 'O', 'U'}
	for i := range vowels {
		if lower[i] != 0 {
			if _, exists := l.Runes[lower[i]]; !exists {
				base := l.Runes[vowels[i]].First
				l.Runes[lower[i]] = deadStroke(deadMod, deadKey, base.Modifiers, base.KeyCode)
			}
		}
		if upper[i] != 0 {
			if _, exists := l.Runes[upper[i]]; !exists {
				base := l.Runes[upperVowels[i]].First
				l.Runes[upper[i]] = deadStroke(deadMod, deadKey, base.Modifiers, base.KeyCode)
			}
		}
	}
}

// GetLayout returns the Layout struct for the given layout name ("US", "DE", "ES", "FR").
func GetLayout(name string) (*Layout, error) {
	upper := strings.ToUpper(strings.TrimSpace(name))
	if l, ok := layouts[upper]; ok {
		return l, nil
	}
	return nil, fmt.Errorf("unsupported layout: %s", name)
}

// MapKeyName resolves a key name (e.g. "CTRL", "GUI", "a", "ENTER", "F4") to a KeyCombo in this layout.
func (l *Layout) MapKeyName(name string) (KeyCombo, bool) {
	trimmed := strings.TrimSpace(name)
	upper := strings.ToUpper(trimmed)
	if combo, ok := commonKeyNames[upper]; ok {
		return combo, true
	}

	if utf8.RuneCountInString(trimmed) == 1 {
		r, _ := utf8.DecodeRuneInString(trimmed)
		ks, ok := l.Runes[unicode.ToLower(r)]
		if !ok {
			ks, ok = l.Runes[r]
		}
		if ok && ks.First.KeyCode != 0 && ks.Second.KeyCode == 0 {
			return KeyCombo{Modifiers: 0, KeyCode: ks.First.KeyCode}, true
		}
	}

	return KeyCombo{}, false
}

func buildUSLayout() *Layout {
	rMap := make(map[rune]KeyStroke, 128)

	for c := 'a'; c <= 'z'; c++ {
		key := KeyA + uint8(c-'a')
		rMap[c] = stroke(0, key)
		rMap[unicode.ToUpper(c)] = stroke(ModLeftShift, key)
	}

	digits := []struct {
		unshifted rune
		shifted   rune
		key       uint8
	}{
		{'1', '!', Key1},
		{'2', '@', Key2},
		{'3', '#', Key3},
		{'4', '$', Key4},
		{'5', '%', Key5},
		{'6', '^', Key6},
		{'7', '&', Key7},
		{'8', '*', Key8},
		{'9', '(', Key9},
		{'0', ')', Key0},
	}
	for _, d := range digits {
		rMap[d.unshifted] = stroke(0, d.key)
		rMap[d.shifted] = stroke(ModLeftShift, d.key)
	}

	rMap['\n'] = stroke(0, KeyEnter)
	rMap['\r'] = stroke(0, KeyEnter)
	rMap['\t'] = stroke(0, KeyTab)
	rMap[' '] = stroke(0, KeySpace)

	rMap['-'] = stroke(0, KeyMinus)
	rMap['_'] = stroke(ModLeftShift, KeyMinus)
	rMap['='] = stroke(0, KeyEqual)
	rMap['+'] = stroke(ModLeftShift, KeyEqual)
	rMap['['] = stroke(0, KeyLeftBrace)
	rMap['{'] = stroke(ModLeftShift, KeyLeftBrace)
	rMap[']'] = stroke(0, KeyRightBrace)
	rMap['}'] = stroke(ModLeftShift, KeyRightBrace)
	rMap['\\'] = stroke(0, KeyBackslash)
	rMap['|'] = stroke(ModLeftShift, KeyBackslash)
	rMap[';'] = stroke(0, KeySemicolon)
	rMap[':'] = stroke(ModLeftShift, KeySemicolon)
	rMap['\''] = stroke(0, KeyApostrophe)
	rMap['"'] = stroke(ModLeftShift, KeyApostrophe)
	rMap['`'] = stroke(0, KeyGrave)
	rMap['~'] = stroke(ModLeftShift, KeyGrave)
	rMap[','] = stroke(0, KeyComma)
	rMap['<'] = stroke(ModLeftShift, KeyComma)
	rMap['.'] = stroke(0, KeyDot)
	rMap['>'] = stroke(ModLeftShift, KeyDot)
	rMap['/'] = stroke(0, KeySlash)
	rMap['?'] = stroke(ModLeftShift, KeySlash)

	return &Layout{Name: "US", Runes: rMap}
}

func buildDELayout() *Layout {
	l := buildUSLayout()
	l.Name = "DE"

	l.Runes['z'] = stroke(0, KeyY)
	l.Runes['Z'] = stroke(ModLeftShift, KeyY)
	l.Runes['y'] = stroke(0, KeyZ)
	l.Runes['Y'] = stroke(ModLeftShift, KeyZ)

	l.Runes['ä'] = stroke(0, KeyApostrophe)
	l.Runes['Ä'] = stroke(ModLeftShift, KeyApostrophe)
	l.Runes['ö'] = stroke(0, KeySemicolon)
	l.Runes['Ö'] = stroke(ModLeftShift, KeySemicolon)
	l.Runes['ü'] = stroke(0, KeyLeftBrace)
	l.Runes['Ü'] = stroke(ModLeftShift, KeyLeftBrace)
	l.Runes['ß'] = stroke(0, KeyMinus)

	// Dead keys in German QWERTZ emit dead key + Space so trailing characters/newlines are never swallowed.
	l.Runes['^'] = deadStroke(0, KeyGrave, 0, KeySpace)
	l.Runes['°'] = stroke(ModLeftShift, KeyGrave)
	l.Runes['´'] = deadStroke(0, KeyEqual, 0, KeySpace)
	l.Runes['`'] = deadStroke(ModLeftShift, KeyEqual, 0, KeySpace)

	l.Runes['"'] = stroke(ModLeftShift, Key2)
	l.Runes['§'] = stroke(ModLeftShift, Key3)
	l.Runes['&'] = stroke(ModLeftShift, Key6)
	l.Runes['/'] = stroke(ModLeftShift, Key7)
	l.Runes['('] = stroke(ModLeftShift, Key8)
	l.Runes[')'] = stroke(ModLeftShift, Key9)
	l.Runes['='] = stroke(ModLeftShift, Key0)

	l.Runes['?'] = stroke(ModLeftShift, KeyMinus)
	l.Runes['+'] = stroke(0, KeyRightBrace)
	l.Runes['*'] = stroke(ModLeftShift, KeyRightBrace)
	l.Runes['~'] = stroke(ModRightAlt, KeyRightBrace)
	l.Runes['#'] = stroke(0, KeyHashtilde)
	l.Runes['\''] = stroke(ModLeftShift, KeyHashtilde)
	l.Runes[';'] = stroke(ModLeftShift, KeyComma)
	l.Runes[':'] = stroke(ModLeftShift, KeyDot)
	l.Runes['-'] = stroke(0, KeySlash)
	l.Runes['_'] = stroke(ModLeftShift, KeySlash)
	l.Runes['<'] = stroke(0, KeyNonUSBackslash)
	l.Runes['>'] = stroke(ModLeftShift, KeyNonUSBackslash)
	l.Runes['|'] = stroke(ModRightAlt, KeyNonUSBackslash)
	l.Runes['@'] = stroke(ModRightAlt, KeyQ)
	l.Runes['€'] = stroke(ModRightAlt, KeyE)
	l.Runes['\\'] = stroke(ModRightAlt, KeyMinus)
	l.Runes['['] = stroke(ModRightAlt, Key8)
	l.Runes[']'] = stroke(ModRightAlt, Key9)
	l.Runes['{'] = stroke(ModRightAlt, Key7)
	l.Runes['}'] = stroke(ModRightAlt, Key0)

	addDeadAccentVowels(l, 0, KeyEqual, [5]rune{'á', 'é', 'í', 'ó', 'ú'}, [5]rune{'Á', 'É', 'Í', 'Ó', 'Ú'})
	addDeadAccentVowels(l, ModLeftShift, KeyEqual, [5]rune{'à', 'è', 'ì', 'ò', 'ù'}, [5]rune{'À', 'È', 'Ì', 'Ò', 'Ù'})
	addDeadAccentVowels(l, 0, KeyGrave, [5]rune{'â', 'ê', 'î', 'ô', 'û'}, [5]rune{'Â', 'Ê', 'Î', 'Ô', 'Û'})

	return l
}

func buildESLayout() *Layout {
	l := buildUSLayout()
	l.Name = "ES"

	l.Runes['ñ'] = stroke(0, KeySemicolon)
	l.Runes['Ñ'] = stroke(ModLeftShift, KeySemicolon)
	l.Runes['ç'] = stroke(0, KeyHashtilde)
	l.Runes['Ç'] = stroke(ModLeftShift, KeyHashtilde)
	l.Runes['º'] = stroke(0, KeyGrave)
	l.Runes['ª'] = stroke(ModLeftShift, KeyGrave)

	l.Runes['"'] = stroke(ModLeftShift, Key2)
	l.Runes['·'] = stroke(ModLeftShift, Key3)
	l.Runes['&'] = stroke(ModLeftShift, Key6)
	l.Runes['/'] = stroke(ModLeftShift, Key7)
	l.Runes['('] = stroke(ModLeftShift, Key8)
	l.Runes[')'] = stroke(ModLeftShift, Key9)
	l.Runes['='] = stroke(ModLeftShift, Key0)

	l.Runes['\''] = stroke(0, KeyMinus)
	l.Runes['?'] = stroke(ModLeftShift, KeyMinus)
	l.Runes['¡'] = stroke(0, KeyEqual)
	l.Runes['¿'] = stroke(ModLeftShift, KeyEqual)

	// Dead keys in Spanish ISO: `, ^, ´, ¨
	l.Runes['`'] = deadStroke(0, KeyLeftBrace, 0, KeySpace)
	l.Runes['^'] = deadStroke(ModLeftShift, KeyLeftBrace, 0, KeySpace)
	l.Runes['['] = stroke(ModRightAlt, KeyLeftBrace)
	l.Runes['+'] = stroke(0, KeyRightBrace)
	l.Runes['*'] = stroke(ModLeftShift, KeyRightBrace)
	l.Runes[']'] = stroke(ModRightAlt, KeyRightBrace)
	l.Runes['´'] = deadStroke(0, KeyApostrophe, 0, KeySpace)
	l.Runes['¨'] = deadStroke(ModLeftShift, KeyApostrophe, 0, KeySpace)
	l.Runes['{'] = stroke(ModRightAlt, KeyApostrophe)
	l.Runes['}'] = stroke(ModRightAlt, KeyHashtilde)
	l.Runes['|'] = stroke(ModRightAlt, Key1)

	l.Runes['-'] = stroke(0, KeySlash)
	l.Runes['_'] = stroke(ModLeftShift, KeySlash)
	l.Runes[';'] = stroke(ModLeftShift, KeyComma)
	l.Runes[':'] = stroke(ModLeftShift, KeyDot)
	l.Runes['<'] = stroke(0, KeyNonUSBackslash)
	l.Runes['>'] = stroke(ModLeftShift, KeyNonUSBackslash)
	l.Runes['@'] = stroke(ModRightAlt, Key2)
	l.Runes['#'] = stroke(ModRightAlt, Key3)
	l.Runes['€'] = stroke(ModRightAlt, KeyE)
	l.Runes['~'] = stroke(ModRightAlt, Key4)
	l.Runes['\\'] = stroke(ModRightAlt, KeyGrave)

	addDeadAccentVowels(l, 0, KeyApostrophe, [5]rune{'á', 'é', 'í', 'ó', 'ú'}, [5]rune{'Á', 'É', 'Í', 'Ó', 'Ú'})
	addDeadAccentVowels(l, ModLeftShift, KeyApostrophe, [5]rune{'ä', 'ë', 'ï', 'ö', 'ü'}, [5]rune{'Ä', 'Ë', 'Ï', 'Ö', 'Ü'})
	addDeadAccentVowels(l, 0, KeyLeftBrace, [5]rune{'à', 'è', 'ì', 'ò', 'ù'}, [5]rune{'À', 'È', 'Ì', 'Ò', 'Ù'})
	addDeadAccentVowels(l, ModLeftShift, KeyLeftBrace, [5]rune{'â', 'ê', 'î', 'ô', 'û'}, [5]rune{'Â', 'Ê', 'Î', 'Ô', 'Û'})

	return l
}

func buildFRLayout() *Layout {
	l := buildUSLayout()
	l.Name = "FR"

	l.Runes['a'] = stroke(0, KeyQ)
	l.Runes['A'] = stroke(ModLeftShift, KeyQ)
	l.Runes['z'] = stroke(0, KeyW)
	l.Runes['Z'] = stroke(ModLeftShift, KeyW)
	l.Runes['q'] = stroke(0, KeyA)
	l.Runes['Q'] = stroke(ModLeftShift, KeyA)
	l.Runes['w'] = stroke(0, KeyZ)
	l.Runes['W'] = stroke(ModLeftShift, KeyZ)
	l.Runes['m'] = stroke(0, KeySemicolon)
	l.Runes['M'] = stroke(ModLeftShift, KeySemicolon)

	for i, r := range []rune{'1', '2', '3', '4', '5', '6', '7', '8', '9', '0'} {
		l.Runes[r] = stroke(ModLeftShift, Key1+uint8(i))
	}

	l.Runes['&'] = stroke(0, Key1)
	l.Runes['é'] = stroke(0, Key2)
	l.Runes['"'] = stroke(0, Key3)
	l.Runes['\''] = stroke(0, Key4)
	l.Runes['('] = stroke(0, Key5)
	l.Runes['-'] = stroke(0, Key6)
	l.Runes['è'] = stroke(0, Key7)
	l.Runes['_'] = stroke(0, Key8)
	l.Runes['ç'] = stroke(0, Key9)
	l.Runes['à'] = stroke(0, Key0)

	l.Runes['²'] = stroke(0, KeyGrave)
	l.Runes[')'] = stroke(0, KeyMinus)

	// Dead keys in French AZERTY: ^, ¨, ~ (AltGr+2), ` (AltGr+7)
	l.Runes['^'] = deadStroke(0, KeyLeftBrace, 0, KeySpace)
	l.Runes['¨'] = deadStroke(ModLeftShift, KeyLeftBrace, 0, KeySpace)
	l.Runes['$'] = stroke(0, KeyRightBrace)
	l.Runes['£'] = stroke(ModLeftShift, KeyRightBrace)
	l.Runes['*'] = stroke(0, KeyHashtilde)
	l.Runes['µ'] = stroke(ModLeftShift, KeyHashtilde)

	l.Runes[','] = stroke(0, KeyM)
	l.Runes['?'] = stroke(ModLeftShift, KeyM)
	l.Runes[';'] = stroke(0, KeyComma)
	l.Runes['.'] = stroke(ModLeftShift, KeyComma)
	l.Runes[':'] = stroke(0, KeyDot)
	l.Runes['/'] = stroke(ModLeftShift, KeyDot)
	l.Runes['!'] = stroke(0, KeySlash)
	l.Runes['§'] = stroke(ModLeftShift, KeySlash)

	l.Runes['<'] = stroke(0, KeyNonUSBackslash)
	l.Runes['>'] = stroke(ModLeftShift, KeyNonUSBackslash)
	l.Runes['@'] = stroke(ModRightAlt, Key0)
	l.Runes['€'] = stroke(ModRightAlt, KeyE)
	l.Runes['~'] = deadStroke(ModRightAlt, Key2, 0, KeySpace)
	l.Runes['#'] = stroke(ModRightAlt, Key3)
	l.Runes['{'] = stroke(ModRightAlt, Key4)
	l.Runes['['] = stroke(ModRightAlt, Key5)
	l.Runes['|'] = stroke(ModRightAlt, Key6)
	l.Runes['`'] = deadStroke(ModRightAlt, Key7, 0, KeySpace)
	l.Runes['\\'] = stroke(ModRightAlt, Key8)
	l.Runes[']'] = stroke(ModRightAlt, KeyMinus)
	l.Runes['}'] = stroke(ModRightAlt, KeyEqual)
	l.Runes['ù'] = stroke(0, KeyApostrophe)
	l.Runes['%'] = stroke(ModLeftShift, KeyApostrophe)

	addDeadAccentVowels(l, 0, KeyLeftBrace, [5]rune{'â', 'ê', 'î', 'ô', 'û'}, [5]rune{'Â', 'Ê', 'Î', 'Ô', 'Û'})
	addDeadAccentVowels(l, ModLeftShift, KeyLeftBrace, [5]rune{'ä', 'ë', 'ï', 'ö', 'ü'}, [5]rune{'Ä', 'Ë', 'Ï', 'Ö', 'Ü'})
	addDeadAccentVowels(l, ModRightAlt, Key7, [5]rune{'à', 'è', 'ì', 'ò', 'ù'}, [5]rune{'À', 'È', 'Ì', 'Ò', 'Ù'})

	return l
}
