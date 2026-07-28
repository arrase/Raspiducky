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
	mu        sync.RWMutex
	activeJob *JobStatus
	cancelFn  context.CancelFunc
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

type wsLogWriter struct {
	re     *RunnerEngine
	source string
}

func (w *wsLogWriter) Write(p []byte) (n int, err error) {
	w.re.broadcastLog("INFO", w.source, strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
}

func (re *RunnerEngine) executeJob(ctx context.Context, job JobStatus, scriptContent string) {
	var execErr error

	engine := scripting.NewScriptEngine(re.keyboard, re.mouse, re.ledWatcher)

	if job.Type == "javascript" {
		execErr = engine.RunJS(ctx, scriptContent, &wsLogWriter{re: re, source: "JS"})
	} else {
		execErr = engine.RunDuckyScript(ctx, scriptContent, &wsLogWriter{re: re, source: "DUCKY"})
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
