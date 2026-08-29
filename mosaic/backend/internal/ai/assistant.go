// Package ai implements the AI Data Assistant: it turns a natural-language
// request ("convert age to integer, drop incomplete rows, join on
// customer_id, export to Excel") into a concrete, previewable pipeline plan
// using a small set of declared Tools, rather than freeform code generation
// — every action the assistant proposes is one of the Tools below, so the
// user always sees exactly what will change before it runs.
package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"mosaic/internal/pipeline"
)

// Provider abstracts the underlying LLM backend (hosted API or a local
// model) so MOSAIC isn't locked to one vendor; configured under
// Settings > AI Assistant with an API key stored in security.Vault.
type Provider interface {
	// Complete sends a system+user prompt and the available tool schema,
	// returning the model's raw text response (expected to be a ToolPlan
	// as JSON when tool use is requested).
	Complete(ctx context.Context, systemPrompt, userPrompt string, tools []ToolSpec) (string, error)
}

// ToolSpec describes one action the assistant is allowed to take, mirrored
// to the LLM as part of its tool-use schema.
type ToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Tools is the fixed, reviewable action surface of the AI Data Assistant.
// Restricting it to pipeline-construction primitives (rather than letting
// the model run arbitrary code) is what keeps every suggestion
// permission-aware and previewable.
var Tools = []ToolSpec{
	{Name: "addNode", Description: "Add a transform node of a given type with config to the pipeline canvas"},
	{Name: "connectNodes", Description: "Connect the output of one node to the input of another"},
	{Name: "explainSchema", Description: "Describe a dataset's inferred schema in plain language"},
	{Name: "findQualityIssues", Description: "Run the Data Quality Score and summarize the worst issues"},
	{Name: "suggestTransform", Description: "Suggest a transform node + config to fix a described problem"},
	{Name: "generateSQL", Description: "Generate a SQL Studio query from a natural-language description"},
	{Name: "generateRegex", Description: "Generate a regular expression matching a described pattern"},
	{Name: "explainPipelineError", Description: "Diagnose why a node failed using its error context"},
	{Name: "suggestPerformanceFix", Description: "Suggest streaming/caching/indexing changes for a slow pipeline on large data"},
}

// ToolCall is one action the assistant wants to take, as structured output
// from the model — never executed directly; always surfaced as a Plan for
// the user to approve first.
type ToolCall struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	Reason string         `json:"reason"`
}

// Plan is the full set of proposed changes for one natural-language
// request, rendered as a diff-like preview (new nodes + edges) before the
// user clicks "Apply to Canvas".
type Plan struct {
	Summary   string             `json:"summary"`
	Calls     []ToolCall         `json:"calls"`
	NewNodes  []pipeline.NodeDef `json:"newNodes"`
	NewEdges  []pipeline.EdgeDef `json:"newEdges"`
}

// Assistant orchestrates a Provider against the fixed Tools surface.
type Assistant struct {
	Provider Provider
}

const systemPrompt = `You are the MOSAIC AI Data Assistant. You convert data engineering
requests into a sequence of tool calls from the fixed tool list. You never invent tools,
never execute anything directly, and always explain your reasoning briefly. Respond with
strict JSON matching the Plan schema.`

// Plan asks the provider to turn a natural-language request (optionally
// with the current schema/pipeline as context) into a reviewable Plan.
func (a *Assistant) Plan(ctx context.Context, request string, schemaContext string) (*Plan, error) {
	if a.Provider == nil {
		return nil, fmt.Errorf("ai: no provider configured (set one under Settings > AI Assistant)")
	}
	userPrompt := request
	if schemaContext != "" {
		userPrompt = "Dataset context:\n" + schemaContext + "\n\nRequest: " + request
	}
	raw, err := a.Provider.Complete(ctx, systemPrompt, userPrompt, Tools)
	if err != nil {
		return nil, fmt.Errorf("ai: provider error: %w", err)
	}
	var plan Plan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, fmt.Errorf("ai: provider returned non-plan output: %w", err)
	}
	return &plan, nil
}
