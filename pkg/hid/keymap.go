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

// Layout defines mapping rules for translating runes and key names into HID reports.
type Layout struct {
	Name  string
	Runes map[rune]KeyboardReport
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
	"LEFTALT":     {Modifiers: ModLeftAlt, KeyCode: 0},
	"RIGHTALT":    {Modifiers: ModRightAlt, KeyCode: 0},
	"ALTGR":       {Modifiers: ModRightAlt, KeyCode: 0},
	"GUI":         {Modifiers: ModLeftGUI, KeyCode: 0},
	"WINDOWS":     {Modifiers: ModLeftGUI, KeyCode: 0},
	"COMMAND":     {Modifiers: ModLeftGUI, KeyCode: 0},
	"SUPER":       {Modifiers: ModLeftGUI, KeyCode: 0},
	"LEFTGUI":     {Modifiers: ModLeftGUI, KeyCode: 0},
	"RIGHTGUI":    {Modifiers: ModRightGUI, KeyCode: 0},
	"ENTER":       {Modifiers: 0, KeyCode: KeyEnter},
	"RETURN":      {Modifiers: 0, KeyCode: KeyEnter},
	"ESC":         {Modifiers: 0, KeyCode: KeyEsc},
	"ESCAPE":      {Modifiers: 0, KeyCode: KeyEsc},
	"BACKSPACE":   {Modifiers: 0, KeyCode: KeyBackspace},
	"TAB":         {Modifiers: 0, KeyCode: KeyTab},
	"SPACE":       {Modifiers: 0, KeyCode: KeySpace},
	"DELETE":      {Modifiers: 0, KeyCode: KeyDelete},
	"DEL":         {Modifiers: 0, KeyCode: KeyDelete},
	"INSERT":      {Modifiers: 0, KeyCode: KeyInsert},
	"INS":         {Modifiers: 0, KeyCode: KeyInsert},
	"HOME":        {Modifiers: 0, KeyCode: KeyHome},
	"END":         {Modifiers: 0, KeyCode: KeyEnd},
	"PAGEUP":      {Modifiers: 0, KeyCode: KeyPageUp},
	"PAGEDOWN":    {Modifiers: 0, KeyCode: KeyPageDown},
	"UP":          {Modifiers: 0, KeyCode: KeyUp},
	"UPARROW":     {Modifiers: 0, KeyCode: KeyUp},
	"DOWN":        {Modifiers: 0, KeyCode: KeyDown},
	"DOWNARROW":   {Modifiers: 0, KeyCode: KeyDown},
	"LEFT":        {Modifiers: 0, KeyCode: KeyLeft},
	"LEFTARROW":   {Modifiers: 0, KeyCode: KeyLeft},
	"RIGHT":       {Modifiers: 0, KeyCode: KeyRight},
	"RIGHTARROW":  {Modifiers: 0, KeyCode: KeyRight},
	"CAPSLOCK":    {Modifiers: 0, KeyCode: KeyCapsLock},
	"NUMLOCK":     {Modifiers: 0, KeyCode: KeyNumLock},
	"SCROLLLOCK":  {Modifiers: 0, KeyCode: KeyScrollLock},
	"PRINTSCREEN": {Modifiers: 0, KeyCode: KeyPrintScreen},
	"PAUSE":       {Modifiers: 0, KeyCode: KeyPause},
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
		aliasSmartQuotes(l)
	}
}

func aliasSmartQuotes(l *Layout) {
	for _, q := range []rune{'\u2018', '\u2019', '\u201a', '\u201b'} {
		l.Runes[q] = l.Runes['\'']
	}
	for _, q := range []rune{'\u201c', '\u201d', '\u201e', '\u201f', '\u00ab', '\u00bb'} {
		l.Runes[q] = l.Runes['"']
	}
}

// GetLayout returns the Layout struct for the given layout name ("us", "de", "es", "fr").
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
		report, ok := l.Runes[unicode.ToLower(r)]
		if !ok {
			report, ok = l.Runes[r]
		}
		if ok {
			if kcs := report.KeyCodes(); len(kcs) > 0 {
				return KeyCombo{Modifiers: 0, KeyCode: kcs[0]}, true
			}
		}
	}

	return KeyCombo{}, false
}

