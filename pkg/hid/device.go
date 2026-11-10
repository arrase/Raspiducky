// Package hid implements USB HID keyboard, mouse, and LED watcher devices.
package hid

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

const (
	hidAttemptDeadline = 100 * time.Millisecond
	hidWriteTimeout    = 1500 * time.Millisecond
	hidRetryBackoff    = 25 * time.Millisecond
)

// ErrHostNotPolling is returned when the USB host does not consume HID reports within the write timeout.
var ErrHostNotPolling = errors.New("hid write timeout: USB host is not polling endpoint")

type hidWriter struct {
	wmu        sync.Mutex
	fmu        sync.Mutex
	devicePath string
	writer     io.Writer
	file       *os.File
}

func isTransientHIDError(err error) bool {
	return errors.Is(err, syscall.EAGAIN) ||
		errors.Is(err, syscall.EINTR) ||
		errors.Is(err, syscall.ESHUTDOWN) ||
		errors.Is(err, syscall.ENXIO) ||
		errors.Is(err, syscall.ENODEV) ||
		errors.Is(err, os.ErrNotExist)
}

func (w *hidWriter) setActiveFile(f *os.File) {
	w.fmu.Lock()
	w.file = f
	w.fmu.Unlock()
}

func (w *hidWriter) closeLockedFile() error {
	w.fmu.Lock()
	f := w.file
	w.file = nil
	w.fmu.Unlock()
	if f == nil {
		return nil
	}
	_ = f.SetWriteDeadline(time.Now())
	return f.Close()
}

// Interrupt wakes any in-flight blocking device write immediately.
func (w *hidWriter) Interrupt() {
	w.fmu.Lock()
	f := w.file
	w.fmu.Unlock()
	if f != nil {
		_ = f.SetWriteDeadline(time.Now())
	}
}

func (w *hidWriter) ensureOpen() error {
	if w.writer != nil || w.devicePath == "" {
		return nil
	}
	f, err := os.OpenFile(w.devicePath, os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	w.setActiveFile(f)
	w.writer = f
	return nil
}

func (w *hidWriter) writeOnce(report []byte) error {
	if err := w.ensureOpen(); err != nil {
		return fmt.Errorf("opening device %q: %w", w.devicePath, err)
	}
	if w.writer == nil {
		return errors.New("hid device not connected")
	}

	if w.file != nil {
		_ = w.file.SetWriteDeadline(time.Now().Add(hidAttemptDeadline))
	}

	n, err := w.writer.Write(report)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return err
		}
		if w.file != nil {
			_ = w.closeLockedFile()
			w.writer = nil
		}
		return err
	}
	if n < len(report) {
		return io.ErrShortWrite
	}
	return nil
}

func (w *hidWriter) writeReport(ctx context.Context, report []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	w.wmu.Lock()
	defer w.wmu.Unlock()

	deadline := time.Now().Add(hidWriteTimeout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := w.writeOnce(report)
		if err == nil {
			return nil
		}

		if errors.Is(err, os.ErrDeadlineExceeded) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if time.Now().After(deadline) {
				return ErrHostNotPolling
			}
			continue
		}

		if isTransientHIDError(err) && time.Now().Before(deadline) {
			if sleepErr := sleepContext(ctx, hidRetryBackoff); sleepErr != nil {
				return sleepErr
			}
			continue
		}

		return fmt.Errorf("hid write error: %w", err)
	}
}

// SetWriter overrides the output writer (useful for testing or custom pipes).
func (w *hidWriter) SetWriter(out io.Writer) {
	w.Interrupt()
	w.wmu.Lock()
	defer w.wmu.Unlock()

	_ = w.closeLockedFile()
	w.writer = out
}

// SetDevicePath updates the target HID device path.
func (w *hidWriter) SetDevicePath(path string) {
	w.Interrupt()
	w.wmu.Lock()
	defer w.wmu.Unlock()

	if path == w.devicePath {
		return
	}
	_ = w.closeLockedFile()
	w.writer = nil
	w.devicePath = path
}

// DevicePath returns the currently configured HID device path.
func (w *hidWriter) DevicePath() string {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	return w.devicePath
}

// Close closes the underlying device file if owned by this HID device.
func (w *hidWriter) Close() error {
	w.Interrupt()
	w.wmu.Lock()
	defer w.wmu.Unlock()

	if w.file != nil {
		err := w.closeLockedFile()
		w.writer = nil
		return err
	}
	return nil
}
