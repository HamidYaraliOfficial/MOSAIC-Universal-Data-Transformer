// Package cache implements MOSAIC's Node Output Cache: if a node's inputs
// and configuration haven't changed since the last run, the pipeline
// executor can reuse its previous output instead of recomputing it, which
// matters a lot on iterative editing of large pipelines.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"

	"mosaic/internal/schema"
)

// Entry is one cached node result along with the fingerprint that produced
// it, so a later lookup can confirm the fingerprint still matches.
type Entry struct {
	Fingerprint string
	Rows        []schema.Row
}

// Cache is a simple in-memory, thread-safe store keyed by node ID. It is
// intentionally process-local (not persisted) — restarting the app forces a
// clean run, which is the safe default for a data engineering tool.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

// New creates an empty cache.
func New() *Cache { return &Cache{entries: map[string]Entry{}} }

// Fingerprint computes a deterministic content hash of a node's config plus
// its resolved input row sets, used to detect "nothing relevant changed"
// between two pipeline runs (Invalidate-by-Input-Hash / Config / Dependency).
func Fingerprint(nodeType string, config map[string]any, inputs ...[]schema.Row) string {
	h := sha256.New()
	h.Write([]byte(nodeType))
	if cfgBytes, err := json.Marshal(sortedConfig(config)); err == nil {
		h.Write(cfgBytes)
	}
	for _, in := range inputs {
		h.Write([]byte{0})
		// Hashing row count + a sample keeps fingerprinting O(sample) even
		// for huge datasets, trading a (very small) chance of a missed
		// invalidation for speed; exact hashing is used below <32k rows.
		if len(in) < 32000 {
			if b, err := json.Marshal(in); err == nil {
				h.Write(b)
			}
		} else {
			sample := append(append([]schema.Row(nil), in[:4000]...), in[len(in)-4000:]...)
			if b, err := json.Marshal(sample); err == nil {
				h.Write(b)
			}
			h.Write([]byte(itoa(len(in))))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sortedConfig(cfg map[string]any) map[string]any {
	// json.Marshal of a Go map already sorts keys, but we make the
	// intent explicit for readers of this file.
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return cfg
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// Get returns a cached result only if the fingerprint matches exactly.
func (c *Cache) Get(nodeID, fingerprint string) ([]schema.Row, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[nodeID]
	if !ok || e.Fingerprint != fingerprint {
		return nil, false
	}
	return e.Rows, true
}

// Set stores a node's output under its current fingerprint.
func (c *Cache) Set(nodeID, fingerprint string, rows []schema.Row) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[nodeID] = Entry{Fingerprint: fingerprint, Rows: rows}
}

// Invalidate drops a single node's cache entry (e.g. when the user manually
// forces a re-run) and, if cascade is true, every entry — used when an
// upstream Input node's source data changes.
func (c *Cache) Invalidate(nodeID string, cascade bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cascade {
		c.entries = map[string]Entry{}
		return
	}
	delete(c.entries, nodeID)
}