func buildUSLayout() *Layout {
	rMap := make(map[rune]KeyboardReport)

	// Letters a-z
	for c := 'a'; c <= 'z'; c++ {
		key := uint8(KeyA + uint8(c-'a'))
		rMap[c] = NewKeyboardReport(0, key)
		rMap[unicode.ToUpper(c)] = NewKeyboardReport(ModLeftShift, key)
	}

	// Digits 0-9
	rMap['1'] = NewKeyboardReport(0, Key1)
	rMap['2'] = NewKeyboardReport(0, Key2)
	rMap['3'] = NewKeyboardReport(0, Key3)
	rMap['4'] = NewKeyboardReport(0, Key4)
	rMap['5'] = NewKeyboardReport(0, Key5)
	rMap['6'] = NewKeyboardReport(0, Key6)
	rMap['7'] = NewKeyboardReport(0, Key7)
	rMap['8'] = NewKeyboardReport(0, Key8)
	rMap['9'] = NewKeyboardReport(0, Key9)
	rMap['0'] = NewKeyboardReport(0, Key0)

	// Shifted digits
	rMap['!'] = NewKeyboardReport(ModLeftShift, Key1)
	rMap['@'] = NewKeyboardReport(ModLeftShift, Key2)
	rMap['#'] = NewKeyboardReport(ModLeftShift, Key3)
	rMap['$'] = NewKeyboardReport(ModLeftShift, Key4)
	rMap['%'] = NewKeyboardReport(ModLeftShift, Key5)
	rMap['^'] = NewKeyboardReport(ModLeftShift, Key6)
	rMap['&'] = NewKeyboardReport(ModLeftShift, Key7)
	rMap['*'] = NewKeyboardReport(ModLeftShift, Key8)
	rMap['('] = NewKeyboardReport(ModLeftShift, Key9)
	rMap[')'] = NewKeyboardReport(ModLeftShift, Key0)

	// Whitespace
	rMap['\n'] = NewKeyboardReport(0, KeyEnter)
	rMap['\r'] = NewKeyboardReport(0, KeyEnter)
	rMap['\t'] = NewKeyboardReport(0, KeyTab)
	rMap[' '] = NewKeyboardReport(0, KeySpace)

	// Punctuations & Symbols
	rMap['-'] = NewKeyboardReport(0, KeyMinus)
	rMap['_'] = NewKeyboardReport(ModLeftShift, KeyMinus)
	rMap['='] = NewKeyboardReport(0, KeyEqual)
	rMap['+'] = NewKeyboardReport(ModLeftShift, KeyEqual)
	rMap['['] = NewKeyboardReport(0, KeyLeftBrace)
	rMap['{'] = NewKeyboardReport(ModLeftShift, KeyLeftBrace)
	rMap[']'] = NewKeyboardReport(0, KeyRightBrace)
	rMap['}'] = NewKeyboardReport(ModLeftShift, KeyRightBrace)
	rMap['\\'] = NewKeyboardReport(0, KeyBackslash)
	rMap['|'] = NewKeyboardReport(ModLeftShift, KeyBackslash)
	rMap[';'] = NewKeyboardReport(0, KeySemicolon)
	rMap[':'] = NewKeyboardReport(ModLeftShift, KeySemicolon)
	rMap['\''] = NewKeyboardReport(0, KeyApostrophe)
	rMap['"'] = NewKeyboardReport(ModLeftShift, KeyApostrophe)
	rMap['`'] = NewKeyboardReport(0, KeyGrave)
	rMap['~'] = NewKeyboardReport(ModLeftShift, KeyGrave)
	rMap[','] = NewKeyboardReport(0, KeyComma)
	rMap['<'] = NewKeyboardReport(ModLeftShift, KeyComma)
	rMap['.'] = NewKeyboardReport(0, KeyDot)
	rMap['>'] = NewKeyboardReport(ModLeftShift, KeyDot)
	rMap['/'] = NewKeyboardReport(0, KeySlash)
	rMap['?'] = NewKeyboardReport(ModLeftShift, KeySlash)

	return &Layout{
		Name:  "US",
		Runes: rMap,
	}
}

