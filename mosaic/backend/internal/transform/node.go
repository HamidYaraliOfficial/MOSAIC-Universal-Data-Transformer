// Package transform implements the library of Pipeline Canvas nodes:
// Select Columns, Filter Rows, Join, Group By, and so on. Every node
// implements the same small Node interface, which is also the public
// contract of the Custom Node SDK — a third-party plugin only needs to
// implement this interface and call Register() to appear on the canvas.
package transform

import (
	"fmt"
	"sync"

	"mosaic/internal/schema"
)

// Metrics captures per-node runtime telemetry surfaced by the Execution
// Inspector (rows in/out, duration, errors...).
type Metrics struct {
	RowsIn      int     `json:"rowsIn"`
	RowsOut     int     `json:"rowsOut"`
	ErrorCount  int     `json:"errorCount"`
	WarnCount   int     `json:"warnCount"`
	DurationMs  float64 `json:"durationMs"`
}

// ErrorRow captures a row that failed transformation, routed to the node's
// Error Output Stream when OnError == "collect".
type ErrorRow struct {
	Row     schema.Row `json:"row"`
	Message string     `json:"message"`
}

// OnErrorMode controls node failure behavior, configurable per node from
// the Inspector panel.
type OnErrorMode string

const (
	OnErrorStop    OnErrorMode = "stop"
	OnErrorSkip    OnErrorMode = "skip"
	OnErrorCollect OnErrorMode = "collect"
)

// Context is passed to every node execution: its config, and the running
// error sink for "collect" mode.
type Context struct {
	Config   map[string]any
	OnError  OnErrorMode
	Errors   []ErrorRow
	Inputs   map[string][]schema.Row // for multi-input nodes (Join, Union)
}

func (c *Context) recordError(row schema.Row, err error) {
	c.Errors = append(c.Errors, ErrorRow{Row: row, Message: err.Error()})
}

// Node is the contract every transform step implements — the same
// interface third-party Custom Node SDK plugins target.
type Node interface {
	// Type is the stable node-type identifier used in pipeline JSON, e.g.
	// "filterRows".
	Type() string
	// Run executes the node against input rows and returns transformed
	// output rows. Errors during row-level processing are handled
	// according to ctx.OnError rather than aborting Run outright.
	Run(ctx *Context, rows []schema.Row) ([]schema.Row, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]func() Node{}
)

// Register adds a node factory to the global registry, called from each
// node file's init(). This is the extension point for the Custom Node SDK:
// third-party plugins register new node types the same way.
func Register(nodeType string, factory func() Node) {
	mu.Lock()
	defer mu.Unlock()
	registry[nodeType] = factory
}

// New instantiates a node by its registered type name.
func New(nodeType string) (Node, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[nodeType]
	if !ok {
		return nil, fmt.Errorf("transform: unknown node type %q", nodeType)
	}
	return f(), nil
}

// List returns every registered node type, used to populate the canvas'
// node palette / Command Palette search.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for t := range registry {
		out = append(out, t)
	}
	return out
}

// RunWithRecovery wraps Run with the OnError policy (stop / skip / collect)
// used uniformly by the pipeline executor for every node.
func RunWithRecovery(n Node, ctx *Context, rows []schema.Row) ([]schema.Row, error) {
	defer func() {
		if r := recover(); r != nil {
			ctx.recordError(nil, fmt.Errorf("panic in node %s: %v", n.Type(), r))
		}
	}()
	return n.Run(ctx, rows)
}
