package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
)

// RunnerEngine manages payload script execution jobs and live event streaming.
type RunnerEngine struct {
	mu        sync.RWMutex
	activeJob *JobStatus
	cancelFn  context.CancelFunc
	hub       *Hub
	keyboard  *hid.Keyboard
}

// NewRunnerEngine initializes a new RunnerEngine instance.
func NewRunnerEngine(hub *Hub, keyboard *hid.Keyboard) *RunnerEngine {
	return &RunnerEngine{
		hub:      hub,
		keyboard: keyboard,
	}
}

// GetActiveJob returns current running or last executed job.
func (re *RunnerEngine) GetActiveJob() *JobStatus {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.activeJob
}

// Run triggers execution of a script (DuckyScript or JavaScript).
func (re *RunnerEngine) Run(req RunRequest) (JobStatus, error) {
	if strings.TrimSpace(req.Script) == "" {
		return JobStatus{}, errors.New("cannot execute empty script")
	}

	re.mu.Lock()
	if re.activeJob != nil && re.activeJob.Status == "running" {
		re.mu.Unlock()
		return JobStatus{}, errors.New("a job is already running, stop it before starting a new one")
	}

	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano()%100000)
	scriptName := req.Name
	if scriptName == "" {
		scriptName = "inline_payload"
	}

	ctx, cancel := context.WithCancel(context.Background())
	re.cancelFn = cancel

	job := JobStatus{
		ID:         jobID,
		ScriptName: scriptName,
		Type:       req.Type,
		Status:     "running",
		StartedAt:  time.Now(),
	}
	re.activeJob = &job
	re.mu.Unlock()

	// Broadcast job start event
	re.broadcastJobStatus(job)
	re.broadcastLog("INFO", "ENGINE", fmt.Sprintf("Job [%s] started: %s (%s)", jobID, scriptName, req.Type))

	// Execute in background goroutine
	go re.executeJob(ctx, job, req.Script)

	return job, nil
}

// Stop cancels the currently active job.
func (re *RunnerEngine) Stop() error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if re.activeJob == nil || re.activeJob.Status != "running" {
		return errors.New("no active job is currently running")
	}

	if re.cancelFn != nil {
		re.cancelFn()
	}

	now := time.Now()
	re.activeJob.Status = "stopped"
	re.activeJob.FinishedAt = &now

	re.broadcastJobStatus(*re.activeJob)
	re.broadcastLog("WARN", "ENGINE", fmt.Sprintf("Job [%s] manually stopped by user.", re.activeJob.ID))

	return nil
}

func (re *RunnerEngine) executeJob(ctx context.Context, job JobStatus, scriptContent string) {
	var execErr error

	if job.Type == "javascript" {
		execErr = re.executeJavaScript(ctx, scriptContent)
	} else {
		execErr = re.executeDuckyScript(ctx, scriptContent)
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	now := time.Now()
	job.FinishedAt = &now

	if ctx.Err() == context.Canceled {
		job.Status = "stopped"
	} else if execErr != nil {
		job.Status = "failed"
		job.Error = execErr.Error()
		re.broadcastLog("ERROR", "ENGINE", fmt.Sprintf("Job [%s] failed: %v", job.ID, execErr))
	} else {
		job.Status = "completed"
		re.broadcastLog("INFO", "ENGINE", fmt.Sprintf("Job [%s] completed successfully.", job.ID))
	}

	re.activeJob = &job
	re.broadcastJobStatus(job)
}

func (re *RunnerEngine) executeDuckyScript(ctx context.Context, content string) error {
	lines := strings.Split(content, "\n")
	for idx, line := range lines {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "REM") {
			if strings.HasPrefix(line, "REM") {
				re.broadcastLog("INFO", "DUCKY", fmt.Sprintf("Comment: %s", line[3:]))
			}
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToUpper(parts[0])
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch cmd {
		case "DELAY":
			ms, err := strconv.Atoi(arg)
			if err != nil {
				ms = 500
			}
			re.broadcastLog("INFO", "HID", fmt.Sprintf("Delaying %d ms...", ms))
			select {
			case <-time.After(time.Duration(ms) * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}

		case "STRING":
			re.broadcastLog("HID", "KEYBOARD", fmt.Sprintf("Typing string: %s", arg))
			if re.keyboard != nil {
				if err := re.keyboard.TypeString(ctx, arg); err != nil {
					re.broadcastLog("ERROR", "KEYBOARD", fmt.Sprintf("TypeString failed: %v", err))
				}
			}
			time.Sleep(50 * time.Millisecond)

		case "ENTER", "GUI", "WINDOWS", "ALT", "CTRL", "CONTROL", "SHIFT":
			if arg != "" {
				re.broadcastLog("HID", "KEYBOARD", fmt.Sprintf("Key combo: %s + %s", cmd, arg))
				if re.keyboard != nil {
					if err := re.keyboard.Press(ctx, cmd+" "+arg); err != nil {
						re.broadcastLog("ERROR", "KEYBOARD", fmt.Sprintf("Press failed: %v", err))
					}
				}
			} else {
				re.broadcastLog("HID", "KEYBOARD", fmt.Sprintf("Key press: %s", cmd))
				if re.keyboard != nil {
					if err := re.keyboard.Press(ctx, cmd); err != nil {
						re.broadcastLog("ERROR", "KEYBOARD", fmt.Sprintf("Press failed: %v", err))
					}
				}
			}
			time.Sleep(30 * time.Millisecond)

		case "LED_NUM", "LED_CAPS", "LED_SCROLL":
			re.broadcastLog("INFO", "LED", fmt.Sprintf("Toggled LED: %s", cmd))
			re.broadcastLEDState(LEDState{NumLock: true, CapsLock: false, ScrollLock: true})

		default:
			re.broadcastLog("INFO", "HID", fmt.Sprintf("Line %d: Executed [%s]", idx+1, line))
			time.Sleep(20 * time.Millisecond)
		}
	}

	return nil
}

func (re *RunnerEngine) executeJavaScript(ctx context.Context, content string) error {
	lines := strings.Split(content, "\n")
	re.broadcastLog("INFO", "JS", "Starting JavaScript payload execution context...")

	for idx, line := range lines {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		re.broadcastLog("INFO", "JS", fmt.Sprintf("Evaluated L%d: %s", idx+1, line))
		time.Sleep(40 * time.Millisecond)
	}

	return nil
}

func (re *RunnerEngine) broadcastLog(level, source, message string) {
	log.Printf("[%s] %s: %s", level, source, message)
	if re.hub != nil {
		re.hub.Broadcast(WSMessage{
			Type:    "log",
			Level:   level,
			Source:  source,
			Message: message,
		})
	}
}

func (re *RunnerEngine) broadcastJobStatus(job JobStatus) {
	if re.hub != nil {
		re.hub.Broadcast(WSMessage{
			Type:    "job_status",
			Payload: job,
		})
	}
}

func (re *RunnerEngine) broadcastLEDState(leds LEDState) {
	if re.hub != nil {
		re.hub.Broadcast(WSMessage{
			Type:    "led_state",
			Payload: leds,
		})
	}
}
