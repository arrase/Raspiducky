package hid

import (
	"fmt"
	"strings"
	"sync"
	"unicode"
)

// KeyCombo represents a modifier bitmask and a main keycode.
type KeyCombo struct {
	Modifiers uint8
	KeyCode   uint8
}

// Layout defines mapping rules for translating runes and key names into HID reports.
type Layout struct {
	Name     string
	Runes    map[rune][]KeyboardReport
	KeyNames map[string]KeyCombo
}

// Clone creates a deep copy of the Layout to prevent data races when accessed concurrently.
func (l *Layout) Clone() *Layout {
	if l == nil {
		return nil
	}
	newRunes := make(map[rune][]KeyboardReport, len(l.Runes))
	for k, v := range l.Runes {
		reportsCopy := make([]KeyboardReport, len(v))
		copy(reportsCopy, v)
		newRunes[k] = reportsCopy
	}
	newKeyNames := make(map[string]KeyCombo, len(l.KeyNames))
	for k, v := range l.KeyNames {
		newKeyNames[k] = v
	}
	return &Layout{
		Name:     l.Name,
		Runes:    newRunes,
		KeyNames: newKeyNames,
	}
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

var (
	layoutsMu sync.RWMutex
	layouts   = map[string]*Layout{}
)

func init() {
	layoutsMu.Lock()
	defer layoutsMu.Unlock()
	layouts["US"] = buildUSLayout()
	layouts["DE"] = buildDELayout()
	layouts["ES"] = buildESLayout()
	layouts["FR"] = buildFRLayout()
}

// GetLayout returns the Layout struct for the given layout name ("us", "de", "es", "fr").
func GetLayout(name string) (*Layout, error) {
	upper := strings.ToUpper(strings.TrimSpace(name))
	layoutsMu.RLock()
	l, ok := layouts[upper]
	layoutsMu.RUnlock()
	if ok {
		return l.Clone(), nil
	}
	return nil, fmt.Errorf("unsupported layout: %s", name)
}

// MapRune returns the sequence of KeyboardReports to produce the given rune in this layout.
func (l *Layout) MapRune(r rune) ([]KeyboardReport, bool) {
	reports, ok := l.Runes[r]
	return reports, ok
}

// MapKeyName resolves a key name (e.g. "CTRL", "GUI", "a", "ENTER", "F4") to a KeyCombo in this layout.
func (l *Layout) MapKeyName(name string) (KeyCombo, bool) {
	upper := strings.ToUpper(strings.TrimSpace(name))
	if combo, ok := l.KeyNames[upper]; ok {
		return combo, true
	}
	if combo, ok := commonKeyNames[upper]; ok {
		return combo, true
	}

	// Single letter or digit check
	if len(upper) == 1 {
		r := rune(upper[0])
		if reports, ok := l.Runes[unicode.ToLower(r)]; ok && len(reports) > 0 {
			return KeyCombo{Modifiers: reports[0].Modifiers(), KeyCode: reports[0].KeyCodes()[0]}, true
		}
	}

	return KeyCombo{}, false
}

func buildUSLayout() *Layout {
	rMap := make(map[rune][]KeyboardReport)

	// Letters a-z
	for c := 'a'; c <= 'z'; c++ {
		key := uint8(KeyA + uint8(c-'a'))
		rMap[c] = []KeyboardReport{NewKeyboardReport(0, key)}
		rMap[unicode.ToUpper(c)] = []KeyboardReport{NewKeyboardReport(ModLeftShift, key)}
	}

	// Digits 0-9
	rMap['1'] = []KeyboardReport{NewKeyboardReport(0, Key1)}
	rMap['2'] = []KeyboardReport{NewKeyboardReport(0, Key2)}
	rMap['3'] = []KeyboardReport{NewKeyboardReport(0, Key3)}
	rMap['4'] = []KeyboardReport{NewKeyboardReport(0, Key4)}
	rMap['5'] = []KeyboardReport{NewKeyboardReport(0, Key5)}
	rMap['6'] = []KeyboardReport{NewKeyboardReport(0, Key6)}
	rMap['7'] = []KeyboardReport{NewKeyboardReport(0, Key7)}
	rMap['8'] = []KeyboardReport{NewKeyboardReport(0, Key8)}
	rMap['9'] = []KeyboardReport{NewKeyboardReport(0, Key9)}
	rMap['0'] = []KeyboardReport{NewKeyboardReport(0, Key0)}

	// Shifted digits
	rMap['!'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key1)}
	rMap['@'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key2)}
	rMap['#'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key3)}
	rMap['$'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key4)}
	rMap['%'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key5)}
	rMap['^'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key6)}
	rMap['&'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key7)}
	rMap['*'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key8)}
	rMap['('] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key9)}
	rMap[')'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key0)}

	// Whitespace
	rMap['\n'] = []KeyboardReport{NewKeyboardReport(0, KeyEnter)}
	rMap['\r'] = []KeyboardReport{NewKeyboardReport(0, KeyEnter)}
	rMap['\t'] = []KeyboardReport{NewKeyboardReport(0, KeyTab)}
	rMap[' '] = []KeyboardReport{NewKeyboardReport(0, KeySpace)}

	// Punctuations & Symbols
	rMap['-'] = []KeyboardReport{NewKeyboardReport(0, KeyMinus)}
	rMap['_'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyMinus)}
	rMap['='] = []KeyboardReport{NewKeyboardReport(0, KeyEqual)}
	rMap['+'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyEqual)}
	rMap['['] = []KeyboardReport{NewKeyboardReport(0, KeyLeftBrace)}
	rMap['{'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyLeftBrace)}
	rMap[']'] = []KeyboardReport{NewKeyboardReport(0, KeyRightBrace)}
	rMap['}'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyRightBrace)}
	rMap['\\'] = []KeyboardReport{NewKeyboardReport(0, KeyBackslash)}
	rMap['|'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyBackslash)}
	rMap[';'] = []KeyboardReport{NewKeyboardReport(0, KeySemicolon)}
	rMap[':'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeySemicolon)}
	rMap['\''] = []KeyboardReport{NewKeyboardReport(0, KeyApostrophe)}
	rMap['"'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyApostrophe)}
	rMap['`'] = []KeyboardReport{NewKeyboardReport(0, KeyGrave)}
	rMap['~'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyGrave)}
	rMap[','] = []KeyboardReport{NewKeyboardReport(0, KeyComma)}
	rMap['<'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyComma)}
	rMap['.'] = []KeyboardReport{NewKeyboardReport(0, KeyDot)}
	rMap['>'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyDot)}
	rMap['/'] = []KeyboardReport{NewKeyboardReport(0, KeySlash)}
	rMap['?'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeySlash)}

	return &Layout{
		Name:     "US",
		Runes:    rMap,
		KeyNames: make(map[string]KeyCombo),
	}
}

