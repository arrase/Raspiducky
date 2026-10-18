package hid

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

type hidWriter struct {
	mu         sync.Mutex
	devicePath string
	writer     io.Writer
	file       *os.File
}

func (w *hidWriter) writeReport(report []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

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

func (w *hidWriter) setWriter(out io.Writer) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	w.writer = out
}

func (w *hidWriter) setDevicePath(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if path == w.devicePath {
		return
	}
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	w.writer = nil
	w.devicePath = path
}

func (w *hidWriter) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		w.writer = nil
		return err
	}
	return nil
}
