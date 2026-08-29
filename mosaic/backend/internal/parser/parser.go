// Package parser implements MOSAIC's plugin-based file ingestion layer.
// Every format (CSV, JSON, XML, YAML, ...) implements the Parser interface
// and self-registers via Register(), so the Import pipeline never needs a
// hardcoded switch statement — new formats are added by dropping in a new
// file that calls Register() in its init().
package parser

import (
	"fmt"
	"io"
	"sync"

	"mosaic/internal/schema"
)

// Options configures how a Parser reads a source. Not every option applies
// to every format; parsers ignore what they don't understand.
type Options struct {
	Delimiter    rune              `json:"delimiter,omitempty"`
	HasHeader    bool              `json:"hasHeader"`
	Encoding     string            `json:"encoding,omitempty"`
	SampleLimit  int               `json:"sampleLimit,omitempty"`
	StreamChunk  int               `json:"streamChunk,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}

// Result is a fully materialized parse (used for profiling / preview).
type Result struct {
	Columns []string     `json:"columns"`
	Rows    []schema.Row `json:"rows"`
	Format  string       `json:"format"`
}

// RowHandler is called once per row during streaming parses so large files
// never need to be fully materialized in memory.
type RowHandler func(row schema.Row) error

// Parser is the contract every format plugin implements.
type Parser interface {
	// Name is the stable identifier used in pipeline JSON, e.g. "csv".
	Name() string
	// Sniff returns a confidence score [0,1] that this parser can read src,
	// based on extension and/or content sniffing, used for format auto-detect.
	Sniff(filename string, head []byte) float64
	// Parse fully materializes the source (used for Preview / Profiler).
	Parse(r io.Reader, opt Options) (Result, error)
	// Stream reads the source row by row without full materialization, used
	// by the Streaming Engine for multi-gigabyte files.
	Stream(r io.Reader, opt Options, handle RowHandler) error
}

var (
	mu       sync.RWMutex
	registry = map[string]Parser{}
)

// Register adds a parser plugin to the global registry. Called from each
// format's init().
func Register(p Parser) {
	mu.Lock()
	defer mu.Unlock()
	registry[p.Name()] = p
}

// Get looks up a parser by explicit format name.
func Get(name string) (Parser, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("parser: unknown format %q", name)
	}
	return p, nil
}

// Detect picks the best-scoring parser for a given filename + content head.
func Detect(filename string, head []byte) (Parser, float64) {
	mu.RLock()
	defer mu.RUnlock()
	var best Parser
	bestScore := 0.0
	for _, p := range registry {
		score := p.Sniff(filename, head)
		if score > bestScore {
			best, bestScore = p, score
		}
	}
	return best, bestScore
}

// List returns the names of every registered format, sorted for stable UI.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	return out
}
