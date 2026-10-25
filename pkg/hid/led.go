package hid

import (
	"context"
	"errors"
	"fmt"
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

func newLEDState(val uint8) LEDState {
	return LEDState{
		NumLock:    (val & LEDNumLock) != 0,
		CapsLock:   (val & LEDCapsLock) != 0,
		ScrollLock: (val & LEDScrollLock) != 0,
	}
}

// LEDWatcher monitors `/dev/hidg0` for host LED state updates.
type LEDWatcher struct {
	mu         sync.Mutex
	devicePath string
	file       *os.File
	current    uint8
	listeners  map[chan LEDState]uint8 // channel -> mask
	stopCh     chan struct{}
	stopOnce   sync.Once
}

// GetState returns current LED lock status.
func (w *LEDWatcher) GetState() LEDState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return newLEDState(w.current)
}

// Subscribe returns a channel that receives LED state updates continuously,
// along with an unsubscribe function.
func (w *LEDWatcher) Subscribe() (<-chan LEDState, func()) {
	ch := make(chan LEDState, 16)

	w.mu.Lock()
	select {
	case <-w.stopCh:
		close(ch)
	default:
		w.listeners[ch] = 0
	}
	w.mu.Unlock()

	var unsubOnce sync.Once
	unsubscribe := func() {
		unsubOnce.Do(func() {
			w.mu.Lock()
			if _, ok := w.listeners[ch]; ok {
				delete(w.listeners, ch)
				close(ch)
			}
			w.mu.Unlock()
		})
	}

	return ch, unsubscribe
}

// NewLEDWatcher initializes an LEDWatcher reading from devicePath.
func NewLEDWatcher(ctx context.Context, devicePath string) *LEDWatcher {
	w := &LEDWatcher{
		devicePath: devicePath,
		listeners:  make(map[chan LEDState]uint8),
		stopCh:     make(chan struct{}),
	}

	go func() {
		select {
		case <-ctx.Done():
			_ = w.Close()
		case <-w.stopCh:
		}
	}()

	go w.readLoop()

	return w
}

// DevicePath returns the currently configured HID device path.
func (w *LEDWatcher) DevicePath() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.devicePath
}

// SetDevicePath updates the target HID device path and closes any currently open descriptor.
func (w *LEDWatcher) SetDevicePath(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.devicePath == path {
		return
	}
	w.devicePath = path
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
}

func (w *LEDWatcher) streamFile(f *os.File, buf []byte) bool {
	defer func() {
		w.mu.Lock()
		if w.file == f {
			w.file = nil
		}
		w.mu.Unlock()
		_ = f.Close()
	}()

	for {
		select {
		case <-w.stopCh:
			return false
		default:
		}

		n, err := f.Read(buf)
		for i := 0; i < n; i++ {
			w.UpdateState(buf[i])
		}
		if err != nil {
			select {
			case <-w.stopCh:
				return false
			case <-time.After(50 * time.Millisecond):
				return true
			}
		}
	}
}

func (w *LEDWatcher) readLoop() {
	buf := make([]byte, 8)
	for {
		select {
		case <-w.stopCh:
			return
		default:
		}

		w.mu.Lock()
		devPath := w.devicePath
		w.mu.Unlock()

		if devPath == "" {
			select {
			case <-w.stopCh:
				return
			case <-time.After(50 * time.Millisecond):
				continue
			}
		}

		f, err := os.Open(devPath)
		if err != nil {
			select {
			case <-w.stopCh:
				return
			case <-time.After(200 * time.Millisecond):
				continue
			}
		}

		w.mu.Lock()
		select {
		case <-w.stopCh:
			w.mu.Unlock()
			_ = f.Close()
			return
		default:
			if w.devicePath != devPath {
				w.mu.Unlock()
				_ = f.Close()
				continue
			}
			w.file = f
		}
		w.mu.Unlock()

		if !w.streamFile(f, buf) {
			return
		}
	}
}

// UpdateState manually updates LED state and notifies matching subscribers.
func (w *LEDWatcher) UpdateState(newState uint8) {
	w.mu.Lock()
	defer w.mu.Unlock()

	prev := w.current
	changed := prev ^ newState
	w.current = newState
	state := newLEDState(newState)

	for ch, mask := range w.listeners {
		if mask == 0 || (changed&mask) != 0 {
			select {
			case ch <- state:
			default:
			}
		}
	}
}

// ParseLEDMask converts string filters ("NUM", "CAPS", "SCROLL", "ANY") to bitmasks.
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
	ch := make(chan LEDState, 1)

	w.mu.Lock()
	select {
	case <-w.stopCh:
		w.mu.Unlock()
		return LEDState{}, errors.New("led watcher stopped")
	default:
		w.listeners[ch] = mask
	}
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
	case <-w.stopCh:
		return LEDState{}, errors.New("led watcher stopped")
	case <-timeoutChan:
		return LEDState{}, errors.New("timeout waiting for LED state change")
	case state, ok := <-ch:
		if !ok {
			return LEDState{}, errors.New("led watcher stopped")
		}
		return state, nil
	}
}

// Close stops the LEDWatcher and closes all subscriber channels and open device files.
func (w *LEDWatcher) Close() error {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.mu.Lock()
	defer w.mu.Unlock()
	for ch := range w.listeners {
		delete(w.listeners, ch)
		close(ch)
	}
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}
