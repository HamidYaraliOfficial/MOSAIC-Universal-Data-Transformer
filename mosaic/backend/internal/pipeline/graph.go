// Package pipeline implements the Pipeline Runtime: turning the JSON graph
// the Visual Pipeline Canvas produces into an executable dependency graph,
// running independent branches in parallel via goroutines, and reporting
// per-node metrics back to the Execution Inspector.
package pipeline

import (
	"fmt"

	"mosaic/internal/transform"
)

// NodeDef is one node as saved in the versioned pipeline JSON file.
type NodeDef struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Config   map[string]any         `json:"config"`
	OnError  transform.OnErrorMode  `json:"onError"`
	Disabled bool                   `json:"disabled"`
	Position [2]float64             `json:"position"`
}

// EdgeDef connects an output port of one node to an input port of another.
// Port defaults to "in" for the primary stream; named ports ("right",
// "reference", "b", ...) feed transform.Context.Inputs for multi-input
// nodes like Join, Lookup/Enrich and Union.
type EdgeDef struct {
	From     string `json:"from"`
	FromPort string `json:"fromPort"`
	To       string `json:"to"`
	ToPort   string `json:"toPort"`
}

// Definition is the full versioned, saveable/exportable pipeline document.
type Definition struct {
	Version     int       `json:"version"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Nodes       []NodeDef `json:"nodes"`
	Edges       []EdgeDef `json:"edges"`
}

// Graph is the resolved, in-memory form of a Definition used by the
// executor: adjacency lists in both directions plus fast node lookup.
type Graph struct {
	Def      *Definition
	byID     map[string]*NodeDef
	incoming map[string][]EdgeDef // edges where node is the target
	outDeg   map[string]int
}

// Build resolves a Definition into a Graph, validating that every edge
// references an existing node.
func Build(def *Definition) (*Graph, error) {
	g := &Graph{
		Def:      def,
		byID:     map[string]*NodeDef{},
		incoming: map[string][]EdgeDef{},
		outDeg:   map[string]int{},
	}
	for i := range def.Nodes {
		g.byID[def.Nodes[i].ID] = &def.Nodes[i]
	}
	for _, e := range def.Edges {
		if _, ok := g.byID[e.From]; !ok {
			return nil, fmt.Errorf("pipeline: edge references unknown source node %q", e.From)
		}
		if _, ok := g.byID[e.To]; !ok {
			return nil, fmt.Errorf("pipeline: edge references unknown target node %q", e.To)
		}
		g.incoming[e.To] = append(g.incoming[e.To], e)
	}
	return g, nil
}

// Levels performs a Kahn's-algorithm topological sort, grouping nodes into
// "levels": each level's nodes have no dependency on each other and can run
// concurrently, which is exactly what the executor's worker pool exploits.
func (g *Graph) Levels() ([][]*NodeDef, error) {
	indeg := map[string]int{}
	for id := range g.byID {
		indeg[id] = len(uniqueSources(g.incoming[id]))
	}

	var levels [][]*NodeDef
	remaining := len(g.byID)
	processed := map[string]bool{}

	for remaining > 0 {
		var level []*NodeDef
		for id, d := range indeg {
			if !processed[id] && d == 0 {
				level = append(level, g.byID[id])
			}
		}
		if len(level) == 0 {
			return nil, fmt.Errorf("pipeline: cycle detected in pipeline graph")
		}
		for _, n := range level {
			processed[n.ID] = true
			remaining--
			for _, e := range g.Def.Edges {
				if e.From == n.ID && !processed[e.To] {
					indeg[e.To]--
				}
			}
		}
		levels = append(levels, level)
	}
	return levels, nil
}

func uniqueSources(edges []EdgeDef) map[string]bool {
	m := map[string]bool{}
	for _, e := range edges {
		m[e.From] = true
	}
	return m
}

// IncomingByPort splits a node's incoming edges by target port name,
// defaulting empty port names to "in" (the primary stream).
func (g *Graph) IncomingByPort(nodeID string) map[string][]EdgeDef {
	out := map[string][]EdgeDef{}
	for _, e := range g.incoming[nodeID] {
		port := e.ToPort
		if port == "" {
			port = "in"
		}
		out[port] = append(out[port], e)
	}
	return out
}
