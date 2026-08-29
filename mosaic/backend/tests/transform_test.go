package tests

import (
	"context"
	"testing"

	"mosaic/internal/pipeline"
	"mosaic/internal/schema"
	"mosaic/internal/transform"
)

func TestFilterRowsNode(t *testing.T) {
	n, _ := transform.New("filterRows")
	ctx := &transform.Context{Config: map[string]any{"expression": "age > 18"}, OnError: transform.OnErrorSkip}
	rows := []schema.Row{{"age": "20"}, {"age": "10"}}
	out, err := n.Run(ctx, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 row after filter, got %d", len(out))
	}
}

func TestJoinInner(t *testing.T) {
	n, _ := transform.New("join")
	ctx := &transform.Context{
		Config: map[string]any{
			"joinType":  "inner",
			"leftKeys":  []any{"id"},
			"rightKeys": []any{"customer_id"},
		},
		Inputs: map[string][]schema.Row{
			"right": {{"customer_id": "1", "email": "a@example.com"}},
		},
	}
	left := []schema.Row{{"id": "1", "name": "Alice"}, {"id": "2", "name": "Bob"}}
	out, err := n.Run(ctx, left)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 matched row, got %d", len(out))
	}
	if out[0]["email"] != "a@example.com" {
		t.Fatalf("expected joined email, got %v", out[0]["email"])
	}
}

func TestGroupByAggregate(t *testing.T) {
	n, _ := transform.New("groupByAggregate")
	ctx := &transform.Context{Config: map[string]any{
		"keys": []any{"category"},
		"aggregations": []any{
			map[string]any{"column": "amount", "func": "sum", "as": "total"},
		},
	}}
	rows := []schema.Row{
		{"category": "a", "amount": "10"},
		{"category": "a", "amount": "5"},
		{"category": "b", "amount": "3"},
	}
	out, err := n.Run(ctx, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(out))
	}
}

func TestPipelineExecutorRunsDAG(t *testing.T) {
	def := &pipeline.Definition{
		Nodes: []pipeline.NodeDef{
			{ID: "in", Type: "input"},
			{ID: "filter", Type: "filterRows", Config: map[string]any{"expression": "age > 18"}},
			{ID: "out", Type: "output"},
		},
		Edges: []pipeline.EdgeDef{
			{From: "in", To: "filter"},
			{From: "filter", To: "out"},
		},
	}
	g, err := pipeline.Build(def)
	if err != nil {
		t.Fatal(err)
	}
	ex := pipeline.NewExecutor()
	sources := pipeline.Sources{"in": {{"age": "30"}, {"age": "5"}}}
	results, err := ex.Run(context.Background(), g, sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(results["out"]) != 1 {
		t.Fatalf("expected 1 row at output, got %d", len(results["out"]))
	}
}

func TestPipelineDetectsCycles(t *testing.T) {
	def := &pipeline.Definition{
		Nodes: []pipeline.NodeDef{{ID: "a", Type: "input"}, {ID: "b", Type: "output"}},
		Edges: []pipeline.EdgeDef{{From: "a", To: "b"}, {From: "b", To: "a"}},
	}
	g, err := pipeline.Build(def)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Levels(); err == nil {
		t.Fatal("expected cycle detection error")
	}
}
