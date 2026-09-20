package main

import (
	"fmt"
	"strings"
)

// Decision is the workflow driver's validated response for one turn.
type Decision struct {
	Choice    string // subworkflow id, or "finish"
	Rationale string // driver's stated reason
	Task      string // the task for the subworkflow execution (becomes its artifact index section heading)
	Path      string // where the validated decision JSON is persisted
}

// buildDriverPrompt assembles the workflow driver's prompt: role, task, the
// full list of available subworkflows, the artifact index path, and the
// driver's own decision history. The JSON output contract (schema + example)
// lives in the role prompt.
func (o *Orchestrator) buildDriverPrompt(st *WorkflowState) string {
	var b strings.Builder
	b.WriteString(o.driverAgent.RolePrompt)
	b.WriteString("\n\n")

	b.WriteString("<task>\nORIGINAL TASK:\n" + wrapText(st.Task) + "\n</task>\n\n")

	b.WriteString(fmt.Sprintf("PROGRESS: this is driver iteration %d.\n\n", st.DriverIteration))

	b.WriteString("AVAILABLE SUBWORKFLOWS:\n")
	for i := range Subworkflows {
		sw := &Subworkflows[i]
		b.WriteString(fmt.Sprintf("* id: %s\n  name: %s\n  what it does: %s\n  produces: %s\n", sw.ID, sw.Name, sw.Description, sw.Produces))
	}
	b.WriteString("\n")

	b.WriteString("ARTIFACT INDEX:\n")
	b.WriteString("The system maintains an index of every document produced so far at:\n")
	b.WriteString("  " + indexPath(o.subdir) + "\n")
	b.WriteString("Each section describes one subworkflow execution: the task you gave it and the documents its agents produced. Read the index, then study any documents you need before deciding.\n\n")

	if len(st.History) > 0 {
		b.WriteString("DECISION HISTORY (your own previous turns in this run):\n")
		for i := range st.History {
			h := &st.History[i]
			b.WriteString(fmt.Sprintf("%d. subworkflow %q -> outcome: %s\n   rationale: %s\n", h.Iteration, h.Subworkflow, h.Outcome, truncate(h.Rationale, 200)))
			if h.Task != "" {
				b.WriteString("   task: " + truncate(h.Task, 200) + "\n")
			}
			if len(h.Artifacts) > 0 {
				b.WriteString("   artifacts: " + strings.Join(h.Artifacts, ", ") + "\n")
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// runDriver invokes the workflow driver agent until it returns a strictly
// validated JSON decision, then persists the validated decision for the
// record. The retry loop is unbounded.
func (o *Orchestrator) runDriver(st *WorkflowState, context *Context) *Decision {
	dd := runJSONDecision(o.driverAgent, o.buildDriverPrompt(st), decodeDriverDecision, driverDecisionExample, context)

	path := AbsPath(st.newDecisionPath(o.subdir))
	ensureParentDir(path)
	AtomicWrite(path, prettyJSON(dd)+"\n")

	dec := &Decision{
		Choice:    dd.Subworkflow,
		Rationale: dd.Rationale,
		Task:      dd.Task,
		Path:      path,
	}
	context.Tracer.trace("driver_decision", map[string]any{
		"iteration": st.DriverIteration,
		"choice":    dec.Choice,
		"rationale": dec.Rationale,
		"task":      dec.Task,
	})
	logStep(fmt.Sprintf("driver iteration %d: chose %q", st.DriverIteration, dec.Choice), "DRIVER")
	return dec
}
