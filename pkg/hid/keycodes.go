package hid

// USB HID Keyboard Modifiers
const (
	ModLeftCtrl   uint8 = 0x01
	ModLeftShift  uint8 = 0x02
	ModLeftAlt    uint8 = 0x04
	ModLeftGUI    uint8 = 0x08
	ModRightCtrl  uint8 = 0x10
	ModRightShift uint8 = 0x20
	ModRightAlt   uint8 = 0x40 // AltGr in many European layouts
	ModRightGUI   uint8 = 0x80
)

// USB HID Keyboard Keycodes (Usage Page 0x07)
const (
	KeyReserved       uint8 = 0x00
	KeyErrorRollover  uint8 = 0x01
	KeyPostFail       uint8 = 0x02
	KeyErrorUndefined uint8 = 0x03

	KeyA uint8 = 0x04
	KeyB uint8 = 0x05
	KeyC uint8 = 0x06
	KeyD uint8 = 0x07
	KeyE uint8 = 0x08
	KeyF uint8 = 0x09
	KeyG uint8 = 0x0a
	KeyH uint8 = 0x0b
	KeyI uint8 = 0x0c
	KeyJ uint8 = 0x0d
	KeyK uint8 = 0x0e
	KeyL uint8 = 0x0f
	KeyM uint8 = 0x10
	KeyN uint8 = 0x11
	KeyO uint8 = 0x12
	KeyP uint8 = 0x13
	KeyQ uint8 = 0x14
	KeyR uint8 = 0x15
	KeyS uint8 = 0x16
	KeyT uint8 = 0x17
	KeyU uint8 = 0x18
	KeyV uint8 = 0x19
	KeyW uint8 = 0x1a
	KeyX uint8 = 0x1b
	KeyY uint8 = 0x1c
	KeyZ uint8 = 0x1d

	Key1 uint8 = 0x1e
	Key2 uint8 = 0x1f
	Key3 uint8 = 0x20
	Key4 uint8 = 0x21
	Key5 uint8 = 0x22
	Key6 uint8 = 0x23
	Key7 uint8 = 0x24
	Key8 uint8 = 0x25
	Key9 uint8 = 0x26
	Key0 uint8 = 0x27

	KeyEnter      uint8 = 0x28
	KeyEsc        uint8 = 0x29
	KeyBackspace  uint8 = 0x2a
	KeyTab        uint8 = 0x2b
	KeySpace      uint8 = 0x2c
	KeyMinus      uint8 = 0x2d
	KeyEqual      uint8 = 0x2e
	KeyLeftBrace  uint8 = 0x2f
	KeyRightBrace uint8 = 0x30
	KeyBackslash  uint8 = 0x31
	KeyHashtilde  uint8 = 0x32
	KeySemicolon  uint8 = 0x33
	KeyApostrophe uint8 = 0x34
	KeyGrave      uint8 = 0x35
	KeyComma      uint8 = 0x36
	KeyDot        uint8 = 0x37
	KeySlash      uint8 = 0x38
	KeyCapsLock   uint8 = 0x39

	KeyF1  uint8 = 0x3a
	KeyF2  uint8 = 0x3b
	KeyF3  uint8 = 0x3c
	KeyF4  uint8 = 0x3d
	KeyF5  uint8 = 0x3e
	KeyF6  uint8 = 0x3f
	KeyF7  uint8 = 0x40
	KeyF8  uint8 = 0x41
	KeyF9  uint8 = 0x42
	KeyF10 uint8 = 0x43
	KeyF11 uint8 = 0x44
	KeyF12 uint8 = 0x45

	KeyPrintScreen uint8 = 0x46
	KeyScrollLock  uint8 = 0x47
	KeyPause       uint8 = 0x48
	KeyInsert      uint8 = 0x49
	KeyHome        uint8 = 0x4a
	KeyPageUp      uint8 = 0x4b
	KeyDelete      uint8 = 0x4c
	KeyEnd         uint8 = 0x4d
	KeyPageDown    uint8 = 0x4e
	KeyRight       uint8 = 0x4f
	KeyLeft        uint8 = 0x50
	KeyDown        uint8 = 0x51
	KeyUp          uint8 = 0x52

	KeyNumLock uint8 = 0x53
	KeyKpSlash uint8 = 0x54
	KeyKpStar  uint8 = 0x55
	KeyKpMinus uint8 = 0x56
	KeyKpPlus  uint8 = 0x57
	KeyKpEnter uint8 = 0x58

	KeyNonUSBackslash uint8 = 0x64
	KeyApplication    uint8 = 0x65

	KeyLeftCtrl   uint8 = 0xe0
	KeyLeftShift  uint8 = 0xe1
	KeyLeftAlt    uint8 = 0xe2
	KeyLeftGUI    uint8 = 0xe3
	KeyRightCtrl  uint8 = 0xe4
	KeyRightShift uint8 = 0xe5
	KeyRightAlt   uint8 = 0xe6
	KeyRightGUI   uint8 = 0xe7
)
