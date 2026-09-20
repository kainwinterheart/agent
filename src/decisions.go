package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Decision agents are lightweight control-flow agents. Unlike content agents,
// whose deliverable is a Markdown document on disk, decision agents respond
// with a single JSON document that is strictly validated against a JSON
// schema. Their prompts carry the schema and an example response, and the
// runtime passes the schema to the LLM via --output-schema.

// DriverDecision is the workflow driver's response.
type DriverDecision struct {
	Subworkflow string `json:"subworkflow"` // subworkflow id, or "finish"
	Rationale   string `json:"rationale"`   // driver's stated reason
	Task        string `json:"task"`        // the task for the subworkflow execution
}

// LoopDecision is the loop decider's response.
type LoopDecision struct {
	Repeat bool   `json:"repeat"`
	Reason string `json:"reason"`
}

func driverDecisionSchema() map[string]any {
	ids := append(append([]string{}, subworkflowIDs()...), "finish")
	enum := make([]any, 0, len(ids))
	for _, id := range ids {
		enum = append(enum, id)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subworkflow": map[string]any{"type": "string", "enum": enum},
			"rationale":   map[string]any{"type": "string"},
			"task":        map[string]any{"type": "string"},
		},
		"required":             []string{"subworkflow", "rationale", "task"},
		"additionalProperties": false,
	}
}

func loopDecisionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"repeat": map[string]any{"type": "boolean"},
			"reason": map[string]any{"type": "string"},
		},
		"required":             []string{"repeat", "reason"},
		"additionalProperties": false,
	}
}

const driverDecisionExample = `{
  "subworkflow": "implement",
  "rationale": "The architecture and plan for the auth domain are approved; the plan's first milestone is ready for implementation.",
  "task": "Implement milestone 1 of the plan: token issuance and storage. Follow the plan's file list; do not touch other domains."
}`

const loopDecisionExample = `{
  "repeat": true,
  "reason": "The code review rejected the implementation report: two claimed files do not exist in the repository, which the next round must fix."
}`

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("prettyJSON: %v", err))
	}
	return string(b)
}

// ExtractJSON pulls the outermost JSON object out of arbitrary model output.
func ExtractJSON(text string) (string, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end > start {
		return text[start : end+1], nil
	}
	return "", fmt.Errorf("text does not contain a JSON object")
}

// decodeDriverDecision strictly validates a driver response against
// driverDecisionSchema: unknown fields are rejected, required fields must be
// present and non-empty, and the subworkflow id must be known.
func decodeDriverDecision(jsonText string) (DriverDecision, error) {
	var raw struct {
		Subworkflow *string `json:"subworkflow"`
		Rationale   *string `json:"rationale"`
		Task        *string `json:"task"`
	}
	dec := json.NewDecoder(strings.NewReader(jsonText))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return DriverDecision{}, fmt.Errorf("invalid JSON object: %v", err)
	}
	if raw.Subworkflow == nil || strings.TrimSpace(*raw.Subworkflow) == "" {
		return DriverDecision{}, fmt.Errorf("missing required field \"subworkflow\"")
	}
	choice := strings.ToLower(strings.TrimSpace(*raw.Subworkflow))
	if choice != "finish" && getSubworkflow(choice) == nil {
		return DriverDecision{}, fmt.Errorf("unknown subworkflow id %q; valid ids: %s", choice, strings.Join(append(subworkflowIDs(), "finish"), ", "))
	}
	if raw.Rationale == nil || strings.TrimSpace(*raw.Rationale) == "" {
		return DriverDecision{}, fmt.Errorf("missing required field \"rationale\" (non-empty string)")
	}
	if raw.Task == nil || strings.TrimSpace(*raw.Task) == "" {
		return DriverDecision{}, fmt.Errorf("missing required field \"task\" (non-empty string)")
	}
	d := DriverDecision{
		Subworkflow: choice,
		Rationale:   strings.TrimSpace(*raw.Rationale),
		Task:        strings.TrimSpace(*raw.Task),
	}
	return d, nil
}

// decodeLoopDecision strictly validates a loop decider response against
// loopDecisionSchema.
func decodeLoopDecision(jsonText string) (LoopDecision, error) {
	var raw struct {
		Repeat *bool   `json:"repeat"`
		Reason *string `json:"reason"`
	}
	dec := json.NewDecoder(strings.NewReader(jsonText))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return LoopDecision{}, fmt.Errorf("invalid JSON object: %v", err)
	}
	if raw.Repeat == nil {
		return LoopDecision{}, fmt.Errorf("missing required field \"repeat\" (boolean)")
	}
	if raw.Reason == nil || strings.TrimSpace(*raw.Reason) == "" {
		return LoopDecision{}, fmt.Errorf("missing required field \"reason\" (non-empty string)")
	}
	return LoopDecision{Repeat: *raw.Repeat, Reason: strings.TrimSpace(*raw.Reason)}, nil
}

// jsonFeedbackBlock is appended to the decision agent's prompt when its
// previous response failed strict validation.
func jsonFeedbackBlock(problem, schemaText, example string) string {
	return fmt.Sprintf(`

<feedback>
Your previous response was INVALID.
<error>
%s
</error>

Output MUST be valid JSON only, conforming to this schema:
%s

Example of a valid response:
%s
</feedback>
`, problem, schemaText, example)
}

