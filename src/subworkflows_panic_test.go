package main

import (
	"strings"
	"testing"
)

// expectPanic runs fn and fails the test unless fn panics with a string
// message containing want.
func expectPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected a panic containing %q, but none occurred", want)
		}
		msg, isString := r.(string)
		if !isString || !strings.Contains(msg, want) {
			t.Fatalf("panic value %#v does not contain %q", r, want)
		}
	}()
	fn()
}

// TestInitSubworkflows_UnknownDecisionAgentPanics pins D-03 (defect register
// in roadmap/2026-09-20_commit_c9831d9_correctness_review.md): a decision
// agent definition whose name is neither workflow_driver nor loop_decider
// must trip the guard at subworkflows.go (panic "unknown decision agent ...").
func TestInitSubworkflows_UnknownDecisionAgentPanics(t *testing.T) {
	savedDefs := make([]AgentDefinition, len(AgentDefinitions))
	copy(savedDefs, AgentDefinitions)
	savedSubworkflows := Subworkflows
	savedDriver := DriverAgent
	savedDecider := LoopDeciderAgent
	defer func() {
		AgentDefinitions = savedDefs
		Subworkflows = savedSubworkflows
		DriverAgent = savedDriver
		LoopDeciderAgent = savedDecider
	}()

	AgentDefinitions = append(AgentDefinitions, AgentDefinition{
		Name:     "fake_decision_agent",
		Timeout:  "1m",
		Decision: true,
	})

	expectPanic(t, "unknown decision agent fake_decision_agent", initSubworkflows)
}

// TestInitSubworkflows_UnknownStepAgentPanics pins D-03: a subworkflow step
// referencing an agent that is not in AgentDefinitions must trip the guard at
// subworkflows.go (panic "subworkflow ... references unknown agent ...").
func TestInitSubworkflows_UnknownStepAgentPanics(t *testing.T) {
	savedDefs := make([]SubworkflowDefinition, len(SubworkflowDefinitions))
	copy(savedDefs, SubworkflowDefinitions)
	savedSubworkflows := Subworkflows
	savedDriver := DriverAgent
	savedDecider := LoopDeciderAgent
	defer func() {
		SubworkflowDefinitions = savedDefs
		Subworkflows = savedSubworkflows
		DriverAgent = savedDriver
		LoopDeciderAgent = savedDecider
	}()

	SubworkflowDefinitions = append(SubworkflowDefinitions, SubworkflowDefinition{
		ID:          "fake_subworkflow",
		Name:        "Fake",
		Description: "test-only subworkflow",
		Produces:    "fake",
		Steps: []StepDefinition{
			{Agent: "ghost_agent", ArtifactKind: "a ghost document"},
		},
	})

	expectPanic(t, "subworkflow fake_subworkflow references unknown agent ghost_agent", initSubworkflows)
}
