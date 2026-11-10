package scripting

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
	"github.com/dop251/goja"
)

// ScriptEngine executes DuckyScript and JavaScript payloads against HID devices.
type ScriptEngine struct {
	keyboard   *hid.Keyboard
	mouse      *hid.Mouse
	ledWatcher *hid.LEDWatcher
}

// NewScriptEngine initializes a ScriptEngine with provided HID devices.
func NewScriptEngine(kbd *hid.Keyboard, mouse *hid.Mouse, ledWatcher *hid.LEDWatcher) *ScriptEngine {
	return &ScriptEngine{
		keyboard:   kbd,
		mouse:      mouse,
		ledWatcher: ledWatcher,
	}
}

func (e *ScriptEngine) releaseAllBestEffort() {
	if e.keyboard != nil {
		_ = e.keyboard.ReleaseHeldKeys()
	}
	if e.mouse != nil {
		_ = e.mouse.ReleaseButtons()
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func clampUint15(v int64) uint16 {
	if v < 0 {
		return 0
	}
	if v > hid.MaxAbsCoord {
		return hid.MaxAbsCoord
	}
	return uint16(v)
}

func throwOrInterrupt(ctx context.Context, vm *goja.Runtime, err error) {
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		vm.Interrupt(err)
		return
	}
	panic(vm.ToValue(err.Error()))
}

// RunJS runs JavaScript code with injected HID functions within the given context.
func (e *ScriptEngine) RunJS(ctx context.Context, jsCode string, logWriter io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	defer e.releaseAllBestEffort()

	vm := goja.New()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt(ctx.Err())
		case <-done:
		}
	}()

	e.injectKeyboard(ctx, vm)
	e.injectMouse(ctx, vm)
	e.injectLED(ctx, vm)
	e.injectUtils(ctx, vm, logWriter)

	if _, err := vm.RunString(jsCode); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		var interrupted *goja.InterruptedError
		if errors.As(err, &interrupted) {
			return fmt.Errorf("script interrupted: %w", err)
		}
		return fmt.Errorf("script execution error: %w", err)
	}
	return nil
}

func (e *ScriptEngine) injectKeyboard(ctx context.Context, vm *goja.Runtime) {
	bindKeyFn := func(name string, fn func(context.Context, string) error) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) == 0 || goja.IsUndefined(call.Argument(0)) || goja.IsNull(call.Argument(0)) {
				return goja.Undefined()
			}
			if e.keyboard == nil {
				panic(vm.ToValue(name + ": keyboard device not initialized"))
			}
			if err := fn(ctx, call.Argument(0).String()); err != nil {
				throwOrInterrupt(ctx, vm, err)
			}
			return goja.Undefined()
		}
	}

	var typeFn, pressFn, holdFn, releaseFn func(context.Context, string) error
	if e.keyboard != nil {
		typeFn = e.keyboard.TypeString
		pressFn = e.keyboard.Press
		holdFn = e.keyboard.Hold
		releaseFn = e.keyboard.Release
	}

	_ = vm.Set("type", bindKeyFn("type", typeFn))
	_ = vm.Set("press", bindKeyFn("press", pressFn))
	_ = vm.Set("hold", bindKeyFn("hold", holdFn))
	_ = vm.Set("release", bindKeyFn("release", releaseFn))

	_ = vm.Set("layout", func(call goja.FunctionCall) goja.Value {
		if e.keyboard == nil {
			panic(vm.ToValue("layout: keyboard device not initialized"))
		}
		if err := e.keyboard.SetLayout(call.Argument(0).String()); err != nil {
			throwOrInterrupt(ctx, vm, err)
		}
		return goja.Undefined()
	})

	_ = vm.Set("typingSpeed", func(call goja.FunctionCall) goja.Value {
		if e.keyboard == nil {
			panic(vm.ToValue("typingSpeed: keyboard device not initialized"))
		}
		d := int(call.Argument(0).ToInteger())
		j := int(call.Argument(1).ToInteger())
		e.keyboard.SetTypingSpeed(d, j)
		return goja.Undefined()
	})
}

