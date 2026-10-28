// Package hid implements USB HID keyboard, mouse, and LED watcher devices.
package hid

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

type hidWriter struct {
	wmu        sync.Mutex
	devicePath string
	writer     io.Writer
	file       *os.File
}

func (w *hidWriter) writeReport(report []byte) error {
	w.wmu.Lock()
	defer w.wmu.Unlock()

	if w.writer == nil && w.devicePath != "" {
		f, err := os.OpenFile(w.devicePath, os.O_WRONLY|os.O_SYNC, 0666)
		if err != nil {
			return fmt.Errorf("opening device %q: %w", w.devicePath, err)
		}
		w.file = f
		w.writer = f
	}

	if w.writer == nil {
		return errors.New("hid device not connected")
	}

	n, err := w.writer.Write(report)
	if err != nil {
		if w.file != nil {
			_ = w.file.Close()
			w.file = nil
			w.writer = nil
		}
		return fmt.Errorf("hid write error: %w", err)
	}
	if n < len(report) {
		return io.ErrShortWrite
	}
	return nil
}

// SetWriter overrides the output writer (useful for testing or custom pipes).
func (w *hidWriter) SetWriter(out io.Writer) {
	w.wmu.Lock()
	defer w.wmu.Unlock()

	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	w.writer = out
}

// SetDevicePath updates the target HID device path.
func (w *hidWriter) SetDevicePath(path string) {
	w.wmu.Lock()
	defer w.wmu.Unlock()

	if path == w.devicePath {
		return
	}
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
		w.writer = nil
	}
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
	w.wmu.Lock()
	defer w.wmu.Unlock()

	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		w.writer = nil
		return err
	}
	return nil
}
