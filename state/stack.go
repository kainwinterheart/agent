package state

type WorkflowStack struct {
	stack []*WorkflowStep
}

func (ws *WorkflowStack) Push(s *WorkflowStep) {
	ws.stack = append(ws.stack, s)
}

func (ws *WorkflowStack) NewStep(stepType WorkflowStepsteptype, state *WorkflowState) *WorkflowStepBuilder {
	return &WorkflowStepBuilder{
		steptype: stepType,
		state:    state,
	}
}

func (ws *WorkflowStack) Pop() (*WorkflowStep, bool) {
	if len(ws.stack) == 0 {
		return nil, false
	}
	idx := len(ws.stack) - 1
	v := ws.stack[idx]
	ws.stack = ws.stack[:idx]
	return v, true
}

func (ws *WorkflowStack) Len() int {
	return len(ws.stack)
}

func (ws *WorkflowStack) GetFrom(idx int) []WorkflowStep {
	out := []WorkflowStep{}
	for ; idx < ws.Len(); idx++ {
		out = append(out, *ws.stack[idx])
	}
	return out
}
