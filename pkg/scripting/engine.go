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
	e.injectMouse(vm)
	e.injectLED(ctx, vm)
	e.injectUtils(ctx, vm, logWriter)

	_, err := vm.RunString(jsCode)
	if err != nil {
		return fmt.Errorf("script execution error: %w", err)
	}
	return nil
}

func (e *ScriptEngine) injectKeyboard(ctx context.Context, vm *goja.Runtime) {
	_ = vm.Set("type", func(call goja.FunctionCall) goja.Value {
		if err := ctx.Err(); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		text := call.Argument(0).String()
		if e.keyboard != nil {
			if err := e.keyboard.TypeString(ctx, text); err != nil {
				panic(vm.ToValue(err.Error()))
			}
		}
		return goja.Undefined()
	})

	_ = vm.Set("press", func(call goja.FunctionCall) goja.Value {
		if err := ctx.Err(); err != nil {
			panic(vm.ToValue(err.Error()))
		}
		keys := call.Argument(0).String()
		if e.keyboard != nil {
			if err := e.keyboard.Press(ctx, keys); err != nil {
				panic(vm.ToValue(err.Error()))
			}
		}
		return goja.Undefined()
	})

	_ = vm.Set("layout", func(call goja.FunctionCall) goja.Value {
		lang := call.Argument(0).String()
		if e.keyboard != nil {
			if err := e.keyboard.SetLayout(lang); err != nil {
				panic(vm.ToValue(err.Error()))
			}
		}
		return goja.Undefined()
	})

	_ = vm.Set("typingSpeed", func(call goja.FunctionCall) goja.Value {
		d := int(call.Argument(0).ToInteger())
		j := int(call.Argument(1).ToInteger())
		if e.keyboard != nil {
			e.keyboard.SetTypingSpeed(d, j)
		}
		return goja.Undefined()
	})
}

func (e *ScriptEngine) injectMouse(vm *goja.Runtime) {
	_ = vm.Set("mouseMove", func(call goja.FunctionCall) goja.Value {
		x := int8(call.Argument(0).ToInteger())
		y := int8(call.Argument(1).ToInteger())
		if e.mouse != nil {
			if err := e.mouse.Move(x, y); err != nil {
				panic(vm.ToValue(err.Error()))
			}
		}
		return goja.Undefined()
	})

	_ = vm.Set("mouseMoveTo", func(call goja.FunctionCall) goja.Value {
		x := uint16(call.Argument(0).ToInteger())
		y := uint16(call.Argument(1).ToInteger())
		if e.mouse != nil {
			if err := e.mouse.MoveTo(x, y); err != nil {
				panic(vm.ToValue(err.Error()))
			}
		}
		return goja.Undefined()
	})

	_ = vm.Set("mouseClick", func(call goja.FunctionCall) goja.Value {
		btn := call.Argument(0).String()
		if e.mouse != nil {
			if err := e.mouse.Click(btn); err != nil {
				panic(vm.ToValue(err.Error()))
			}
		}
		return goja.Undefined()
	})
}

func (e *ScriptEngine) injectLED(ctx context.Context, vm *goja.Runtime) {
	_ = vm.Set("waitLED", func(call goja.FunctionCall) goja.Value {
		filter := call.Argument(0).String()
		timeoutMs := call.Argument(1).ToInteger()
		if timeoutMs <= 0 {
			timeoutMs = 5000
		}

		if e.ledWatcher != nil {
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
		}
		return goja.Undefined()
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
		if logWriter != nil {
			_, _ = logWriter.Write([]byte(msg))
		}
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