func (e *ScriptEngine) injectMouse(ctx context.Context, vm *goja.Runtime) {
	_ = vm.Set("mouseMove", func(call goja.FunctionCall) goja.Value {
		if e.mouse == nil {
			panic(vm.ToValue("mouseMove: mouse device not initialized"))
		}
		dx := int(call.Argument(0).ToInteger())
		dy := int(call.Argument(1).ToInteger())
		if err := e.mouse.MoveContext(ctx, dx, dy); err != nil {
			throwOrInterrupt(ctx, vm, err)
		}
		return goja.Undefined()
	})

	_ = vm.Set("mouseMoveTo", func(call goja.FunctionCall) goja.Value {
		if e.mouse == nil {
			panic(vm.ToValue("mouseMoveTo: mouse device not initialized"))
		}
		x := clampUint15(call.Argument(0).ToInteger())
		y := clampUint15(call.Argument(1).ToInteger())
		if err := e.mouse.MoveToContext(ctx, x, y); err != nil {
			throwOrInterrupt(ctx, vm, err)
		}
		return goja.Undefined()
	})

	_ = vm.Set("mouseClick", func(call goja.FunctionCall) goja.Value {
		if e.mouse == nil {
			panic(vm.ToValue("mouseClick: mouse device not initialized"))
		}
		btn := "left"
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Argument(0)) && !goja.IsNull(call.Argument(0)) {
			btn = call.Argument(0).String()
		}
		if err := e.mouse.ClickContext(ctx, btn); err != nil {
			throwOrInterrupt(ctx, vm, err)
		}
		return goja.Undefined()
	})
}

func (e *ScriptEngine) injectLED(ctx context.Context, vm *goja.Runtime) {
	_ = vm.Set("waitLED", func(call goja.FunctionCall) goja.Value {
		if e.ledWatcher == nil {
			panic(vm.ToValue("waitLED: LED watcher not initialized"))
		}
		filter := "ANY"
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Argument(0)) && !goja.IsNull(call.Argument(0)) {
			filter = call.Argument(0).String()
		}
		var timeoutMs int64 = 5000
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) && !goja.IsNull(call.Argument(1)) {
			timeoutMs = call.Argument(1).ToInteger()
		}
		if timeoutMs <= 0 {
			timeoutMs = 5000
		}

		mask, err := hid.ParseLEDMask(filter)
		if err != nil {
			throwOrInterrupt(ctx, vm, err)
		}
		state, err := e.ledWatcher.WaitLED(ctx, mask, time.Duration(timeoutMs)*time.Millisecond)
		if err != nil {
			throwOrInterrupt(ctx, vm, err)
		}
		obj := vm.NewObject()
		_ = obj.Set("numLock", state.NumLock)
		_ = obj.Set("capsLock", state.CapsLock)
		_ = obj.Set("scrollLock", state.ScrollLock)
		return obj
	})
}

func (e *ScriptEngine) injectUtils(ctx context.Context, vm *goja.Runtime, logWriter io.Writer) {
	_ = vm.Set("delay", func(call goja.FunctionCall) goja.Value {
		ms := call.Argument(0).ToInteger()
		if ms <= 0 {
			return goja.Undefined()
		}
		if err := sleepWithContext(ctx, time.Duration(ms)*time.Millisecond); err != nil {
			vm.Interrupt(err)
		}
		return goja.Undefined()
	})

	logFn := func(call goja.FunctionCall) goja.Value {
		if logWriter == nil {
			return goja.Undefined()
		}
		args := make([]string, 0, len(call.Arguments))
		for _, arg := range call.Arguments {
			args = append(args, arg.String())
		}
		_, _ = io.WriteString(logWriter, strings.Join(args, " ")+"\n")
		return goja.Undefined()
	}

	_ = vm.Set("log", logFn)
	consoleObj := vm.NewObject()
	_ = consoleObj.Set("log", logFn)
	_ = vm.Set("console", consoleObj)
}

