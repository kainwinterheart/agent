package main

import (
	"fmt"
	"strings"

	"agent-go/pkg/loader"
)

// finishReassessmentMarker heads the mandatory reassessment block that is
// appended to the driver prompt whenever the driver responds with "finish";
// the session trace and the tests key off it to identify reassessment turns.
// It is intentionally distinct from the role prompt's "FINISH REASSESSMENT"
// section header so reassessment prompts can be told apart from regular ones.
const finishReassessmentMarker = "FINISH REASSESSMENT - MANDATORY"

// bootstrapRunStatePrefix opens the authoritative bootstrap line stated in
// every driver prompt while the run has zero completed executions. It is
// deliberately distinct from the role prompt's documentation of the same
// line, so the two occurrences can be told apart.
const bootstrapRunStatePrefix = "RUN STATE: BOOTSTRAP - "

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

	// The authoritative run state is stated directly: the driver must not
	// have to read the index to know whether the run has started.
	if n := completedExecutions(st); n == 0 {
		b.WriteString(bootstrapRunStatePrefix + "the artifact index contains ZERO completed workflow executions. The run has NOT started. Per RULE 1, the only valid decision now is subworkflow \"spec\"; \"finish\" is INVALID.\n\n")
	} else {
		b.WriteString(fmt.Sprintf("RUN STATE: %d completed workflow execution(s) in the artifact index. Your job this turn: select the subworkflow that must run next. \"finish\" is legal only if completed artifacts in the index prove the requested outcome is achieved and reviewed.\n\n", n))
	}

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
			if h.FinishDecisionPath != "" {
				if h.FinishConfirmed {
					b.WriteString("   note: finish confirmed on the mandatory finish reassessment\n")
				} else {
					b.WriteString("   note: the driver initially chose \"finish\"; the mandatory finish reassessment overrode it\n")
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// runDriver invokes the workflow driver agent until it returns a strictly
// validated JSON decision, then persists the validated decision for the
// record. Validation is state-aware: a "finish" in bootstrap state is
// rejected with the structural reason and the driver is asked again. The
// retry loop is unbounded.
func (o *Orchestrator) runDriver(st *WorkflowState, context *Context) *Decision {
	dd := runJSONDecision(o.driverAgent, o.buildDriverPrompt(st), decodeDriverDecisionFor(st), driverDecisionExample, context)

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

// completedExecutions counts the subworkflow executions recorded in the
// history - the same sections the artifact index renders (finish turns are
// not executions).
func completedExecutions(st *WorkflowState) int {
	n := 0
	for i := range st.History {
		if st.History[i].Subworkflow != "finish" {
			n++
		}
	}
	return n
}

// confirmFinishAllowed reports whether a claim of completion can be valid in
// the current state: a run with zero completed executions has not started,
// so it cannot be complete.
func confirmFinishAllowed(st *WorkflowState) bool {
	return completedExecutions(st) > 0
}

// decodeDriverDecisionFor returns the regular-turn decision validator for
// the current state: strict schema validation plus the structural rule that
// "finish" is impossible in bootstrap state. A rejected response is fed
// back to the driver with the reason and the driver is asked again, so the
// worst-case first-turn failure is corrected at the source instead of
// costing a reassessment round trip.
func decodeDriverDecisionFor(st *WorkflowState) func(string) (DriverDecision, error) {
	return func(jsonText string) (DriverDecision, error) {
		dd, err := decodeDriverDecision(jsonText)
		if err != nil {
			return dd, err
		}
		if dd.Subworkflow == "finish" && !confirmFinishAllowed(st) {
			return dd, fmt.Errorf("finish is impossible in BOOTSTRAP state (zero completed executions): the run has not started, so it cannot be complete. Respond with subworkflow \"spec\" (RULE 1)")
		}
		return dd, nil
	}
}

// decodeFinishReassessmentFor returns the reassessment validator for the
// current state: strict branch/combination validation plus the same
// structural rule applied to confirmations (defense in depth - a bootstrap
// finish is already rejected on the regular turn).
func decodeFinishReassessmentFor(st *WorkflowState) func(string) (FinishReassessment, error) {
	return func(jsonText string) (FinishReassessment, error) {
		fr, err := decodeFinishReassessment(jsonText)
		if err != nil {
			return fr, err
		}
		if fr.Decision == "confirm_finish" && !confirmFinishAllowed(st) {
			return fr, fmt.Errorf("confirm_finish is impossible in BOOTSTRAP state (zero completed executions): the run has not started, so it cannot be complete. Respond with decision \"select_subworkflow\" and subworkflow \"spec\"")
		}
		return fr, nil
	}
}

// buildFinishReassessmentPrompt extends the regular driver prompt with the
// driver's full finish response and the mandatory reassessment instruction.
// The response is a two-branch protocol: select the subworkflow that was
// actually needed, or explicitly confirm the finish.
func (o *Orchestrator) buildFinishReassessmentPrompt(st *WorkflowState, finish *Decision) string {
	var b strings.Builder
	b.WriteString(o.buildDriverPrompt(st))

	b.WriteString("<finish_reassessment>\n")
	b.WriteString(finishReassessmentMarker + "\n\n")
	b.WriteString("Your response for this driver turn was:\n\n")
	b.WriteString(prettyJSON(map[string]any{
		"subworkflow": finish.Choice,
		"rationale":   finish.Rationale,
		"task":        finish.Task,
	}) + "\n\n")
	b.WriteString("You selected \"finish\". The run will NOT terminate on that response alone. Reassess the situation and second-guess your own decision before the run can end.\n\n")
	b.WriteString("Re-read the artifact index and the decision history. Your response to this reassessment uses the \"decision\" field to state which branch you take:\n\n")
	b.WriteString("BRANCH 1 - select_subworkflow (the run is NOT complete):\n")
	b.WriteString("  decision = \"select_subworkflow\", subworkflow = the id of the subworkflow to run, task = its task.\n")
	b.WriteString("  Take this branch if anything is missing, unfinished, unreviewed, or uncertain. In BOOTSTRAP state (zero completed executions) the work has NOT started: this is the ONLY valid branch, and the subworkflow is \"spec\".\n")
	b.WriteString("  The subworkflow field MUST name the subworkflow you are selecting. Never set it to \"finish\" on this branch.\n\n")
	b.WriteString("BRANCH 2 - confirm_finish (the run IS complete):\n")
	b.WriteString("  decision = \"confirm_finish\", subworkflow = \"finish\".\n")
	b.WriteString("  Take this branch only if every condition of your FINISH GATE holds. The rationale MUST cite the specific completed artifacts in the index that prove the requested outcome was achieved. Restating your previous rationale is not a confirmation. In BOOTSTRAP state this branch is impossible.\n\n")
	b.WriteString("Hard rule: if your rationale says the finish is wrong, premature, invalid, or forbidden, your decision MUST be \"select_subworkflow\". A response that argues against finishing while declaring confirm_finish is invalid and will be rejected.\n\n")
	b.WriteString("Your response to this reassessment is final: a subworkflow selection is executed immediately, and a confirmed finish terminates the run.\n")
	b.WriteString("</finish_reassessment>\n")

	return b.String()
}

// reassessmentAgent returns the driver agent bound to the finish
// reassessment contract: the same role, but its Output section carries the
// reassessment schema and example, matching the schema enforced at
// generation time.
func (o *Orchestrator) reassessmentAgent() *Agent {
	schema := finishReassessmentSchema()
	a := NewAgent(
		o.driverAgent.Name,
		loader.RenderDecisionPrompt(loader.WORKFLOW_DRIVER_PROMPT_ID, schema, finishReassessmentExample),
		o.driverAgent.Timeout,
	)
	a.Schema = schema
	a.Inputs = o.driverAgent.Inputs
	a.Outputs = o.driverAgent.Outputs
	a.OutputTerminal = o.driverAgent.OutputTerminal
	return a
}

// runFinishReassessment sends the driver's full finish response back to it
// and asks it to reassess: confirm the finish or pick the appropriate
// subworkflow. The validated reassessment decision is persisted for the
// record; the returned decision is the final decision for the turn.
//
// The orchestrator enforces what it knows to be true: in BOOTSTRAP state
// (zero completed executions) a finish confirmation is structurally
// impossible - the run has not started, so it cannot be complete. Such a
// response is rejected and the driver is asked again.
func (o *Orchestrator) runFinishReassessment(st *WorkflowState, finish *Decision, context *Context) *Decision {
	fr := runJSONDecision(o.reassessmentAgent(), o.buildFinishReassessmentPrompt(st, finish), decodeFinishReassessmentFor(st), finishReassessmentExample, context)

	path := AbsPath(st.newReassessmentPath(o.subdir))
	ensureParentDir(path)
	AtomicWrite(path, prettyJSON(fr)+"\n")

	dec := &Decision{
		Choice:    fr.Subworkflow,
		Rationale: fr.Rationale,
		Task:      fr.Task,
		Path:      path,
	}
	confirmed := fr.Decision == "confirm_finish"
	context.Tracer.trace("driver_finish_reassessment", map[string]any{
		"iteration":         st.DriverIteration,
		"initial_choice":    finish.Choice,
		"initial_rationale": finish.Rationale,
		"initial_task":      finish.Task,
		"decision":          fr.Decision,
		"final_choice":      dec.Choice,
		"confirmed":         confirmed,
		"rationale":         dec.Rationale,
		"task":              dec.Task,
	})
	if confirmed {
		logStep(fmt.Sprintf("driver iteration %d: finish confirmed on reassessment", st.DriverIteration), "DRIVER")
	} else {
		logStep(fmt.Sprintf("driver iteration %d: finish overridden on reassessment -> %q", st.DriverIteration, dec.Choice), "DRIVER")
	}
	return dec
}
