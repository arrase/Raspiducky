package scripting

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
)

func TestEngineJSExecution(t *testing.T) {
	kbdBuf := &bytes.Buffer{}
	mouseBuf := &bytes.Buffer{}
	logBuf := &bytes.Buffer{}

	kbd, _ := hid.NewKeyboard("", "US")
	kbd.SetWriter(kbdBuf)

	mouse, _ := hid.NewMouse("")
	mouse.SetWriter(mouseBuf)

	engine := NewScriptEngine(kbd, mouse, nil)

	jsCode := `
		log("Starting script");
		type("Hello");
		press("ENTER");
		mouseMove(10, 20);
		mouseClick("left");
	`

	err := engine.RunJS(context.Background(), jsCode, logBuf)
	if err != nil {
		t.Fatalf("RunJS failed: %v", err)
	}

	if !strings.Contains(logBuf.String(), "Starting script") {
		t.Errorf("Expected log output 'Starting script', got %q", logBuf.String())
	}
	if kbdBuf.Len() == 0 {
		t.Errorf("Expected keyboard report written, got 0 bytes")
	}
	if mouseBuf.Len() == 0 {
		t.Errorf("Expected mouse report written, got 0 bytes")
	}
}

func TestJobRunner(t *testing.T) {
	kbdBuf := &bytes.Buffer{}
	kbd, _ := hid.NewKeyboard("", "US")
	kbd.SetWriter(kbdBuf)

	engine := NewScriptEngine(kbd, nil, nil)
	runner := NewRunner(engine)

	duckyCode := `
REM Async Test
DELAY 10
STRING Async Hello
`

	job, err := runner.SubmitJob("ducky", duckyCode)
	if err != nil {
		t.Fatalf("SubmitJob failed: %v", err)
	}

	// Wait for job completion
	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)
		s, ok := runner.GetJob(job.ID)
		if ok && (s.Status == StatusCompleted || s.Status == StatusFailed) {
			if s.Status != StatusCompleted {
				t.Fatalf("Job failed with error: %s", s.Error)
			}
			break
		}
	}

	snapshot, ok := runner.GetJob(job.ID)
	if !ok || snapshot.Status != StatusCompleted {
		t.Fatalf("Expected job status %s, got %s", StatusCompleted, snapshot.Status)
	}
}

func TestJobCancellation(t *testing.T) {
	engine := NewScriptEngine(nil, nil, nil)
	runner := NewRunner(engine)

	jsCode := `
		delay(2000);
	`

	job, err := runner.SubmitJob("js", jsCode)
	if err != nil {
		t.Fatalf("SubmitJob failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	err = runner.CancelJob(job.ID)
	if err != nil {
		t.Fatalf("CancelJob failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	snapshot, ok := runner.GetJob(job.ID)
	if !ok || snapshot.Status != StatusCancelled {
		t.Fatalf("Expected job status %s, got %s", StatusCancelled, snapshot.Status)
	}
}
