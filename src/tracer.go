package main

import (
	state "agent-go/state"
	td "agent-go/test_data"
	immutable "github.com/benbjohnson/immutable"
	"io"
)

type Tracer struct {
	Actions []td.DataAction
}

func (t *Tracer) trace(action string, details *td.ActionDetails) {
	t.Actions = append(t.Actions, *td.NewDataActionBuilder(nil).WithAction(action).WithDetails(details).Build())
}

func traceStack(w io.Writer, currentStep *state.WorkflowStep, followUpSteps []state.WorkflowStep, actions []td.DataAction) {
	data := MarshalJSON(td.NewTestCaseBuilder(nil).WithStep(currentStep).WithFollowups(immutable.NewList(followUpSteps...)).WithActions(immutable.NewList(actions...)).Build())
	w.Write([]byte(data))
	w.Write([]byte("\n"))
}
