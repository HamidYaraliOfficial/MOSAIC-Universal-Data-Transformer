package pipeline

import (
	"context"
	"runtime"
	"sync"
	"time"

	"mosaic/internal/schema"
)

// Status is a Job's lifecycle state, shown as a badge in the Job Engine UI.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Job tracks one pipeline execution: its status, live counters and the
// cancellation/pause plumbing the UI's Pause/Cancel buttons drive.
type Job struct {
	ID            string    `json:"id"`
	PipelineName  string    `json:"pipelineName"`
	Status        Status    `json:"status"`
	StartedAt     time.Time `json:"startedAt"`
	FinishedAt    time.Time `json:"finishedAt,omitempty"`
	RowsProcessed int       `json:"rowsProcessed"`
	RowsPerSec    float64   `json:"rowsPerSec"`
	MemoryMB      float64   `json:"memoryMb"`
	Error         string    `json:"error,omitempty"`

	mu       sync.Mutex
	cancel   context.CancelFunc
	pauseCh  chan struct{}
	resumeCh chan struct{}
	paused   bool
}

// Engine manages the set of active/historical Jobs for a project.
type Engine struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewEngine creates an empty Job Engine.
func NewEngine() *Engine { return &Engine{jobs: map[string]*Job{}} }

// Submit turns a pipeline Definition + Sources into a running Job. onResult
// streams per-node progress (see Executor.OnResult) up to the caller, which
// the bridge layer forwards to the frontend as Server-Sent Events.
func (e *Engine) Submit(id, name string, def *Definition, sources Sources, onResult ProgressFunc, onDone func(*Job, map[string][]schema.Row)) (*Job, error) {
	g, err := Build(def)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &Job{
		ID:           id,
		PipelineName: name,
		Status:       StatusQueued,
		cancel:       cancel,
		pauseCh:      make(chan struct{}),
		resumeCh:     make(chan struct{}),
	}
	e.mu.Lock()
	e.jobs[id] = job
	e.mu.Unlock()

	go func() {
		job.mu.Lock()
		job.Status = StatusRunning
		job.StartedAt = time.Now()
		job.mu.Unlock()

		ex := NewExecutor()
		wrapped := func(res NodeResult) {
			job.mu.Lock()
			job.RowsProcessed += res.Metrics.RowsOut
			elapsed := time.Since(job.StartedAt).Seconds()
			if elapsed > 0 {
				job.RowsPerSec = float64(job.RowsProcessed) / elapsed
			}
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			job.MemoryMB = float64(ms.Alloc) / (1024 * 1024)
			job.mu.Unlock()
			if onResult != nil {
				onResult(res)
			}
			job.waitIfPaused()
		}
		ex.OnResult = wrapped

		results, err := ex.Run(ctx, g, sources)

		job.mu.Lock()
		job.FinishedAt = time.Now()
		switch {
		case ctx.Err() == context.Canceled:
			job.Status = StatusCancelled
		case err != nil:
			job.Status = StatusFailed
			job.Error = err.Error()
		default:
			job.Status = StatusCompleted
		}
		job.mu.Unlock()

		if onDone != nil {
			onDone(job, results)
		}
	}()

	return job, nil
}

func (j *Job) waitIfPaused() {
	j.mu.Lock()
	paused := j.paused
	j.mu.Unlock()
	if !paused {
		return
	}
	<-j.resumeCh
}

// Pause suspends progress after the currently-running node completes (real
// pause, not a UI-only flag: the executor blocks between pipeline levels).
func (j *Job) Pause() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Status != StatusRunning {
		return
	}
	j.paused = true
	j.Status = StatusPaused
}

// Resume continues a paused Job.
func (j *Job) Resume() {
	j.mu.Lock()
	if j.Status != StatusPaused {
		j.mu.Unlock()
		return
	}
	j.paused = false
	j.Status = StatusRunning
	j.mu.Unlock()
	select {
	case j.resumeCh <- struct{}{}:
	default:
	}
}

// Cancel stops the Job at the next safe checkpoint (between pipeline
// levels), via context cancellation propagated into the worker pool.
func (j *Job) Cancel() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cancel != nil {
		j.cancel()
	}
	if j.paused {
		j.paused = false
		select {
		case j.resumeCh <- struct{}{}:
		default:
		}
	}
}

// Get returns a job by ID for status polling.
func (e *Engine) Get(id string) (*Job, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	j, ok := e.jobs[id]
	return j, ok
}

// List returns every known job, most recent first, for the Job Engine panel.
func (e *Engine) List() []*Job {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Job, 0, len(e.jobs))
	for _, j := range e.jobs {
		out = append(out, j)
	}
	return out
}
