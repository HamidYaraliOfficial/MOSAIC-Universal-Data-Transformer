// Package runtime provides the low-level execution primitives shared by the
// Pipeline Runtime: a bounded goroutine worker pool and a chunked streaming
// reader used to keep multi-gigabyte datasets out of RAM.
package runtime

import (
	"context"
	"runtime"
	"sync"
)

// WorkerPool runs a bounded number of jobs concurrently. Pipeline levels
// (see pipeline.Graph.Levels) submit their independent nodes here so CPU
// parallelism scales with GOMAXPROCS rather than the number of nodes.
type WorkerPool struct {
	sem chan struct{}
	wg  sync.WaitGroup
	mu  sync.Mutex
	err error
}

// NewWorkerPool creates a pool sized to the host's CPU count when size<=0.
func NewWorkerPool(size int) *WorkerPool {
	if size <= 0 {
		size = runtime.NumCPU()
	}
	return &WorkerPool{sem: make(chan struct{}, size)}
}

// Submit runs fn in a goroutine once a slot is free. The first error from
// any job is retained and returned by Wait; subsequent jobs still run to
// completion so a Job's error stream (per-node) stays consistent, but the
// pool signals failure via ctx cancellation for cooperative early-exit.
func (p *WorkerPool) Submit(ctx context.Context, fn func() error) {
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() { <-p.sem }()
		if err := fn(); err != nil {
			p.mu.Lock()
			if p.err == nil {
				p.err = err
			}
			p.mu.Unlock()
		}
	}()
}

// Wait blocks until every submitted job has completed and returns the first
// recorded error, if any.
func (p *WorkerPool) Wait() error {
	p.wg.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}