func (e *ScriptEngine) executeKeyboardInstruction(ctx context.Context, inst Instruction) error {
	if e.keyboard == nil {
		return errors.New("keyboard device not initialized")
	}
	switch inst.Kind {
	case OpTypeString:
		return e.keyboard.TypeString(ctx, inst.Text)
	case OpPress:
		return e.keyboard.Press(ctx, inst.Text)
	case OpHold:
		return e.keyboard.Hold(ctx, inst.Text)
	case OpRelease:
		return e.keyboard.Release(ctx, inst.Text)
	case OpLayout:
		return e.keyboard.SetLayout(inst.Text)
	case OpTypingSpeed:
		e.keyboard.SetTypingSpeed(inst.Int1, inst.Int2)
		return nil
	default:
		return fmt.Errorf("unknown keyboard instruction kind %d", inst.Kind)
	}
}

func (e *ScriptEngine) executeMouseInstruction(ctx context.Context, inst Instruction) error {
	if e.mouse == nil {
		return errors.New("mouse device not initialized")
	}
	switch inst.Kind {
	case OpMouseMove:
		return e.mouse.MoveContext(ctx, inst.Int1, inst.Int2)
	case OpMouseMoveTo:
		return e.mouse.MoveToContext(ctx, clampUint15(int64(inst.Int1)), clampUint15(int64(inst.Int2)))
	case OpMouseClick:
		return e.mouse.ClickContext(ctx, inst.Text)
	default:
		return fmt.Errorf("unknown mouse instruction kind %d", inst.Kind)
	}
}

func (e *ScriptEngine) executeInstruction(ctx context.Context, inst Instruction) error {
	switch inst.Kind {
	case OpDelay:
		return sleepWithContext(ctx, time.Duration(inst.Int1)*time.Millisecond)
	case OpTypeString, OpPress, OpHold, OpRelease, OpLayout, OpTypingSpeed:
		return e.executeKeyboardInstruction(ctx, inst)
	case OpMouseMove, OpMouseMoveTo, OpMouseClick:
		return e.executeMouseInstruction(ctx, inst)
	case OpWaitLED:
		if e.ledWatcher == nil {
			return errors.New("LED watcher not initialized")
		}
		_, err := e.ledWatcher.WaitLED(ctx, inst.LEDMask, inst.Timeout)
		return err
	default:
		return fmt.Errorf("unknown instruction kind %d", inst.Kind)
	}
}

// RunDuckyScript compiles and executes DuckyScript directly without intermediate JS transpilation.
func (e *ScriptEngine) RunDuckyScript(ctx context.Context, duckyCode string, _ io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	initialLayout := "US"
	if e.keyboard != nil {
		initialLayout = e.keyboard.GetLayout()
	}

	program, err := CompileDuckyScript(duckyCode, initialLayout)
	if err != nil {
		return fmt.Errorf("duckyscript compile error: %w", err)
	}

	defer e.releaseAllBestEffort()

	for _, inst := range program {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := e.executeInstruction(ctx, inst); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("line %d: %w", inst.Line, err)
		}
		if inst.PostDelay > 0 {
			if err := sleepWithContext(ctx, time.Duration(inst.PostDelay)*time.Millisecond); err != nil {
				return err
			}
		}
	}
	return nil
}

// Run executes code as JavaScript or DuckyScript based on scriptType or fileName.
func (e *ScriptEngine) Run(ctx context.Context, scriptType, fileName, code string, logWriter io.Writer) error {
	if DetectScriptType(scriptType, fileName) == TypeJavaScript {
		return e.RunJS(ctx, code, logWriter)
	}
	return e.RunDuckyScript(ctx, code, logWriter)
}