func buildDELayout() *Layout {
	l := buildUSLayout()
	l.Name = "DE"
	rMap := make(map[rune][]KeyboardReport)
	for k, v := range l.Runes {
		rMap[k] = v
	}

	// Y and Z swap in QWERTZ
	rMap['z'] = []KeyboardReport{NewKeyboardReport(0, KeyY)}
	rMap['Z'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyY)}
	rMap['y'] = []KeyboardReport{NewKeyboardReport(0, KeyZ)}
	rMap['Y'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyZ)}

	// Umlauts & German specific
	rMap['ä'] = []KeyboardReport{NewKeyboardReport(0, KeyApostrophe)}
	rMap['Ä'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyApostrophe)}
	rMap['ö'] = []KeyboardReport{NewKeyboardReport(0, KeySemicolon)}
	rMap['Ö'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeySemicolon)}
	rMap['ü'] = []KeyboardReport{NewKeyboardReport(0, KeyLeftBrace)}
	rMap['Ü'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyLeftBrace)}
	rMap['ß'] = []KeyboardReport{NewKeyboardReport(0, KeyMinus)}

	// Symbols for DE layout
	rMap['!'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key1)}
	rMap['"'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key2)}
	rMap['§'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key3)}
	rMap['$'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key4)}
	rMap['%'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key5)}
	rMap['&'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key6)}
	rMap['/'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key7)}
	rMap['('] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key8)}
	rMap[')'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key9)}
	rMap['='] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key0)}

	rMap['?'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyEqual)}
	rMap['+'] = []KeyboardReport{NewKeyboardReport(0, KeyRightBrace)}
	rMap['*'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyRightBrace)}
	rMap['~'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyRightBrace)}
	rMap['#'] = []KeyboardReport{NewKeyboardReport(0, KeyHashtilde)}
	rMap['\''] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyHashtilde)}
	rMap[';'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyComma)}
	rMap[':'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyDot)}
	rMap['-'] = []KeyboardReport{NewKeyboardReport(0, KeySlash)}
	rMap['_'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeySlash)}
	rMap['<'] = []KeyboardReport{NewKeyboardReport(0, KeyNonUSBackslash)}
	rMap['>'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyNonUSBackslash)}
	rMap['|'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyNonUSBackslash)}
	rMap['@'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyQ)}
	rMap['€'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyE)}
	rMap['\\'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyMinus)}
	rMap['['] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key8)}
	rMap[']'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key9)}
	rMap['{'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key7)}
	rMap['}'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key0)}

	l.Runes = rMap
	return l
}

