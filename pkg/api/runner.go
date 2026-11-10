package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
	"github.com/arrase/Raspiducky/pkg/scripting"
)

// RunnerEngine manages payload script execution jobs and live event streaming.
type RunnerEngine struct {
	mu         sync.RWMutex
	seq        atomic.Uint64
	activeJob  *JobStatus
	cancelFn   context.CancelFunc
	jobDone    chan struct{}
	hub        *Hub
	keyboard   *hid.Keyboard
	mouse      *hid.Mouse
	ledWatcher *hid.LEDWatcher
}

// NewRunnerEngine initializes a new RunnerEngine instance.
func NewRunnerEngine(hub *Hub, keyboard *hid.Keyboard, mouse *hid.Mouse, ledWatcher *hid.LEDWatcher) *RunnerEngine {
	return &RunnerEngine{
		hub:        hub,
		keyboard:   keyboard,
		mouse:      mouse,
		ledWatcher: ledWatcher,
	}
}

// GetActiveJob returns current running or last executed job.
func (re *RunnerEngine) GetActiveJob() *JobStatus {
	re.mu.RLock()
	defer re.mu.RUnlock()
	if re.activeJob == nil {
		return nil
	}
	cp := *re.activeJob
	return &cp
}

// Run triggers execution of a script (DuckyScript or JavaScript).
func (re *RunnerEngine) Run(req RunRequest) (JobStatus, error) {
	if strings.TrimSpace(req.Script) == "" {
		return JobStatus{}, errors.New("cannot execute empty script")
	}
	if req.Layout != "" {
		if _, err := hid.GetLayout(req.Layout); err != nil {
			return JobStatus{}, err
		}
	}

	scriptName := req.Name
	if scriptName == "" {
		scriptName = "inline_payload"
	}
	scriptType := scripting.DetectScriptType(req.Type, scriptName)

	// Pre-validate DuckyScript syntax before starting the background job so typos fail immediately.
	if scriptType == scripting.TypeDuckyScript {
		initialLayout := req.Layout
		if initialLayout == "" && re.keyboard != nil {
			initialLayout = re.keyboard.GetLayout()
		}
		if _, err := scripting.CompileDuckyScript(req.Script, initialLayout); err != nil {
			return JobStatus{}, err
		}
	}

	re.mu.Lock()
	if re.activeJob != nil && re.activeJob.Status == "running" {
		re.mu.Unlock()
		return JobStatus{}, errors.New("a job is already running, stop it before starting a new one")
	}

	jobID := fmt.Sprintf("job-%d-%d", time.Now().UnixNano(), re.seq.Add(1))
	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})
	re.cancelFn = cancel
	re.jobDone = doneCh

	job := JobStatus{
		ID:         jobID,
		ScriptName: scriptName,
		Type:       scriptType,
		Status:     "running",
		StartedAt:  time.Now(),
	}
	re.activeJob = &job
	re.mu.Unlock()

	re.broadcastJobStatus(job)
	re.broadcastLog("INFO", "ENGINE", fmt.Sprintf("Job [%s] started: %s (%s)", jobID, scriptName, scriptType))

	go re.executeJob(ctx, doneCh, job, req)

	return job, nil
}

// Stop cancels the currently active job, interrupts any blocked HID write, and waits for full cleanup.
func (re *RunnerEngine) Stop() error {
	re.mu.Lock()
	if re.activeJob == nil || re.activeJob.Status != "running" {
		re.mu.Unlock()
		return errors.New("no active job is currently running")
	}
	cancel := re.cancelFn
	doneCh := re.jobDone
	re.mu.Unlock()

	cancel()
	if re.keyboard != nil {
		re.keyboard.Interrupt()
	}
	if re.mouse != nil {
		re.mouse.Interrupt()
	}

	if doneCh != nil {
		<-doneCh
	}
	return nil
}

type wsLogWriter struct {
	re     *RunnerEngine
	source string
}

func (w *wsLogWriter) Write(p []byte) (n int, err error) {
	w.re.broadcastLog("INFO", w.source, strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
}

func (re *RunnerEngine) executeJob(ctx context.Context, doneCh chan struct{}, job JobStatus, req RunRequest) {
	defer close(doneCh)

	var origLayout string
	var origDelay, origJitter int
	if re.keyboard != nil {
		origLayout = re.keyboard.GetLayout()
		origDelay, origJitter = re.keyboard.GetTypingSpeed()
		if req.TypingDelayMs != nil {
			re.keyboard.SetTypingSpeed(*req.TypingDelayMs, 0)
		}
		if req.Layout != "" {
			_ = re.keyboard.SetLayout(req.Layout)
		}
	}

	source := "DUCKY"
	if job.Type == scripting.TypeJavaScript {
		source = "JS"
	}

	engine := scripting.NewScriptEngine(re.keyboard, re.mouse, re.ledWatcher)
	execErr := engine.Run(ctx, job.Type, job.ScriptName, req.Script, &wsLogWriter{re: re, source: source})

	if re.keyboard != nil {
		_ = re.keyboard.SetLayout(origLayout)
		re.keyboard.SetTypingSpeed(origDelay, origJitter)
		_ = re.keyboard.ReleaseKeys()
	}
	if re.mouse != nil {
		_ = re.mouse.ReleaseButtons()
	}

	now := time.Now()
	job.FinishedAt = &now

	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		job.Status = "stopped"
	case execErr != nil:
		job.Status = "failed"
		job.Error = execErr.Error()
	default:
		job.Status = "completed"
	}

	re.mu.Lock()
	if re.activeJob != nil && re.activeJob.ID == job.ID {
		re.activeJob = &job
	}
	re.mu.Unlock()

	switch job.Status {
	case "stopped":
		re.broadcastLog("WARN", "ENGINE", fmt.Sprintf("Job [%s] manually stopped by user.", job.ID))
	case "failed":
		re.broadcastLog("ERROR", "ENGINE", fmt.Sprintf("Job [%s] failed: %v", job.ID, execErr))
	case "completed":
		re.broadcastLog("INFO", "ENGINE", fmt.Sprintf("Job [%s] completed successfully.", job.ID))
	}
	re.broadcastJobStatus(job)
}

func (re *RunnerEngine) broadcastLog(level, source, message string) {
	sanitizedMsg := strings.ReplaceAll(strings.ReplaceAll(message, "\n", " "), "\r", " ")
	log.Printf("[%s] %s: %s", level, source, sanitizedMsg)
	re.hub.Broadcast(WSMessage{
		Type:    "log",
		Level:   level,
		Source:  source,
		Message: message,
	})
}

func (re *RunnerEngine) broadcastJobStatus(job JobStatus) {
	re.hub.Broadcast(WSMessage{
		Type:    "job_status",
		Payload: job,
	})
}
