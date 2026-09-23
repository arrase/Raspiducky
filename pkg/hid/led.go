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
	stopCh := make(chan struct{})

	go func() {
		defer close(out)
		for {
			select {
			case <-stopCh:
				return
			case <-w.stopCh:
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

	var unsubOnce sync.Once
	unsubscribe := func() {
		unsubOnce.Do(func() {
			close(stopCh)
			w.mu.Lock()
			delete(w.listeners, ch)
			w.mu.Unlock()
		})
	}

	return out, unsubscribe
}

// LEDWatcher monitors `/dev/hidg0` for host LED state updates.
type LEDWatcher struct {
	mu         sync.Mutex
	devicePath string
	reader     io.Reader
	ownsReader bool
	running    bool
	current    uint8
	listeners  map[chan uint8]uint8 // channel -> mask
	stopCh     chan struct{}
	stopOnce   sync.Once
}

// NewLEDWatcher initializes an LEDWatcher reading from devicePath.
func NewLEDWatcher(ctx context.Context, devicePath string) (*LEDWatcher, error) {
	w := &LEDWatcher{
		devicePath: devicePath,
		listeners:  make(map[chan uint8]uint8),
		stopCh:     make(chan struct{}),
		running:    true,
	}

	go func() {
		select {
		case <-ctx.Done():
			_ = w.Close()
		case <-w.stopCh:
		}
	}()

	go w.readLoop()

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
	w.mu.Unlock()
}

func (w *LEDWatcher) streamReader(r io.Reader, buf []byte, fileToClose io.Closer, isCustomReader, ownsReader bool) bool {
	defer func() {
		if fileToClose != nil {
			_ = fileToClose.Close()
		}
	}()

	for {
		select {
		case <-w.stopCh:
			return false
		default:
		}

		n, err := r.Read(buf)
		if err != nil {
			if isCustomReader && !ownsReader {
				return false
			}
			time.Sleep(200 * time.Millisecond)
			return true
		}

		if n > 0 {
			w.UpdateState(buf[0])
		}
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
		case <-w.stopCh:
			return
		default:
		}

		w.mu.Lock()
		customReader := w.reader
		ownsReader := w.ownsReader
		devPath := w.devicePath
		w.mu.Unlock()

		if customReader != nil {
			if !w.streamReader(customReader, buf, nil, true, ownsReader) {
				return
			}
			continue
		}

		if devPath == "" {
			select {
			case <-w.stopCh:
				return
			case <-time.After(200 * time.Millisecond):
				continue
			}
		}

		f, err := os.Open(devPath)
		if err != nil {
			select {
			case <-w.stopCh:
				return
			case <-time.After(500 * time.Millisecond):
				continue
			}
		}

		if !w.streamReader(f, buf, f, false, false) {
			return
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
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
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
