package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mosaic/internal/cache"
	"mosaic/internal/runtime"
	"mosaic/internal/schema"
	"mosaic/internal/transform"
)

// NodeResult is everything the Execution Inspector needs to render for one
// node after a run: its output rows (capped for transport), metrics and any
// collected error rows.
type NodeResult struct {
	NodeID  string             `json:"nodeId"`
	Metrics transform.Metrics  `json:"metrics"`
	Errors  []transform.ErrorRow `json:"errors,omitempty"`
	Cached  bool               `json:"cached"`
}

// ProgressFunc is invoked after each node completes, letting the Job Engine
// stream live progress (rows processed, throughput) back to the UI.
type ProgressFunc func(result NodeResult)

// Executor runs a resolved Graph against a set of source datasets (from
// Input nodes) using bounded goroutine parallelism per dependency level.
type Executor struct {
	Cache    *cache.Cache
	Pool     *runtime.WorkerPool
	OnResult ProgressFunc
}

// NewExecutor builds an executor with a fresh cache and a worker pool sized
// to the host CPU count.
func NewExecutor() *Executor {
	return &Executor{Cache: cache.New(), Pool: runtime.NewWorkerPool(0)}
}

// Sources maps Input-node IDs to their already-parsed dataset.
type Sources map[string][]schema.Row

// Run executes every node in the graph, honoring dependencies and running
// independent nodes within a level concurrently. It returns the final rows
// produced by each terminal (no-outgoing-edge) node, keyed by node ID.
func (ex *Executor) Run(ctx context.Context, g *Graph, sources Sources) (map[string][]schema.Row, error) {
	levels, err := g.Levels()
	if err != nil {
		return nil, err
	}

	results := map[string][]schema.Row{}
	var mu sync.Mutex

	for _, level := range levels {
		var levelErr error
		for _, node := range level {
			node := node
			ex.Pool.Submit(ctx, func() error {
				out, res, err := ex.runNode(g, node, sources, results, &mu)
				mu.Lock()
				if err == nil {
					results[node.ID] = out
				}
				mu.Unlock()
				if ex.OnResult != nil {
					ex.OnResult(res)
				}
				return err
			})
		}
		if err := ex.Pool.Wait(); err != nil {
			levelErr = err
		}
		// Re-create the pool for the next level (a WorkerPool is drained
		// after Wait returns nil error state, but we rebuild defensively
		// in case a future refactor makes it single-use).
		ex.Pool = runtime.NewWorkerPool(0)
		if levelErr != nil {
			return results, levelErr
		}
	}
	return results, nil
}

func (ex *Executor) runNode(g *Graph, node *NodeDef, sources Sources, results map[string][]schema.Row, mu *sync.Mutex) ([]schema.Row, NodeResult, error) {
	start := time.Now()
	res := NodeResult{NodeID: node.ID}

	if node.Disabled {
		rows := ex.gatherPrimary(g, node, sources, results, mu)
		res.Metrics = transform.Metrics{RowsIn: len(rows), RowsOut: len(rows)}
		return rows, res, nil
	}

	if node.Type == "input" {
		rows := sources[node.ID]
		res.Metrics = transform.Metrics{RowsIn: 0, RowsOut: len(rows), DurationMs: msSince(start)}
		return rows, res, nil
	}
	if node.Type == "output" {
		rows := ex.gatherPrimary(g, node, sources, results, mu)
		res.Metrics = transform.Metrics{RowsIn: len(rows), RowsOut: len(rows), DurationMs: msSince(start)}
		return rows, res, nil
	}

	n, err := transform.New(node.Type)
	if err != nil {
		res.Errors = []transform.ErrorRow{{Message: err.Error()}}
		return nil, res, err
	}

	primary := ex.gatherPrimary(g, node, sources, results, mu)
	inputs := ex.gatherNamedInputs(g, node, sources, results, mu)

	fp := cache.Fingerprint(node.Type, node.Config, append([][]schema.Row{primary}, mapValuesOf(inputs)...)...)
	if cached, ok := ex.Cache.Get(node.ID, fp); ok {
		res.Metrics = transform.Metrics{RowsIn: len(primary), RowsOut: len(cached), DurationMs: msSince(start)}
		res.Cached = true
		return cached, res, nil
	}

	tctx := &transform.Context{Config: node.Config, OnError: node.OnError, Inputs: inputs}
	if tctx.OnError == "" {
		tctx.OnError = transform.OnErrorSkip
	}

	out, err := transform.RunWithRecovery(n, tctx, primary)
	if err != nil && tctx.OnError == transform.OnErrorStop {
		res.Errors = tctx.Errors
		res.Metrics = transform.Metrics{RowsIn: len(primary), ErrorCount: len(tctx.Errors), DurationMs: msSince(start)}
		return nil, res, fmt.Errorf("node %s (%s): %w", node.ID, node.Type, err)
	}

	ex.Cache.Set(node.ID, fp, out)
	res.Errors = tctx.Errors
	res.Metrics = transform.Metrics{
		RowsIn:     len(primary),
		RowsOut:    len(out),
		ErrorCount: len(tctx.Errors),
		DurationMs: msSince(start),
	}
	return out, res, nil
}

func (ex *Executor) gatherPrimary(g *Graph, node *NodeDef, sources Sources, results map[string][]schema.Row, mu *sync.Mutex) []schema.Row {
	ports := g.IncomingByPort(node.ID)
	edges := ports["in"]
	if len(edges) == 0 {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if len(edges) == 1 {
		return lookupRows(edges[0].From, sources, results)
	}
	var combined []schema.Row
	for _, e := range edges {
		combined = append(combined, lookupRows(e.From, sources, results)...)
	}
	return combined
}

func (ex *Executor) gatherNamedInputs(g *Graph, node *NodeDef, sources Sources, results map[string][]schema.Row, mu *sync.Mutex) map[string][]schema.Row {
	ports := g.IncomingByPort(node.ID)
	out := map[string][]schema.Row{}
	mu.Lock()
	defer mu.Unlock()
	for port, edges := range ports {
		if port == "in" {
			continue
		}
		for _, e := range edges {
			out[port] = append(out[port], lookupRows(e.From, sources, results)...)
		}
	}
	return out
}

func lookupRows(nodeID string, sources Sources, results map[string][]schema.Row) []schema.Row {
	if rows, ok := results[nodeID]; ok {
		return rows
	}
	return sources[nodeID]
}

func mapValuesOf(m map[string][]schema.Row) [][]schema.Row {
	out := make([][]schema.Row, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func msSince(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000.0 }
