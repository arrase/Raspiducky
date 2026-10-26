package scripting

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
	"github.com/dop251/goja"
)

// ScriptEngine executes JavaScript and DuckyScript using Goja runtime and HID devices.
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

// RunJS runs JavaScript code with injected HID functions within the given context.
func (e *ScriptEngine) RunJS(ctx context.Context, jsCode string, logWriter io.Writer) error {
	vm := goja.New()

	// Intercept execution on context cancellation
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

	_, err := vm.RunString(jsCode)
	if err != nil {
		return fmt.Errorf("script execution error: %w", err)
	}
	return nil
}

func (e *ScriptEngine) injectKeyboard(ctx context.Context, vm *goja.Runtime) {
	bindKeyFn := func(fn func(context.Context, string) error) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) == 0 || goja.IsUndefined(call.Argument(0)) || goja.IsNull(call.Argument(0)) {
				return goja.Undefined()
			}
			if err := fn(ctx, call.Argument(0).String()); err != nil {
				panic(vm.ToValue(err.Error()))
			}
			return goja.Undefined()
		}
	}

	_ = vm.Set("type", bindKeyFn(e.keyboard.TypeString))
	_ = vm.Set("press", bindKeyFn(e.keyboard.Press))
	_ = vm.Set("hold", bindKeyFn(e.keyboard.Hold))
	_ = vm.Set("release", bindKeyFn(e.keyboard.Release))

	_ = vm.Set("layout", func(call goja.FunctionCall) goja.Value {
		lang := call.Argument(0).String()
		if err := e.keyboard.SetLayout(lang); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return goja.Undefined()
	})

	_ = vm.Set("typingSpeed", func(call goja.FunctionCall) goja.Value {
		d := int(call.Argument(0).ToInteger())
		j := int(call.Argument(1).ToInteger())
		e.keyboard.SetTypingSpeed(d, j)
		return goja.Undefined()
	})
}

func clampInt8(v int64) int8 {
	if v > 127 {
		return 127
	}
	if v < -127 {
		return -127
	}
	return int8(v)
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

func (e *ScriptEngine) injectMouse(ctx context.Context, vm *goja.Runtime) {
	_ = vm.Set("mouseMove", func(call goja.FunctionCall) goja.Value {
		x := clampInt8(call.Argument(0).ToInteger())
		y := clampInt8(call.Argument(1).ToInteger())
		if err := e.mouse.Move(x, y); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return goja.Undefined()
	})

	_ = vm.Set("mouseMoveTo", func(call goja.FunctionCall) goja.Value {
		x := clampUint15(call.Argument(0).ToInteger())
		y := clampUint15(call.Argument(1).ToInteger())
		if err := e.mouse.MoveTo(x, y); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return goja.Undefined()
	})

	_ = vm.Set("mouseClick", func(call goja.FunctionCall) goja.Value {
		btn := "left"
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Argument(0)) && !goja.IsNull(call.Argument(0)) {
			btn = call.Argument(0).String()
		}
		if err := e.mouse.ClickContext(ctx, btn); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return goja.Undefined()
	})
}

func (e *ScriptEngine) injectLED(ctx context.Context, vm *goja.Runtime) {
	_ = vm.Set("waitLED", func(call goja.FunctionCall) goja.Value {
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
			panic(vm.ToValue(err.Error()))
		}
		state, err := e.ledWatcher.WaitLED(ctx, mask, time.Duration(timeoutMs)*time.Millisecond)
		if err != nil {
			panic(vm.ToValue(err.Error()))
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

		select {
		case <-ctx.Done():
			panic(vm.ToValue(ctx.Err().Error()))
		case <-time.After(time.Duration(ms) * time.Millisecond):
		}
		return goja.Undefined()
	})

	logFn := func(call goja.FunctionCall) goja.Value {
		var args []string
		for _, arg := range call.Arguments {
			args = append(args, arg.String())
		}
		msg := strings.Join(args, " ") + "\n"
		_, _ = logWriter.Write([]byte(msg))
		return goja.Undefined()
	}

	_ = vm.Set("log", logFn)
	consoleObj := vm.NewObject()
	_ = consoleObj.Set("log", logFn)
	_ = vm.Set("console", consoleObj)
}

// RunDuckyScript compiles DuckyScript source to JS and executes it.
func (e *ScriptEngine) RunDuckyScript(ctx context.Context, duckyCode string, logWriter io.Writer) error {
	jsCode, err := ParseDuckyScript(duckyCode)
	if err != nil {
		return fmt.Errorf("duckyscript compile error: %w", err)
	}
	return e.RunJS(ctx, jsCode, logWriter)
}

// Run executes code as JavaScript or DuckyScript based on scriptType or fileName.
func (e *ScriptEngine) Run(ctx context.Context, scriptType, fileName, code string, logWriter io.Writer) error {
	if DetectScriptType(scriptType, fileName) == TypeJavaScript {
		return e.RunJS(ctx, code, logWriter)
	}
	return e.RunDuckyScript(ctx, code, logWriter)
}
