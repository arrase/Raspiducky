package scripting

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"
)

type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
	StatusCancelled JobStatus = "cancelled"
)

// Job represents an asynchronous script execution task.
type Job struct {
	mu         sync.Mutex
	ID         string    `json:"id"`
	ScriptType string    `json:"scriptType"` // "js" or "ducky"
	Source     string    `json:"source"`
	Status     JobStatus `json:"status"`
	StartTime  time.Time `json:"startTime,omitempty"`
	EndTime    time.Time `json:"endTime,omitempty"`
	Error      string    `json:"error,omitempty"`
	Logs       string    `json:"logs"`

	logBuf     bytes.Buffer
	cancelFunc context.CancelFunc
}

// snapshotLocked returns a copy of the job's public state. Caller must hold j.mu.
func (j *Job) snapshotLocked() *Job {
	return &Job{
		ID:         j.ID,
		ScriptType: j.ScriptType,
		Source:     j.Source,
		Status:     j.Status,
		StartTime:  j.StartTime,
		EndTime:    j.EndTime,
		Error:      j.Error,
		Logs:       j.logBuf.String(),
	}
}

func (j *Job) appendLog(msg []byte) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.logBuf.Write(msg)
}

type jobWriter struct {
	job *Job
}

func (w *jobWriter) Write(p []byte) (n int, err error) {
	w.job.appendLog(p)
	return len(p), nil
}

// Runner manages asynchronous job creation, cancellation, and status tracking.
type Runner struct {
	mu     sync.RWMutex
	engine *ScriptEngine
	jobs   map[string]*Job
	counter int64
}

// NewRunner initializes a Runner connected to a ScriptEngine.
func NewRunner(engine *ScriptEngine) *Runner {
	return &Runner{
		engine: engine,
		jobs:   make(map[string]*Job),
	}
}

// SubmitJob starts a new script job asynchronously in the background.
func (r *Runner) SubmitJob(scriptType string, source string) (*Job, error) {
	r.mu.Lock()
	r.counter++
	jobID := fmt.Sprintf("job-%d-%d", time.Now().UnixNano(), r.counter)

	ctx, cancel := context.WithCancel(context.Background())
	job := &Job{
		ID:         jobID,
		ScriptType: scriptType,
		Source:     source,
		Status:     StatusPending,
		cancelFunc: cancel,
	}

	r.jobs[jobID] = job
	r.mu.Unlock()

	go r.executeJob(ctx, job)

	return job, nil
}

func (r *Runner) executeJob(ctx context.Context, job *Job) {
	defer job.cancelFunc()

	job.mu.Lock()
	job.Status = StatusRunning
	job.StartTime = time.Now()
	job.mu.Unlock()

	writer := &jobWriter{job: job}
	var err error

	if job.ScriptType == "ducky" || job.ScriptType == "duckyscript" {
		err = r.engine.RunDuckyScript(ctx, job.Source, writer)
	} else {
		err = r.engine.RunJS(ctx, job.Source, writer)
	}

	job.mu.Lock()
	defer job.mu.Unlock()
	job.EndTime = time.Now()

	if ctx.Err() != nil {
		job.Status = StatusCancelled
		job.Error = ctx.Err().Error()
	} else if err != nil {
		job.Status = StatusFailed
		job.Error = err.Error()
	} else {
		job.Status = StatusCompleted
	}
}

// CancelJob cancels a running job by ID.
func (r *Runner) CancelJob(jobID string) error {
	r.mu.RLock()
	job, exists := r.jobs[jobID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.mu.Lock()
	if job.cancelFunc != nil {
		job.cancelFunc()
	}
	job.mu.Unlock()
	return nil
}

// GetJob returns a snapshot of a job by ID.
func (r *Runner) GetJob(jobID string) (*Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return nil, false
	}

	job.mu.Lock()
	defer job.mu.Unlock()
	return job.snapshotLocked(), true
}

// ListJobs returns all current and historical jobs.
func (r *Runner) ListJobs() []*Job {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		j.mu.Lock()
		list = append(list, j.snapshotLocked())
		j.mu.Unlock()
	}
	return list
}
