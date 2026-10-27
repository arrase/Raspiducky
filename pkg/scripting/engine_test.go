package scripting

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
)

func TestScriptEngine_MouseClickAndMoveClamping(t *testing.T) {
	var mouseBuf bytes.Buffer
	mouse := hid.NewMouse("")
	mouse.SetWriter(&mouseBuf)

	var kbdBuf bytes.Buffer
	kbd, _ := hid.NewKeyboard("", "US")
	kbd.SetWriter(&kbdBuf)

	engine := NewScriptEngine(kbd, mouse, nil)
	var logBuf bytes.Buffer

	js := `
type();
press();
hold();
release();
mouseMove(200, -300);
mouseMoveTo(-50, 50000);
mouseClick();
`
	if err := engine.RunJS(context.Background(), js, &logBuf); err != nil {
		t.Fatalf("RunJS failed: %v", err)
	}

	if kbdBuf.Len() != 0 {
		t.Errorf("expected type() and press() with no args to write 0 bytes, got %d", kbdBuf.Len())
	}

	data := mouseBuf.Bytes()
	// 1st report (6 bytes): Move(127, -127)
	// 2nd report (6 bytes): MoveTo(0, 32767)
	// 3rd report (6 bytes): ButtonDown("left")
	// 4th report (6 bytes): ButtonUp("left")
	if len(data) != 24 {
		t.Fatalf("expected 24 bytes of mouse reports, got %d", len(data))
	}

	if int8(data[2]) != 127 || int8(data[3]) != -127 {
		t.Errorf("expected clamped move (127, -127), got (%d, %d)", int8(data[2]), int8(data[3]))
	}
	if data[6] != 0x02 || data[8] != 0 || data[9] != 0 || data[10] != 0xff || data[11] != 0x7f {
		t.Errorf("expected clamped MoveTo (0, 32767), got report %v", data[6:12])
	}
	if data[12+1] != hid.MouseButtonLeft {
		t.Errorf("expected default mouseClick() to press left button (0x01), got 0x%02x", data[12+1])
	}
	if data[18+1] != 0x00 {
		t.Errorf("expected mouseClick() release report to have buttons 0x00, got 0x%02x", data[18+1])
	}
}

func TestScriptEngine_WaitLEDNoArgs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ledWatcher := hid.NewLEDWatcher(ctx, "")
	defer func() { _ = ledWatcher.Close() }()

	engine := NewScriptEngine(nil, nil, ledWatcher)
	var logBuf bytes.Buffer

	go func() {
		time.Sleep(30 * time.Millisecond)
		ledWatcher.UpdateState(hid.LEDCapsLock)
	}()

	js := `
const state = waitLED();
console.log("caps:", state.capsLock);
`
	if err := engine.RunJS(ctx, js, &logBuf); err != nil {
		t.Fatalf("RunJS waitLED() failed: %v", err)
	}

	if got := logBuf.String(); got != "caps: true\n" {
		t.Errorf("expected log %q, got %q", "caps: true\n", got)
	}
}

func TestScriptEngine_DuckyScriptMinusKeyAndTrailingSpace(t *testing.T) {
	var kbdBuf bytes.Buffer
	kbd, err := hid.NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	kbd.SetWriter(&kbdBuf)
	kbd.SetTypingSpeed(0, 0)

	engine := NewScriptEngine(kbd, nil, nil)
	var logBuf bytes.Buffer

	ducky := "-\nCTRL -\nSTRING  \n"
	if err := engine.RunDuckyScript(context.Background(), ducky, &logBuf); err != nil {
		t.Fatalf("RunDuckyScript failed: %v", err)
	}

	data := kbdBuf.Bytes()
	// 1st action ("-"): press + release = 16B
	// 2nd action ("CTRL -"): mod + mod+key + release = 24B
	// 3rd action ("STRING  "): press + release = 16B
	// Total = 56 bytes
	if len(data) != 56 {
		t.Fatalf("expected 56 bytes of keyboard reports, got %d", len(data))
	}

	// 1st press: "-" -> KeyMinus, no modifiers
	if data[0] != 0 || data[2] != hid.KeyMinus {
		t.Errorf("expected '-' report (mod=0, key=KeyMinus), got (mod=0x%02x, key=0x%02x)", data[0], data[2])
	}
	// 2nd press (offset 16): "CTRL -" -> modifier report first, then ModLeftCtrl + KeyMinus
	if data[16] != hid.ModLeftCtrl || data[18] != 0 {
		t.Errorf("expected 'CTRL -' modifier report (mod=ModLeftCtrl, key=0), got (mod=0x%02x, key=0x%02x)", data[16], data[18])
	}
	if data[24] != hid.ModLeftCtrl || data[26] != hid.KeyMinus {
		t.Errorf("expected 'CTRL -' combo report (mod=ModLeftCtrl, key=KeyMinus), got (mod=0x%02x, key=0x%02x)", data[24], data[26])
	}
	// 3rd press (offset 40): "STRING  " -> KeySpace
	if data[40] != 0 || data[42] != hid.KeySpace {
		t.Errorf("expected 'STRING  ' report (mod=0, key=KeySpace), got (mod=0x%02x, key=0x%02x)", data[40], data[42])
	}
}

