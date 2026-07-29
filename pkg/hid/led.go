package hid

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	LEDNumLock    uint8 = 1 << 0 // 0x01
	LEDCapsLock   uint8 = 1 << 1 // 0x02
	LEDScrollLock uint8 = 1 << 2 // 0x04
	LEDCompose    uint8 = 1 << 3 // 0x08
	LEDKana       uint8 = 1 << 4 // 0x10
	LEDAny        uint8 = LEDNumLock | LEDCapsLock | LEDScrollLock
)

// LEDState holds the status of host-driven keyboard LEDs.
type LEDState struct {
	NumLock    bool `json:"numLock"`
	CapsLock   bool `json:"capsLock"`
	ScrollLock bool `json:"scrollLock"`
}

// GetState returns current LED lock status.
func (w *LEDWatcher) GetState() LEDState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return LEDState{
		NumLock:    (w.current & LEDNumLock) != 0,
		CapsLock:   (w.current & LEDCapsLock) != 0,
		ScrollLock: (w.current & LEDScrollLock) != 0,
	}
}

// Subscribe returns a channel that receives LED state updates continuously,
// along with an unsubscribe function.
func (w *LEDWatcher) Subscribe() (<-chan LEDState, func()) {
	ch := make(chan uint8, 16)

	w.mu.Lock()
	w.listeners[ch] = 0
	w.mu.Unlock()

	out := make(chan LEDState, 16)
	ctx, cancel := context.WithCancel(w.ctx)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case val, ok := <-ch:
				if !ok {
					return
				}
				state := LEDState{
					NumLock:    (val & LEDNumLock) != 0,
					CapsLock:   (val & LEDCapsLock) != 0,
					ScrollLock: (val & LEDScrollLock) != 0,
				}
				select {
				case out <- state:
				default:
				}
			}
		}
	}()

	unsubscribe := func() {
		cancel()
		w.mu.Lock()
		delete(w.listeners, ch)
		w.mu.Unlock()
	}

	return out, unsubscribe
}

// LEDWatcher monitors `/dev/hidg0` for host LED state updates.
type LEDWatcher struct {
	mu         sync.Mutex
	reader     io.Reader
	ownsReader bool
	running    bool
	current    uint8
	listeners  map[chan uint8]uint8 // channel -> mask
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewLEDWatcher initializes an LEDWatcher reading from devicePath.
func NewLEDWatcher(ctx context.Context, devicePath string) (*LEDWatcher, error) {
	ctx, cancel := context.WithCancel(ctx)
	w := &LEDWatcher{
		listeners: make(map[chan uint8]uint8),
		ctx:       ctx,
		cancel:    cancel,
	}

	if devicePath != "" {
		f, err := os.Open(devicePath)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("opening LED watcher device %q: %w", devicePath, err)
		}
		w.reader = f
		w.ownsReader = true
		w.running = true
		go w.readLoop()
	}

	return w, nil
}

// SetReader sets a custom reader (for testing or non-file sources).
func (w *LEDWatcher) SetReader(r io.Reader) {
	w.mu.Lock()
	if w.ownsReader && w.reader != nil {
		if closer, ok := w.reader.(io.Closer); ok {
			_ = closer.Close()
		}
	}
	w.reader = r
	w.ownsReader = false
	shouldStart := !w.running
	w.running = true
	w.mu.Unlock()

	if shouldStart {
		go w.readLoop()
	}
}

func (w *LEDWatcher) readLoop() {
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	buf := make([]byte, 8)
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		w.mu.Lock()
		r := w.reader
		w.mu.Unlock()

		if r == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		n, err := r.Read(buf)
		if err != nil {
			return
		}

		if n > 0 {
			newState := buf[0]
			w.UpdateState(newState)
		}
	}
}

// UpdateState manually updates LED state and notifies matching subscribers.
func (w *LEDWatcher) UpdateState(newState uint8) {
	w.mu.Lock()
	w.current = newState
	for ch, mask := range w.listeners {
		if mask == 0 || (newState&mask) != 0 {
			select {
			case ch <- newState:
			default:
			}
		}
	}
	w.mu.Unlock()
}

// ParseLEDMask converts string filters ("NUM", "CAPS", "SCROLL", "ANY") or numeric strings to bitmasks.
func ParseLEDMask(filter string) (uint8, error) {
	upper := strings.ToUpper(strings.TrimSpace(filter))
	switch upper {
	case "NUM", "NUMLOCK":
		return LEDNumLock, nil
	case "CAPS", "CAPSLOCK":
		return LEDCapsLock, nil
	case "SCROLL", "SCROLLLOCK":
		return LEDScrollLock, nil
	case "ANY", "":
		return LEDAny, nil
	}

	var mask uint8
	if strings.Contains(upper, "NUM") {
		mask |= LEDNumLock
	}
	if strings.Contains(upper, "CAPS") {
		mask |= LEDCapsLock
	}
	if strings.Contains(upper, "SCROLL") {
		mask |= LEDScrollLock
	}
	if mask == 0 {
		return 0, fmt.Errorf("unrecognized LED mask string %q", filter)
	}
	return mask, nil
}

// WaitLED waits for a host LED state change matching mask within the given timeout.
func (w *LEDWatcher) WaitLED(ctx context.Context, mask uint8, timeout time.Duration) (LEDState, error) {
	ch := make(chan uint8, 1)

	w.mu.Lock()
	w.listeners[ch] = mask
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		delete(w.listeners, ch)
		w.mu.Unlock()
	}()

	var timeoutChan <-chan time.Time
	if timeout > 0 {
		timeoutChan = time.After(timeout)
	}

	select {
	case <-ctx.Done():
		return LEDState{}, ctx.Err()
	case <-timeoutChan:
		return LEDState{}, errors.New("timeout waiting for LED state change")
	case val := <-ch:
		return LEDState{
			NumLock:    (val & LEDNumLock) != 0,
			CapsLock:   (val & LEDCapsLock) != 0,
			ScrollLock: (val & LEDScrollLock) != 0,
		}, nil
	}
}

// Close stops the LEDWatcher.
func (w *LEDWatcher) Close() error {
	w.cancel()
	w.mu.Lock()
	defer w.mu.Unlock()
	var closeErr error
	if w.ownsReader && w.reader != nil {
		if closer, ok := w.reader.(io.Closer); ok {
			closeErr = closer.Close()
		}
		w.reader = nil
	}
	w.running = false
	return closeErr
}
