package hid

// KeyboardReport represents an 8-byte USB HID keyboard report:
// Byte 0: Modifiers bitmask
// Byte 1: Reserved (0x00)
// Bytes 2-7: Keycodes (up to 6 keys)
type KeyboardReport [8]byte

// NewKeyboardReport creates a KeyboardReport with given modifiers and keycodes.
func NewKeyboardReport(modifiers uint8, keycodes ...uint8) KeyboardReport {
	var r KeyboardReport
	r[0] = modifiers
	r[1] = 0x00
	for i := 0; i < len(keycodes) && i < 6; i++ {
		r[2+i] = keycodes[i]
	}
	return r
}

// EmptyKeyboardReport returns a zeroed KeyboardReport (key release).
func EmptyKeyboardReport() KeyboardReport {
	return KeyboardReport{}
}

// Modifiers returns the modifier bitmask of the report.
func (r KeyboardReport) Modifiers() uint8 {
	return r[0]
}

// KeyCodes returns the keycodes slice (non-zero entries).
func (r KeyboardReport) KeyCodes() []uint8 {
	var keys []uint8
	for i := 2; i < 8; i++ {
		if r[i] != 0 {
			keys = append(keys, r[i])
		}
	}
	return keys
}
