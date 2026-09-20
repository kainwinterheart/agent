package main

import (
	"strings"
	"testing"
)

func TestRenderIndex_Empty(t *testing.T) {
	st := newWorkflowState("t", "")
	idx := renderIndex(st)
	if !strings.Contains(idx, "# Artifact Index") {
		t.Errorf("missing title: %s", idx)
	}
	if !strings.Contains(idx, "No subworkflow executions yet.") {
		t.Errorf("empty state must say so: %s", idx)
	}
}

func TestRenderIndex_HistorySections(t *testing.T) {
	st := newWorkflowState("t", "")
	st.Artifacts = []Artifact{
		{ID: "002-a", Iteration: 1, Subworkflow: "spec", Agent: "product_manager", Round: 1, Description: "Spec document", Path: "/s/002.md"},
		{ID: "003-b", Iteration: 1, Subworkflow: "spec", Agent: "pm_review", Round: 1, Description: "Spec review", Path: "/s/003.md"},
		{ID: "004-c", Iteration: 2, Subworkflow: "classify", Agent: "investigation_classifier", Round: 1, Description: "Classification", Path: "/s/004.md"},
	}
	st.History = []HistoryEntry{
		{Iteration: 1, Subworkflow: "spec", Task: "Refine the spec.", Outcome: "completed", Artifacts: []string{"002-a", "003-b"}},
		{Iteration: 2, Subworkflow: "classify", Task: "Classify.", Outcome: "completed", Artifacts: []string{"004-c"}},
		{Iteration: 3, Subworkflow: "finish", Task: "Done.", Outcome: "finished"},
	}
	idx := renderIndex(st)
	for _, want := range []string{
		"## Iteration 1 — subworkflow: spec",
		"Task: Refine the spec.",
		"- /s/002.md — Spec document (agent: product_manager, round 1)",
		"- /s/003.md — Spec review (agent: pm_review, round 1)",
		"## Iteration 2 — subworkflow: classify",
		"- /s/004.md — Classification (agent: investigation_classifier, round 1)",
	} {
		if !strings.Contains(idx, want) {
			t.Errorf("index missing %q:\n%s", want, idx)
		}
	}
	// Finish turns produce no section.
	if strings.Contains(idx, "subworkflow: finish") {
		t.Error("finish turn must not create an index section")
	}
}

func TestRenderIndex_ActiveExecution(t *testing.T) {
	st := newWorkflowState("t", "")
	st.Artifacts = []Artifact{
		{ID: "002-a", Iteration: 1, Subworkflow: "spec", Agent: "product_manager", Round: 1, Description: "Spec document", Path: "/s/002.md"},
	}
	st.History = []HistoryEntry{
		{Iteration: 1, Subworkflow: "spec", Task: "Refine the spec.", Outcome: "completed", Artifacts: []string{"002-a"}},
	}
	st.Active = &ActiveExecution{Iteration: 2, Subworkflow: "architect", Task: "Design the auth domain."}
	idx := renderIndex(st)
	for _, want := range []string{
		"## Iteration 2 — subworkflow: architect",
		"Task: Design the auth domain.",
		"- (none yet)",
	} {
		if !strings.Contains(idx, want) {
			t.Errorf("index missing %q:\n%s", want, idx)
		}
	}

	// Artifacts of the active iteration appear as soon as they are produced.
	st.Artifacts = append(st.Artifacts, Artifact{ID: "003-b", Iteration: 2, Subworkflow: "architect", Agent: "arch", Round: 1, Description: "Architecture", Path: "/s/003.md"})
	idx = renderIndex(st)
	if !strings.Contains(idx, "- /s/003.md — Architecture (agent: arch, round 1)") {
		t.Errorf("active section missing produced document:\n%s", idx)
	}
	if strings.Contains(idx, "- (none yet)") {
		t.Error("(none yet) must disappear once documents exist")
	}
}
