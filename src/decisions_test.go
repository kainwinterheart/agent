package main

import (
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"pure json", `{"a":1}`, `{"a":1}`, false},
		{"fenced", "Here you go:\n```json\n{\"a\":1}\n```\nDone.", `{"a":1}`, false},
		{"embedded in prose", `Sure! The decision is {"a":{"b":2}} as requested.`, `{"a":{"b":2}}`, false},
		{"no json", "I cannot decide.", "", true},
		{"only open brace", "text { more text", "", true},
	}
	for _, tc := range cases {
		got, err := ExtractJSON(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error, got %q", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: ExtractJSON = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDecodeDriverDecision(t *testing.T) {
	valid := `{"subworkflow":"classify","rationale":"The task is ambiguous.","task":"Classify the task."}`
	d, err := decodeDriverDecision(valid)
	if err != nil {
		t.Fatalf("valid decision rejected: %v", err)
	}
	if d.Subworkflow != "classify" || d.Rationale != "The task is ambiguous." || d.Task != "Classify the task." {
		t.Errorf("decoded = %+v", d)
	}

	finish, err := decodeDriverDecision(`{"subworkflow":"finish","rationale":"done","task":"All milestones implemented and reviewed."}`)
	if err != nil || finish.Subworkflow != "finish" {
		t.Errorf("finish decision = %+v, err = %v", finish, err)
	}

	full, err := decodeDriverDecision(`{"subworkflow":"implement","rationale":"r","task":"b"}`)
	if err != nil {
		t.Fatalf("full decision rejected: %v", err)
	}
	if full.Task != "b" {
		t.Errorf("full decoded = %+v", full)
	}

	// Ids are matched case-insensitively, like the subworkflow registry lookup.
	up, err := decodeDriverDecision(`{"subworkflow":"IMPLement","rationale":"r","task":"t"}`)
	if err != nil || up.Subworkflow != "implement" {
		t.Errorf("uppercase id = %+v, err = %v", up, err)
	}

	invalid := []string{
		`{"rationale":"missing subworkflow"}`,
		`{"subworkflow":"","rationale":"empty id"}`,
		`{"subworkflow":"classify"}`,
		`{"subworkflow":"classify","rationale":""}`,
		`{"subworkflow":"definitely_not_a_subworkflow","rationale":"unknown id"}`,
		`{"subworkflow":"classify","rationale":"r","unexpected_field":true}`,
		`{"subworkflow":"classify","rationale":"r","context_ids":["001-x"]}`,
		`{"subworkflow":42,"rationale":"wrong type"}`,
		`{"subworkflow":"classify","rationale":"r"}`,
		`{"subworkflow":"classify","rationale":"r","task":""}`,
		`not json at all`,
	}
	for _, in := range invalid {
		if _, err := decodeDriverDecision(in); err == nil {
			t.Errorf("invalid driver decision accepted: %s", in)
		}
	}
}

func TestDecodeLoopDecision(t *testing.T) {
	d, err := decodeLoopDecision(`{"repeat":true,"reason":"two high-severity issues remain"}`)
	if err != nil || !d.Repeat || d.Reason == "" {
		t.Errorf("valid repeat decision = %+v, err = %v", d, err)
	}
	d2, err := decodeLoopDecision(`{"repeat":false,"reason":"reviews are positive"}`)
	if err != nil || d2.Repeat {
		t.Errorf("valid stop decision = %+v, err = %v", d2, err)
	}

	invalid := []string{
		`{"reason":"missing repeat"}`,
		`{"repeat":true}`,
		`{"repeat":true,"reason":""}`,
		`{"repeat":true,"reason":"r","extra":1}`,
		`{"repeat":"yes","reason":"wrong type"}`,
		`no json`,
	}
	for _, in := range invalid {
		if _, err := decodeLoopDecision(in); err == nil {
			t.Errorf("invalid loop decision accepted: %s", in)
		}
	}
}

func TestDriverSchemaMatchesRegistry(t *testing.T) {
	schema := driverDecisionSchema()
	props, _ := schema["properties"].(map[string]any)
	enum, _ := props["subworkflow"].(map[string]any)["enum"].([]any)
	want := append(append([]string{}, subworkflowIDs()...), "finish")
	if len(enum) != len(want) {
		t.Fatalf("schema enum = %v, want %v", enum, want)
	}
	for i, id := range want {
		if enum[i].(string) != id {
			t.Errorf("schema enum[%d] = %v, want %s", i, enum[i], id)
		}
	}
	req, _ := schema["required"].([]string)
	if len(req) != 3 || req[0] != "subworkflow" || req[1] != "rationale" || req[2] != "task" {
		t.Errorf("schema required = %v", req)
	}
	if ap, _ := schema["additionalProperties"].(bool); ap {
		t.Error("schema must reject additional properties")
	}
}

func TestDecisionPromptsCarrySchemaAndExample(t *testing.T) {
	if DriverAgent == nil || DriverAgent.Schema == nil {
		t.Fatal("driver agent missing or has no schema")
	}
	if LoopDeciderAgent == nil || LoopDeciderAgent.Schema == nil {
		t.Fatal("loop decider agent missing or has no schema")
	}
	for name, a := range map[string]*Agent{"workflow_driver": DriverAgent, "loop_decider": LoopDeciderAgent} {
		p := a.RolePrompt
		if !strings.Contains(p, "Output MUST be valid JSON only") {
			t.Errorf("%s prompt missing JSON output contract", name)
		}
		if !strings.Contains(p, prettyJSON(a.Schema)) {
			t.Errorf("%s prompt missing its schema", name)
		}
		if !strings.Contains(p, "Example of a valid response") {
			t.Errorf("%s prompt missing the example response", name)
		}
		if strings.Contains(p, "{{") {
			t.Errorf("%s prompt has unrendered template markers", name)
		}
	}
}

func TestDecodeFinishReassessment(t *testing.T) {
	confirm, err := decodeFinishReassessment(`{"decision":"confirm_finish","subworkflow":"finish","rationale":"All milestones implemented and reviewed: 007-implement_coder.md.","task":"The requested implementation is complete."}`)
	if err != nil {
		t.Fatalf("valid confirmation rejected: %v", err)
	}
	if confirm.Decision != "confirm_finish" || confirm.Subworkflow != "finish" {
		t.Errorf("decoded confirmation = %+v", confirm)
	}

	selectSW, err := decodeFinishReassessment(`{"decision":"SELECT_Subworkflow","subworkflow":"Spec","rationale":"Not started.","task":"Produce the spec."}`)
	if err != nil {
		t.Fatalf("valid selection rejected: %v", err)
	}
	if selectSW.Decision != "select_subworkflow" || selectSW.Subworkflow != "spec" {
		t.Errorf("decoded selection = %+v", selectSW)
	}

	invalid := []string{
		`{"subworkflow":"finish","rationale":"r","task":"t"}`,                                       // missing decision
		`{"decision":"","subworkflow":"finish","rationale":"r","task":"t"}`,                         // empty decision
		`{"decision":"maybe","subworkflow":"finish","rationale":"r","task":"t"}`,                    // unknown branch
		`{"decision":"confirm_finish","subworkflow":"spec","rationale":"r","task":"t"}`,             // confirm must select finish
		`{"decision":"select_subworkflow","subworkflow":"finish","rationale":"r","task":"t"}`,       // select must not select finish
		`{"decision":"select_subworkflow","subworkflow":"nope","rationale":"r","task":"t"}`,         // unknown subworkflow
		`{"decision":"confirm_finish","subworkflow":"finish","task":"t"}`,                           // missing rationale
		`{"decision":"confirm_finish","subworkflow":"finish","rationale":"r"}`,                      // missing task
		`{"decision":"confirm_finish","subworkflow":"finish","rationale":"r","task":"t","extra":1}`, // unknown field
		`not json`,
	}
	for _, in := range invalid {
		if _, err := decodeFinishReassessment(in); err == nil {
			t.Errorf("invalid reassessment accepted: %s", in)
		}
	}
}

func TestFinishReassessmentSchemaShape(t *testing.T) {
	schema := finishReassessmentSchema()
	props, _ := schema["properties"].(map[string]any)

	branchEnum, _ := props["decision"].(map[string]any)["enum"].([]any)
	if len(branchEnum) != 2 || branchEnum[0].(string) != "confirm_finish" || branchEnum[1].(string) != "select_subworkflow" {
		t.Errorf("decision enum = %v", branchEnum)
	}

	subEnum, _ := props["subworkflow"].(map[string]any)["enum"].([]any)
	want := append(append([]string{}, subworkflowIDs()...), "finish")
	if len(subEnum) != len(want) {
		t.Fatalf("subworkflow enum = %v, want %v", subEnum, want)
	}
	for i, id := range want {
		if subEnum[i].(string) != id {
			t.Errorf("subworkflow enum[%d] = %v, want %s", i, subEnum[i], id)
		}
	}

	req, _ := schema["required"].([]string)
	if len(req) != 4 || req[0] != "decision" || req[1] != "subworkflow" || req[2] != "rationale" || req[3] != "task" {
		t.Errorf("required = %v", req)
	}
	if ap, _ := schema["additionalProperties"].(bool); ap {
		t.Error("schema must reject additional properties")
	}
}
