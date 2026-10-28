package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/arrase/Raspiducky/pkg/hid"
	"github.com/arrase/Raspiducky/pkg/scripting"
)

// RunnerEngine manages payload script execution jobs and live event streaming.
type RunnerEngine struct {
	mu         sync.RWMutex
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

	re.mu.Lock()
	if re.activeJob != nil && re.activeJob.Status == "running" {
		re.mu.Unlock()
		return JobStatus{}, errors.New("a job is already running, stop it before starting a new one")
	}

	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	scriptName := req.Name
	if scriptName == "" {
		scriptName = "inline_payload"
	}
	scriptType := scripting.DetectScriptType(req.Type, scriptName)

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

// Stop cancels the currently active job and waits for its execution goroutine to clean up.
func (re *RunnerEngine) Stop() error {
	re.mu.Lock()
	if re.activeJob == nil || re.activeJob.Status != "running" {
		re.mu.Unlock()
		return errors.New("no active job is currently running")
	}

	re.cancelFn()
	doneCh := re.jobDone

	now := time.Now()
	re.activeJob.Status = "stopped"
	re.activeJob.FinishedAt = &now
	stoppedJob := *re.activeJob
	re.mu.Unlock()

	if doneCh != nil {
		<-doneCh
	}

	re.broadcastJobStatus(stoppedJob)
	re.broadcastLog("WARN", "ENGINE", fmt.Sprintf("Job [%s] manually stopped by user.", stoppedJob.ID))

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

	origLayout := re.keyboard.GetLayout()
	origDelay, origJitter := re.keyboard.GetTypingSpeed()
	if req.TypingDelayMs > 0 {
		re.keyboard.SetTypingSpeed(req.TypingDelayMs, 0)
	}
	if req.Layout != "" {
		_ = re.keyboard.SetLayout(req.Layout)
	}

	source := "DUCKY"
	if job.Type == scripting.TypeJavaScript {
		source = "JS"
	}

	engine := scripting.NewScriptEngine(re.keyboard, re.mouse, re.ledWatcher)
	execErr := engine.Run(ctx, job.Type, job.ScriptName, req.Script, &wsLogWriter{re: re, source: source})

	_ = re.keyboard.SetLayout(origLayout)
	re.keyboard.SetTypingSpeed(origDelay, origJitter)
	_ = re.keyboard.ReleaseKeys()

	re.mu.Lock()
	defer re.mu.Unlock()

	if re.activeJob == nil || re.activeJob.ID != job.ID || re.activeJob.Status == "stopped" {
		return
	}

	now := time.Now()
	job.FinishedAt = &now

	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		job.Status = "stopped"
	case execErr != nil:
		job.Status = "failed"
		job.Error = execErr.Error()
		re.broadcastLog("ERROR", "ENGINE", fmt.Sprintf("Job [%s] failed: %v", job.ID, execErr))
	default:
		job.Status = "completed"
		re.broadcastLog("INFO", "ENGINE", fmt.Sprintf("Job [%s] completed successfully.", job.ID))
	}

	re.activeJob = &job
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