func TestScriptEngine_DuckyScriptQuotes(t *testing.T) {
	var kbdBuf bytes.Buffer
	kbd, err := hid.NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	kbd.SetWriter(&kbdBuf)
	kbd.SetTypingSpeed(0, 0)

	engine := NewScriptEngine(kbd, nil, nil)
	var logBuf bytes.Buffer

	ducky := "STRING '\"‘’“”\n"
	if err := engine.RunDuckyScript(context.Background(), ducky, &logBuf); err != nil {
		t.Fatalf("RunDuckyScript failed: %v", err)
	}

	data := kbdBuf.Bytes()
	// 6 characters, each writes press (8 bytes) + release (8 bytes) = 96 bytes total
	if len(data) != 96 {
		t.Fatalf("expected 96 bytes of keyboard reports, got %d", len(data))
	}

	wantMods := []uint8{0, hid.ModLeftShift, 0, 0, hid.ModLeftShift, hid.ModLeftShift}
	for i, wantMod := range wantMods {
		offset := i * 16
		if data[offset] != wantMod || data[offset+2] != hid.KeyApostrophe {
			t.Errorf("char %d: expected (mod=0x%02x, key=KeyApostrophe), got (mod=0x%02x, key=0x%02x)",
				i, wantMod, data[offset], data[offset+2])
		}
	}
}

func TestScriptEngine_DuckyScriptHoldAndRelease(t *testing.T) {
	var kbdBuf bytes.Buffer
	kbd, err := hid.NewKeyboard("", "US")
	if err != nil {
		t.Fatalf("NewKeyboard failed: %v", err)
	}
	kbd.SetWriter(&kbdBuf)
	kbd.SetTypingSpeed(0, 0)

	engine := NewScriptEngine(kbd, nil, nil)
	var logBuf bytes.Buffer

	ducky := "HOLD SHIFT\nDOWNARROW\nRELEASE SHIFT\n"
	if err := engine.RunDuckyScript(context.Background(), ducky, &logBuf); err != nil {
		t.Fatalf("RunDuckyScript failed: %v", err)
	}

	data := kbdBuf.Bytes()
	// 1st report (8B): HOLD SHIFT -> (ModLeftShift, 0)
	// 2nd report (8B): DOWNARROW press -> (ModLeftShift, KeyDown)
	// 3rd report (8B): DOWNARROW release -> (ModLeftShift, 0)
	// 4th report (8B): RELEASE SHIFT -> (0, 0)
	if len(data) != 32 {
		t.Fatalf("expected 32 bytes of keyboard reports, got %d", len(data))
	}

	if data[0] != hid.ModLeftShift || data[2] != 0 {
		t.Errorf("expected HOLD SHIFT report (ModLeftShift, 0), got (0x%02x, 0x%02x)", data[0], data[2])
	}
	if data[8] != hid.ModLeftShift || data[10] != hid.KeyDown {
		t.Errorf("expected DOWNARROW press report (ModLeftShift, KeyDown), got (0x%02x, 0x%02x)", data[8], data[10])
	}
	if data[16] != hid.ModLeftShift || data[18] != 0 {
		t.Errorf("expected DOWNARROW release report (ModLeftShift, 0), got (0x%02x, 0x%02x)", data[16], data[18])
	}
	if data[24] != 0 || data[26] != 0 {
		t.Errorf("expected RELEASE SHIFT report (0, 0), got (0x%02x, 0x%02x)", data[24], data[26])
	}
}