func buildESLayout() *Layout {
	l := buildUSLayout()
	l.Name = "ES"
	rMap := make(map[rune][]KeyboardReport)
	for k, v := range l.Runes {
		rMap[k] = v
	}

	// Spanish specific
	rMap['ñ'] = []KeyboardReport{NewKeyboardReport(0, KeySemicolon)}
	rMap['Ñ'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeySemicolon)}
	rMap['ç'] = []KeyboardReport{NewKeyboardReport(0, KeyRightBrace)}
	rMap['Ç'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyRightBrace)}
	rMap['ª'] = []KeyboardReport{NewKeyboardReport(0, KeyGrave)}
	rMap['º'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyGrave)}

	rMap['!'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key1)}
	rMap['"'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key2)}
	rMap['·'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key3)}
	rMap['$'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key4)}
	rMap['%'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key5)}
	rMap['&'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key6)}
	rMap['/'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key7)}
	rMap['('] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key8)}
	rMap[')'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key9)}
	rMap['='] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key0)}

	rMap['?'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyMinus)}
	rMap['¿'] = []KeyboardReport{NewKeyboardReport(0, KeyMinus)}
	rMap['\''] = []KeyboardReport{NewKeyboardReport(0, KeyEqual)}
	rMap['¡'] = []KeyboardReport{NewKeyboardReport(0, KeyEqual)}
	rMap['+'] = []KeyboardReport{NewKeyboardReport(0, KeyLeftBrace)}
	rMap['*'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyLeftBrace)}
	rMap[']'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyLeftBrace)}
	rMap['`'] = []KeyboardReport{NewKeyboardReport(0, KeyApostrophe)}
	rMap['^'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyApostrophe)}
	rMap['['] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyApostrophe)}
	rMap['}'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyHashtilde)}

	rMap['-'] = []KeyboardReport{NewKeyboardReport(0, KeySlash)}
	rMap['_'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeySlash)}
	rMap[';'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyComma)}
	rMap[':'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyDot)}
	rMap['<'] = []KeyboardReport{NewKeyboardReport(0, KeyNonUSBackslash)}
	rMap['>'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyNonUSBackslash)}
	rMap['@'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key2)}
	rMap['#'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key3)}
	rMap['€'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyE)}
	rMap['~'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key4)}
	rMap['\\'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyGrave)}

	l.Runes = rMap
	return l
}