func buildDELayout() *Layout {
	l := buildUSLayout()
	l.Name = "DE"

	// Y and Z swap in QWERTZ
	l.Runes['z'] = NewKeyboardReport(0, KeyY)
	l.Runes['Z'] = NewKeyboardReport(ModLeftShift, KeyY)
	l.Runes['y'] = NewKeyboardReport(0, KeyZ)
	l.Runes['Y'] = NewKeyboardReport(ModLeftShift, KeyZ)

	// Umlauts & German specific
	l.Runes['ä'] = NewKeyboardReport(0, KeyApostrophe)
	l.Runes['Ä'] = NewKeyboardReport(ModLeftShift, KeyApostrophe)
	l.Runes['ö'] = NewKeyboardReport(0, KeySemicolon)
	l.Runes['Ö'] = NewKeyboardReport(ModLeftShift, KeySemicolon)
	l.Runes['ü'] = NewKeyboardReport(0, KeyLeftBrace)
	l.Runes['Ü'] = NewKeyboardReport(ModLeftShift, KeyLeftBrace)
	l.Runes['ß'] = NewKeyboardReport(0, KeyMinus)

	// Symbols for DE layout
	l.Runes['^'] = NewKeyboardReport(0, KeyGrave)
	l.Runes['°'] = NewKeyboardReport(ModLeftShift, KeyGrave)
	l.Runes['´'] = NewKeyboardReport(0, KeyEqual)
	l.Runes['`'] = NewKeyboardReport(ModLeftShift, KeyEqual)
	l.Runes['"'] = NewKeyboardReport(ModLeftShift, Key2)
	l.Runes['§'] = NewKeyboardReport(ModLeftShift, Key3)
	l.Runes['&'] = NewKeyboardReport(ModLeftShift, Key6)
	l.Runes['/'] = NewKeyboardReport(ModLeftShift, Key7)
	l.Runes['('] = NewKeyboardReport(ModLeftShift, Key8)
	l.Runes[')'] = NewKeyboardReport(ModLeftShift, Key9)
	l.Runes['='] = NewKeyboardReport(ModLeftShift, Key0)

	l.Runes['?'] = NewKeyboardReport(ModLeftShift, KeyMinus)
	l.Runes['+'] = NewKeyboardReport(0, KeyRightBrace)
	l.Runes['*'] = NewKeyboardReport(ModLeftShift, KeyRightBrace)
	l.Runes['~'] = NewKeyboardReport(ModRightAlt, KeyRightBrace)
	l.Runes['#'] = NewKeyboardReport(0, KeyHashtilde)
	l.Runes['\''] = NewKeyboardReport(ModLeftShift, KeyHashtilde)
	l.Runes[';'] = NewKeyboardReport(ModLeftShift, KeyComma)
	l.Runes[':'] = NewKeyboardReport(ModLeftShift, KeyDot)
	l.Runes['-'] = NewKeyboardReport(0, KeySlash)
	l.Runes['_'] = NewKeyboardReport(ModLeftShift, KeySlash)
	l.Runes['<'] = NewKeyboardReport(0, KeyNonUSBackslash)
	l.Runes['>'] = NewKeyboardReport(ModLeftShift, KeyNonUSBackslash)
	l.Runes['|'] = NewKeyboardReport(ModRightAlt, KeyNonUSBackslash)
	l.Runes['@'] = NewKeyboardReport(ModRightAlt, KeyQ)
	l.Runes['€'] = NewKeyboardReport(ModRightAlt, KeyE)
	l.Runes['\\'] = NewKeyboardReport(ModRightAlt, KeyMinus)
	l.Runes['['] = NewKeyboardReport(ModRightAlt, Key8)
	l.Runes[']'] = NewKeyboardReport(ModRightAlt, Key9)
	l.Runes['{'] = NewKeyboardReport(ModRightAlt, Key7)
	l.Runes['}'] = NewKeyboardReport(ModRightAlt, Key0)

	return l
}

