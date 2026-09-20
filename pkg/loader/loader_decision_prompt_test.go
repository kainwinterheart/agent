package loader

import (
	"strings"
	"testing"
)

// TestRenderDecisionPrompt_NoMissingValues pins D-01 (defect register in
// roadmap/2026-09-20_commit_c9831d9_correctness_review.md): the two decision
// prompts are the only templates with data fields ({{.Schema}}/{{.Example}}),
// and rendering them with nil data silently emits "<no value>" on the pinned
// toolchain (go1.26.4) without an error. Rendering them through the real
// RenderDecisionPrompt code path with real-shaped schema/example data must
// yield zero "<no value>" occurrences and must actually contain the injected
// schema and example. The data mirrors src/decisions.go
// (driverDecisionSchema / loopDecisionSchema / driverDecisionExample /
// loopDecisionExample); it lives in package main, so the shapes are repeated
// here rather than imported.
func TestRenderDecisionPrompt_NoMissingValues(t *testing.T) {
	InitPromptLoader()

	cases := []struct {
		name    string
		id      PromptID
		schema  map[string]any
		example string
	}{
		{
			name: "workflow_driver",
			id:   WORKFLOW_DRIVER_PROMPT_ID,
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"subworkflow": map[string]any{"type": "string", "enum": []any{"implement", "finish"}},
					"rationale":   map[string]any{"type": "string"},
					"task":        map[string]any{"type": "string"},
				},
				"required":             []string{"subworkflow", "rationale", "task"},
				"additionalProperties": false,
			},
			example: `{"subworkflow": "implement", "rationale": "The plan's first milestone is ready for implementation.", "task": "Implement milestone 1 of the plan."}`,
		},
		{
			name: "loop_decider",
			id:   LOOP_DECIDER_PROMPT_ID,
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"repeat": map[string]any{"type": "boolean"},
					"reason": map[string]any{"type": "string"},
				},
				"required":             []string{"repeat", "reason"},
				"additionalProperties": false,
			},
			example: `{"repeat": true, "reason": "The code review rejected the implementation report."}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := RenderDecisionPrompt(tc.id, tc.schema, tc.example)
			if n := strings.Count(out, "<no value>"); n != 0 {
				t.Errorf("rendered %s prompt contains %d occurrence(s) of <no value>", tc.name, n)
			}
			if !strings.Contains(out, `"additionalProperties": false`) {
				t.Errorf("rendered %s prompt does not contain the injected schema", tc.name)
			}
			if !strings.Contains(out, tc.example) {
				t.Errorf("rendered %s prompt does not contain the injected example", tc.name)
			}
		})
	}
}