func buildFRLayout() *Layout {
	l := buildUSLayout()
	l.Name = "FR"
	rMap := make(map[rune][]KeyboardReport)
	for k, v := range l.Runes {
		rMap[k] = v
	}

	// AZERTY letter swaps
	rMap['a'] = []KeyboardReport{NewKeyboardReport(0, KeyQ)}
	rMap['A'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyQ)}
	rMap['z'] = []KeyboardReport{NewKeyboardReport(0, KeyW)}
	rMap['Z'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyW)}
	rMap['q'] = []KeyboardReport{NewKeyboardReport(0, KeyA)}
	rMap['Q'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyA)}
	rMap['w'] = []KeyboardReport{NewKeyboardReport(0, KeyZ)}
	rMap['W'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyZ)}
	rMap['m'] = []KeyboardReport{NewKeyboardReport(0, KeySemicolon)}
	rMap['M'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeySemicolon)}

	// French digits require shift
	rMap['1'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key1)}
	rMap['2'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key2)}
	rMap['3'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key3)}
	rMap['4'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key4)}
	rMap['5'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key5)}
	rMap['6'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key6)}
	rMap['7'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key7)}
	rMap['8'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key8)}
	rMap['9'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key9)}
	rMap['0'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, Key0)}

	// Unshifted number keys in FR
	rMap['&'] = []KeyboardReport{NewKeyboardReport(0, Key1)}
	rMap['é'] = []KeyboardReport{NewKeyboardReport(0, Key2)}
	rMap['"'] = []KeyboardReport{NewKeyboardReport(0, Key3)}
	rMap['\''] = []KeyboardReport{NewKeyboardReport(0, Key4)}
	rMap['('] = []KeyboardReport{NewKeyboardReport(0, Key5)}
	rMap['-'] = []KeyboardReport{NewKeyboardReport(0, Key6)}
	rMap['è'] = []KeyboardReport{NewKeyboardReport(0, Key7)}
	rMap['_'] = []KeyboardReport{NewKeyboardReport(0, Key8)}
	rMap['ç'] = []KeyboardReport{NewKeyboardReport(0, Key9)}
	rMap['à'] = []KeyboardReport{NewKeyboardReport(0, Key0)}

	rMap[')'] = []KeyboardReport{NewKeyboardReport(0, KeyMinus)}
	rMap['='] = []KeyboardReport{NewKeyboardReport(0, KeyEqual)}
	rMap['+'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyEqual)}
	rMap['$'] = []KeyboardReport{NewKeyboardReport(0, KeyRightBrace)}
	rMap['£'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyRightBrace)}
	rMap['*'] = []KeyboardReport{NewKeyboardReport(0, KeyHashtilde)}
	rMap['µ'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyHashtilde)}

	rMap[','] = []KeyboardReport{NewKeyboardReport(0, KeyM)}
	rMap['?'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyM)}
	rMap[';'] = []KeyboardReport{NewKeyboardReport(0, KeyComma)}
	rMap['.'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyComma)}
	rMap[':'] = []KeyboardReport{NewKeyboardReport(0, KeyDot)}
	rMap['/'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyDot)}
	rMap['!'] = []KeyboardReport{NewKeyboardReport(0, KeySlash)}
	rMap['§'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeySlash)}

	rMap['<'] = []KeyboardReport{NewKeyboardReport(0, KeyNonUSBackslash)}
	rMap['>'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyNonUSBackslash)}
	rMap['@'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key0)}
	rMap['€'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyE)}
	rMap['~'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, KeyN)}
	rMap['#'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key3)}
	rMap['{'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key4)}
	rMap['['] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key5)}
	rMap['|'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key6)}
	rMap['`'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key7)}
	rMap['\\'] = []KeyboardReport{NewKeyboardReport(ModRightAlt, Key8)}
	rMap['ù'] = []KeyboardReport{NewKeyboardReport(0, KeyApostrophe)}
	rMap['%'] = []KeyboardReport{NewKeyboardReport(ModLeftShift, KeyApostrophe)}

	l.Runes = rMap
	return l
}