func buildESLayout() *Layout {
	l := buildUSLayout()
	l.Name = "ES"

	// Spanish specific
	l.Runes['ñ'] = NewKeyboardReport(0, KeySemicolon)
	l.Runes['Ñ'] = NewKeyboardReport(ModLeftShift, KeySemicolon)
	l.Runes['ç'] = NewKeyboardReport(0, KeyHashtilde)
	l.Runes['Ç'] = NewKeyboardReport(ModLeftShift, KeyHashtilde)
	l.Runes['º'] = NewKeyboardReport(0, KeyGrave)
	l.Runes['ª'] = NewKeyboardReport(ModLeftShift, KeyGrave)

	l.Runes['"'] = NewKeyboardReport(ModLeftShift, Key2)
	l.Runes['·'] = NewKeyboardReport(ModLeftShift, Key3)
	l.Runes['&'] = NewKeyboardReport(ModLeftShift, Key6)
	l.Runes['/'] = NewKeyboardReport(ModLeftShift, Key7)
	l.Runes['('] = NewKeyboardReport(ModLeftShift, Key8)
	l.Runes[')'] = NewKeyboardReport(ModLeftShift, Key9)
	l.Runes['='] = NewKeyboardReport(ModLeftShift, Key0)

	l.Runes['\''] = NewKeyboardReport(0, KeyMinus)
	l.Runes['?'] = NewKeyboardReport(ModLeftShift, KeyMinus)
	l.Runes['¡'] = NewKeyboardReport(0, KeyEqual)
	l.Runes['¿'] = NewKeyboardReport(ModLeftShift, KeyEqual)
	l.Runes['`'] = NewKeyboardReport(0, KeyLeftBrace)
	l.Runes['^'] = NewKeyboardReport(ModLeftShift, KeyLeftBrace)
	l.Runes['['] = NewKeyboardReport(ModRightAlt, KeyLeftBrace)
	l.Runes['+'] = NewKeyboardReport(0, KeyRightBrace)
	l.Runes['*'] = NewKeyboardReport(ModLeftShift, KeyRightBrace)
	l.Runes[']'] = NewKeyboardReport(ModRightAlt, KeyRightBrace)
	l.Runes['´'] = NewKeyboardReport(0, KeyApostrophe)
	l.Runes['¨'] = NewKeyboardReport(ModLeftShift, KeyApostrophe)
	l.Runes['{'] = NewKeyboardReport(ModRightAlt, KeyApostrophe)
	l.Runes['}'] = NewKeyboardReport(ModRightAlt, KeyHashtilde)
	l.Runes['|'] = NewKeyboardReport(ModRightAlt, Key1)

	l.Runes['-'] = NewKeyboardReport(0, KeySlash)
	l.Runes['_'] = NewKeyboardReport(ModLeftShift, KeySlash)
	l.Runes[';'] = NewKeyboardReport(ModLeftShift, KeyComma)
	l.Runes[':'] = NewKeyboardReport(ModLeftShift, KeyDot)
	l.Runes['<'] = NewKeyboardReport(0, KeyNonUSBackslash)
	l.Runes['>'] = NewKeyboardReport(ModLeftShift, KeyNonUSBackslash)
	l.Runes['@'] = NewKeyboardReport(ModRightAlt, Key2)
	l.Runes['#'] = NewKeyboardReport(ModRightAlt, Key3)
	l.Runes['€'] = NewKeyboardReport(ModRightAlt, KeyE)
	l.Runes['~'] = NewKeyboardReport(ModRightAlt, Key4)
	l.Runes['\\'] = NewKeyboardReport(ModRightAlt, KeyGrave)

	return l
}

