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

// USB HID keyboard LED indicator bitmasks.
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
	mu          sync.Mutex
	devicePath  string
	file        *os.File
	current     uint8
	listeners   map[chan LEDState]uint8
	pathChanged chan struct{}
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
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
		devicePath:  devicePath,
		listeners:   make(map[chan LEDState]uint8),
		pathChanged: make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.readLoop(ctx)
	}()

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
	if w.devicePath == path {
		w.mu.Unlock()
		return
	}
	w.devicePath = path
	w.current = 0
	f := w.file
	w.file = nil
	w.mu.Unlock()

	if f != nil {
		_ = f.SetReadDeadline(time.Now())
		_ = f.Close()
	}
	select {
	case w.pathChanged <- struct{}{}:
	default:
	}
}

func (w *LEDWatcher) waitStopOrPathChange(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return false
		case <-w.stopCh:
			return false
		case <-w.pathChanged:
			return true
		}
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-w.stopCh:
		return false
	case <-w.pathChanged:
		return true
	case <-timer.C:
		return true
	}
}

func (w *LEDWatcher) streamFile(ctx context.Context, f *os.File, buf []byte) bool {
	defer func() {
		w.mu.Lock()
		if w.file == f {
			w.file = nil
		}
		w.mu.Unlock()
		_ = f.Close()
	}()

	for {
		n, err := f.Read(buf)
		for i := 0; i < n; i++ {
			w.UpdateState(buf[i])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return w.waitStopOrPathChange(ctx, 0)
			}
			return w.waitStopOrPathChange(ctx, 50*time.Millisecond)
		}
	}
}

func (w *LEDWatcher) openCurrentDevice() (*os.File, string, bool) {
	w.mu.Lock()
	devPath := w.devicePath
	select {
	case <-w.pathChanged:
	default:
	}
	w.mu.Unlock()
	if devPath == "" {
		return nil, "", false
	}

	f, err := os.Open(devPath)
	if err != nil {
		return nil, devPath, false
	}

	w.mu.Lock()
	select {
	case <-w.stopCh:
		w.mu.Unlock()
		_ = f.Close()
		return nil, "", false
	default:
	}
	if w.devicePath != devPath {
		w.mu.Unlock()
		_ = f.Close()
		return nil, devPath, true
	}
	w.file = f
	w.mu.Unlock()
	return f, devPath, true
}

func (w *LEDWatcher) readLoop(ctx context.Context) {
	defer w.shutdownListeners()

	buf := make([]byte, 8)
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
		}

		f, devPath, retryNow := w.openCurrentDevice()
		if f == nil {
			if retryNow {
				continue
			}
			delay := 200 * time.Millisecond
			if devPath == "" {
				delay = 0
			}
			if !w.waitStopOrPathChange(ctx, delay) {
				return
			}
			continue
		}

		if !w.streamFile(ctx, f, buf) {
			return
		}
	}
}

// UpdateState manually updates LED state and notifies matching subscribers.
func (w *LEDWatcher) UpdateState(newState uint8) {
	w.mu.Lock()
	defer w.mu.Unlock()

	changed := w.current ^ newState
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

// ParseLEDMask converts string filters ("NUM", "CAPS", "SCROLL", "ANY", "NUM|SCROLL") to bitmasks.
func ParseLEDMask(filter string) (uint8, error) {
	upper := strings.ToUpper(strings.TrimSpace(filter))
	if upper == "" || upper == "ANY" {
		return LEDAny, nil
	}

	tokens := strings.FieldsFunc(upper, func(r rune) bool {
		return r == '|' || r == ',' || r == '+' || r == ' ' || r == '\t'
	})
	var mask uint8
	for _, tok := range tokens {
		switch tok {
		case "NUM", "NUMLOCK":
			mask |= LEDNumLock
		case "CAPS", "CAPSLOCK":
			mask |= LEDCapsLock
		case "SCROLL", "SCROLLLOCK":
			mask |= LEDScrollLock
		case "ANY":
			mask |= LEDAny
		default:
			return 0, fmt.Errorf("unrecognized LED mask token %q in %q", tok, filter)
		}
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

	var timeoutCh <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case <-ctx.Done():
		return LEDState{}, ctx.Err()
	case <-w.stopCh:
		return LEDState{}, errors.New("led watcher stopped")
	case <-timeoutCh:
		return LEDState{}, errors.New("timeout waiting for LED state change")
	case state, ok := <-ch:
		if !ok {
			return LEDState{}, errors.New("led watcher stopped")
		}
		return state, nil
	}
}

func (w *LEDWatcher) shutdownListeners() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for ch := range w.listeners {
		delete(w.listeners, ch)
		close(ch)
	}
}

// Close stops the LEDWatcher and closes all subscriber channels and open device files.
func (w *LEDWatcher) Close() error {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})

	w.mu.Lock()
	f := w.file
	w.file = nil
	w.mu.Unlock()

	var closeErr error
	if f != nil {
		_ = f.SetReadDeadline(time.Now())
		closeErr = f.Close()
	}

	w.shutdownListeners()
	w.wg.Wait()
	return closeErr
}
