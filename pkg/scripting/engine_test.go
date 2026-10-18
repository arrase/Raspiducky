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

	engine := NewScriptEngine(nil, mouse, nil)
	var logBuf bytes.Buffer

	js := `
mouseMove(200, -300);
mouseClick();
`
	if err := engine.RunJS(context.Background(), js, &logBuf); err != nil {
		t.Fatalf("RunJS failed: %v", err)
	}

	data := mouseBuf.Bytes()
	// 1st report (6 bytes): Move(127, -127)
	// 2nd report (6 bytes): ButtonDown("left")
	// 3rd report (6 bytes): ButtonUp("left")
	if len(data) != 18 {
		t.Fatalf("expected 18 bytes of mouse reports, got %d", len(data))
	}

	if int8(data[2]) != 127 || int8(data[3]) != -127 {
		t.Errorf("expected clamped move (127, -127), got (%d, %d)", int8(data[2]), int8(data[3]))
	}
	if data[6+1] != hid.MouseButtonLeft {
		t.Errorf("expected default mouseClick() to press left button (0x01), got 0x%02x", data[6+1])
	}
	if data[12+1] != 0x00 {
		t.Errorf("expected mouseClick() release report to have buttons 0x00, got 0x%02x", data[12+1])
	}
}

func TestScriptEngine_WaitLEDNoArgs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ledWatcher := hid.NewLEDWatcher(ctx, "")
	defer ledWatcher.Close()

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
	// 3 actions, each writes press (8 bytes) + release (8 bytes) = 48 bytes total
	if len(data) != 48 {
		t.Fatalf("expected 48 bytes of keyboard reports, got %d", len(data))
	}

	// 1st press: "-" -> KeyMinus, no modifiers
	if data[0] != 0 || data[2] != hid.KeyMinus {
		t.Errorf("expected '-' report (mod=0, key=KeyMinus), got (mod=0x%02x, key=0x%02x)", data[0], data[2])
	}
	// 2nd press (offset 16): "CTRL -" -> ModLeftCtrl + KeyMinus
	if data[16] != hid.ModLeftCtrl || data[18] != hid.KeyMinus {
		t.Errorf("expected 'CTRL -' report (mod=ModLeftCtrl, key=KeyMinus), got (mod=0x%02x, key=0x%02x)", data[16], data[18])
	}
	// 3rd press (offset 32): "STRING  " -> KeySpace
	if data[32] != 0 || data[34] != hid.KeySpace {
		t.Errorf("expected 'STRING  ' report (mod=0, key=KeySpace), got (mod=0x%02x, key=0x%02x)", data[32], data[34])
	}
}