func buildFRLayout() *Layout {
	l := buildUSLayout()
	l.Name = "FR"

	// AZERTY letter swaps
	l.Runes['a'] = NewKeyboardReport(0, KeyQ)
	l.Runes['A'] = NewKeyboardReport(ModLeftShift, KeyQ)
	l.Runes['z'] = NewKeyboardReport(0, KeyW)
	l.Runes['Z'] = NewKeyboardReport(ModLeftShift, KeyW)
	l.Runes['q'] = NewKeyboardReport(0, KeyA)
	l.Runes['Q'] = NewKeyboardReport(ModLeftShift, KeyA)
	l.Runes['w'] = NewKeyboardReport(0, KeyZ)
	l.Runes['W'] = NewKeyboardReport(ModLeftShift, KeyZ)
	l.Runes['m'] = NewKeyboardReport(0, KeySemicolon)
	l.Runes['M'] = NewKeyboardReport(ModLeftShift, KeySemicolon)

	// French digits require shift
	l.Runes['1'] = NewKeyboardReport(ModLeftShift, Key1)
	l.Runes['2'] = NewKeyboardReport(ModLeftShift, Key2)
	l.Runes['3'] = NewKeyboardReport(ModLeftShift, Key3)
	l.Runes['4'] = NewKeyboardReport(ModLeftShift, Key4)
	l.Runes['5'] = NewKeyboardReport(ModLeftShift, Key5)
	l.Runes['6'] = NewKeyboardReport(ModLeftShift, Key6)
	l.Runes['7'] = NewKeyboardReport(ModLeftShift, Key7)
	l.Runes['8'] = NewKeyboardReport(ModLeftShift, Key8)
	l.Runes['9'] = NewKeyboardReport(ModLeftShift, Key9)
	l.Runes['0'] = NewKeyboardReport(ModLeftShift, Key0)

	// Unshifted number keys in FR
	l.Runes['&'] = NewKeyboardReport(0, Key1)
	l.Runes['é'] = NewKeyboardReport(0, Key2)
	l.Runes['"'] = NewKeyboardReport(0, Key3)
	l.Runes['\''] = NewKeyboardReport(0, Key4)
	l.Runes['('] = NewKeyboardReport(0, Key5)
	l.Runes['-'] = NewKeyboardReport(0, Key6)
	l.Runes['è'] = NewKeyboardReport(0, Key7)
	l.Runes['_'] = NewKeyboardReport(0, Key8)
	l.Runes['ç'] = NewKeyboardReport(0, Key9)
	l.Runes['à'] = NewKeyboardReport(0, Key0)

	l.Runes['²'] = NewKeyboardReport(0, KeyGrave)
	l.Runes[')'] = NewKeyboardReport(0, KeyMinus)
	l.Runes['^'] = NewKeyboardReport(0, KeyLeftBrace)
	l.Runes['¨'] = NewKeyboardReport(ModLeftShift, KeyLeftBrace)
	l.Runes['$'] = NewKeyboardReport(0, KeyRightBrace)
	l.Runes['£'] = NewKeyboardReport(ModLeftShift, KeyRightBrace)
	l.Runes['*'] = NewKeyboardReport(0, KeyHashtilde)
	l.Runes['µ'] = NewKeyboardReport(ModLeftShift, KeyHashtilde)

	l.Runes[','] = NewKeyboardReport(0, KeyM)
	l.Runes['?'] = NewKeyboardReport(ModLeftShift, KeyM)
	l.Runes[';'] = NewKeyboardReport(0, KeyComma)
	l.Runes['.'] = NewKeyboardReport(ModLeftShift, KeyComma)
	l.Runes[':'] = NewKeyboardReport(0, KeyDot)
	l.Runes['/'] = NewKeyboardReport(ModLeftShift, KeyDot)
	l.Runes['!'] = NewKeyboardReport(0, KeySlash)
	l.Runes['§'] = NewKeyboardReport(ModLeftShift, KeySlash)

	l.Runes['<'] = NewKeyboardReport(0, KeyNonUSBackslash)
	l.Runes['>'] = NewKeyboardReport(ModLeftShift, KeyNonUSBackslash)
	l.Runes['@'] = NewKeyboardReport(ModRightAlt, Key0)
	l.Runes['€'] = NewKeyboardReport(ModRightAlt, KeyE)
	l.Runes['~'] = NewKeyboardReport(ModRightAlt, KeyN)
	l.Runes['#'] = NewKeyboardReport(ModRightAlt, Key3)
	l.Runes['{'] = NewKeyboardReport(ModRightAlt, Key4)
	l.Runes['['] = NewKeyboardReport(ModRightAlt, Key5)
	l.Runes['|'] = NewKeyboardReport(ModRightAlt, Key6)
	l.Runes['`'] = NewKeyboardReport(ModRightAlt, Key7)
	l.Runes['\\'] = NewKeyboardReport(ModRightAlt, Key8)
	l.Runes[']'] = NewKeyboardReport(ModRightAlt, KeyMinus)
	l.Runes['}'] = NewKeyboardReport(ModRightAlt, KeyEqual)
	l.Runes['ù'] = NewKeyboardReport(0, KeyApostrophe)
	l.Runes['%'] = NewKeyboardReport(ModLeftShift, KeyApostrophe)

	return l
}