// runJSONDecision invokes a decision agent until its response passes strict
// validation. The loop is unbounded: every invalid response is fed back to the
// agent for another attempt.
func runJSONDecision[T any](agent *Agent, basePrompt string, decode func(string) (T, error), example string, context *Context) T {
	cur := basePrompt
	attempt := 0
	for {
		attempt++
		context.Tracer.trace("decision_invoke", map[string]any{
			"agent": agent.Name, "attempt": attempt, "prompt": cur,
		})

		var out T
		var problem string
		stdout, runErr := agent.Run(cur, context)
		if runErr != nil {
			problem = "the agent run itself failed: " + runErr.Error()
		} else if j, err := ExtractJSON(stdout); err != nil {
			problem = "the response contains no JSON object: " + err.Error()
		} else if out, err = decode(j); err != nil {
			problem = "the response failed strict JSON validation: " + err.Error()
		} else {
			context.Tracer.trace("decision_result", map[string]any{
				"agent": agent.Name, "attempt": attempt, "response": prettyJSON(out),
			})
			logStep(fmt.Sprintf("%s: valid decision after %d attempt(s)", agent.Name, attempt), "DECISION")
			return out
		}

		logStep(fmt.Sprintf("%s attempt %d invalid: %s", agent.Name, attempt, problem), "DECISION")
		context.Tracer.trace("decision_retry", map[string]any{
			"agent": agent.Name, "attempt": attempt, "problem": problem,
		})
		cur = basePrompt + jsonFeedbackBlock(problem, prettyJSON(agent.Schema), example)
	}
}

// FinishReassessment is the driver's response to a mandatory finish
// reassessment. The decision field makes the branch explicit: confirming the
// finish is a named act ("confirm_finish"), distinct from selecting the
// subworkflow that was actually needed ("select_subworkflow"). The two
// branches are mutually exclusive with the subworkflow field, and the
// combination is validated strictly.
type FinishReassessment struct {
	Decision    string `json:"decision"`    // "confirm_finish" or "select_subworkflow"
	Subworkflow string `json:"subworkflow"` // "finish" when confirming; a subworkflow id when selecting
	Rationale   string `json:"rationale"`
	Task        string `json:"task"`
}

func finishReassessmentSchema() map[string]any {
	ids := append(append([]string{}, subworkflowIDs()...), "finish")
	enum := make([]any, 0, len(ids))
	for _, id := range ids {
		enum = append(enum, id)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"decision": map[string]any{
				"type": "string",
				"enum": []any{"confirm_finish", "select_subworkflow"},
			},
			"subworkflow": map[string]any{"type": "string", "enum": enum},
			"rationale":   map[string]any{"type": "string"},
			"task":        map[string]any{"type": "string"},
		},
		"required":             []string{"decision", "subworkflow", "rationale", "task"},
		"additionalProperties": false,
	}
}

const finishReassessmentExample = `{
  "decision": "select_subworkflow",
  "subworkflow": "spec",
  "rationale": "The run is in BOOTSTRAP: the artifact index has zero completed executions, so the work has not started and finish cannot be correct. The first step is the refined task specification.",
  "task": "Produce the durable refined task specification from the user's request."
}`

// decodeFinishReassessment strictly validates a finish-reassessment response:
// unknown fields are rejected, all four fields must be present and non-empty,
// and the decision/subworkflow combination must be coherent:
//
//	confirm_finish       requires subworkflow == "finish"
//	select_subworkflow   requires a known subworkflow id (never "finish")
func decodeFinishReassessment(jsonText string) (FinishReassessment, error) {
	var raw struct {
		Decision    *string `json:"decision"`
		Subworkflow *string `json:"subworkflow"`
		Rationale   *string `json:"rationale"`
		Task        *string `json:"task"`
	}
	dec := json.NewDecoder(strings.NewReader(jsonText))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return FinishReassessment{}, fmt.Errorf("invalid JSON object: %v", err)
	}
	if raw.Decision == nil || strings.TrimSpace(*raw.Decision) == "" {
		return FinishReassessment{}, fmt.Errorf("missing required field \"decision\" (non-empty string)")
	}
	branch := strings.ToLower(strings.TrimSpace(*raw.Decision))
	if branch != "confirm_finish" && branch != "select_subworkflow" {
		return FinishReassessment{}, fmt.Errorf("invalid decision %q; must be \"confirm_finish\" or \"select_subworkflow\"", branch)
	}
	if raw.Subworkflow == nil || strings.TrimSpace(*raw.Subworkflow) == "" {
		return FinishReassessment{}, fmt.Errorf("missing required field \"subworkflow\"")
	}
	choice := strings.ToLower(strings.TrimSpace(*raw.Subworkflow))
	switch branch {
	case "confirm_finish":
		if choice != "finish" {
			return FinishReassessment{}, fmt.Errorf("decision \"confirm_finish\" requires subworkflow \"finish\", got %q", choice)
		}
	case "select_subworkflow":
		if choice == "finish" {
			return FinishReassessment{}, fmt.Errorf("decision \"select_subworkflow\" must not select \"finish\"; name the subworkflow to run")
		}
		if getSubworkflow(choice) == nil {
			return FinishReassessment{}, fmt.Errorf("unknown subworkflow id %q; valid ids: %s", choice, strings.Join(subworkflowIDs(), ", "))
		}
	}
	if raw.Rationale == nil || strings.TrimSpace(*raw.Rationale) == "" {
		return FinishReassessment{}, fmt.Errorf("missing required field \"rationale\" (non-empty string)")
	}
	if raw.Task == nil || strings.TrimSpace(*raw.Task) == "" {
		return FinishReassessment{}, fmt.Errorf("missing required field \"task\" (non-empty string)")
	}
	return FinishReassessment{
		Decision:    branch,
		Subworkflow: choice,
		Rationale:   strings.TrimSpace(*raw.Rationale),
		Task:        strings.TrimSpace(*raw.Task),
	}, nil
}
