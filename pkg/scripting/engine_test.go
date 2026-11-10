package scripting

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
)

func TestScriptEngine_MouseClickAndMultiStepMove(t *testing.T) {
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
	// MoveContext(200, -300) -> 3 reports: (127,-127), (73,-127), (0,-46) = 18 bytes
	// MoveTo(0, 32767) -> 1 report = 6 bytes
	// Click("left") -> 2 reports = 12 bytes
	// Total = 36 bytes
	if len(data) != 36 {
		t.Fatalf("expected 36 bytes of mouse reports, got %d", len(data))
	}

	if int8(data[2]) != 127 || int8(data[8]) != 73 || int8(data[14]) != 0 {
		t.Errorf("unexpected multi-step X deltas: %d, %d, %d", int8(data[2]), int8(data[8]), int8(data[14]))
	}
	if int8(data[3]) != -127 || int8(data[9]) != -127 || int8(data[15]) != -46 {
		t.Errorf("unexpected multi-step Y deltas: %d, %d, %d", int8(data[3]), int8(data[9]), int8(data[15]))
	}
	if data[18] != 0x02 || data[20] != 0 || data[21] != 0 || data[22] != 0xff || data[23] != 0x7f {
		t.Errorf("expected clamped MoveTo (0, 32767), got report %v", data[18:24])
	}
	if data[24+1] != hid.MouseButtonLeft {
		t.Errorf("expected default mouseClick() to press left button (0x01), got 0x%02x", data[24+1])
	}
	if data[30+1] != 0x00 {
		t.Errorf("expected mouseClick() release report to have buttons 0x00, got 0x%02x", data[30+1])
	}
}

func TestScriptEngine_UncatchableCancelAndAutoKeyRelease(t *testing.T) {
	var kbdBuf bytes.Buffer
	kbd, _ := hid.NewKeyboard("", "US")
	kbd.SetWriter(&kbdBuf)
	kbd.SetTypingSpeed(0, 0)

	engine := NewScriptEngine(kbd, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	// Even when JS wraps delay() inside an infinite try/catch loop while holding SHIFT,
	// cancellation must interrupt the VM and release held SHIFT automatically.
	js := `
hold("SHIFT");
while (true) {
	try {
		delay(20);
	} catch (e) {}
}
`
	err := engine.RunJS(ctx, js, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded on cancelled infinite JS loop, got %v", err)
	}

	data := kbdBuf.Bytes()
	if len(data) < 16 {
		t.Fatalf("expected hold report + automatic release report, got %d bytes", len(data))
	}
	lastReport := data[len(data)-8:]
	if lastReport[0] != 0 || lastReport[2] != 0 {
		t.Errorf("expected held SHIFT to be released on script abort, got trailing report %v", lastReport)
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
	if len(data) != 56 {
		t.Fatalf("expected 56 bytes of keyboard reports, got %d", len(data))
	}

	if data[0] != 0 || data[2] != hid.KeyMinus {
		t.Errorf("expected '-' report (mod=0, key=KeyMinus), got (mod=0x%02x, key=0x%02x)", data[0], data[2])
	}
	if data[16] != hid.ModLeftCtrl || data[18] != 0 {
		t.Errorf("expected 'CTRL -' modifier report (mod=ModLeftCtrl, key=0), got (mod=0x%02x, key=0x%02x)", data[16], data[18])
	}
	if data[24] != hid.ModLeftCtrl || data[26] != hid.KeyMinus {
		t.Errorf("expected 'CTRL -' combo report (mod=ModLeftCtrl, key=KeyMinus), got (mod=0x%02x, key=0x%02x)", data[24], data[26])
	}
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

func TestScriptEngine_RunAndAllDuckyInstructions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var kbdBuf, mouseBuf, logBuf bytes.Buffer
	kbd, _ := hid.NewKeyboard("", "US")
	kbd.SetWriter(&kbdBuf)
	mouse := hid.NewMouse("")
	mouse.SetWriter(&mouseBuf)
	ledWatcher := hid.NewLEDWatcher(ctx, "")
	defer func() { _ = ledWatcher.Close() }()

	go func() {
		time.Sleep(25 * time.Millisecond)
		ledWatcher.UpdateState(hid.LEDNumLock)
	}()

	engine := NewScriptEngine(kbd, mouse, ledWatcher)

	ducky := `WAIT_LED NUM 500
DEFAULT_DELAY 1
LAYOUT ES
TYPING_SPEED 0 0
DELAY 1
STRINGLN ñ
MOUSE_MOVE 10 -10
MOUSE_MOVETO 100 200
MOUSE_CLICK right
`
	if err := engine.Run(ctx, "", "all_ops.ducky", ducky, &logBuf); err != nil {
		t.Fatalf("engine.Run ducky failed: %v", err)
	}
	if kbd.GetLayout() != "ES" {
		t.Errorf("expected layout ES after LAYOUT ES, got %s", kbd.GetLayout())
	}
	if mouseBuf.Len() == 0 {
		t.Errorf("expected mouse reports from DuckyScript mouse commands")
	}

	js := `
layout("DE");
typingSpeed(2, 1);
log("ready");
`
	if err := engine.Run(ctx, "", "config.js", js, &logBuf); err != nil {
		t.Fatalf("engine.Run js failed: %v", err)
	}
	if kbd.GetLayout() != "DE" {
		t.Errorf("expected layout DE after JS layout('DE'), got %s", kbd.GetLayout())
	}
	if got := logBuf.String(); got != "ready\n" {
		t.Errorf("expected log 'ready\\n', got %q", got)
	}
}
