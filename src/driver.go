package main

import (
	"fmt"
	"strings"
)

// finishReassessmentMarker heads the mandatory reassessment block that is
// appended to the driver prompt whenever the driver responds with "finish";
// the session trace and the tests key off it to identify reassessment turns.
// It is intentionally distinct from the role prompt's "FINISH REASSESSMENT"
// section header so reassessment prompts can be told apart from regular ones.
const finishReassessmentMarker = "FINISH REASSESSMENT - MANDATORY"

// The phase markers head the three phase instructions of the driver's
// multi-turn invocation. The mock runtime and the tests key off them to
// identify phase calls; they are deliberately distinct from any section
// header in the role prompt.
const (
	driverReadMarker    = "DRIVER PHASE 1 OF 3: READ INPUTS"
	driverAnalyzeMarker = "DRIVER PHASE 2 OF 3: ANALYZE AND SAVE"
	driverRespondMarker = "DRIVER PHASE 3 OF 3: RESPOND"
)

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

// buildDriverPrompt assembles the driver's full state bundle: role, task,
// the full list of available subworkflows, the artifact index path, and the
// driver's own decision history. It is sent in full on the read-inputs call
// (which starts the turn's session) and, in fallback mode, is prepended to
// every later phase call so each call is self-contained.
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

// runDriver executes one driver turn as a multi-turn conversation:
//
//  1. a model call in a fresh session, from clean context, reads all the
//     inputs and yields the session id (kept in memory only);
//  2. a model call resuming that session analyzes the data with the
//     intention of producing the decision and saves its analysis and
//     reasoning to a file;
//  3. a model call resuming that session returns the JSON-structured
//     decision, strictly validated.
//
// The validated decision is persisted for the record. The returned
// agentTurn carries the in-memory session so the mandatory finish
// reassessment can reuse it when the decision is "finish".
func (o *Orchestrator) runDriver(st *WorkflowState, context *Context) (*Decision, *agentTurn) {
	dt := newAgentTurn(o, o.driverAgent, context, o.buildDriverPrompt(st))
	dt.readInputs(buildDriverReadPhaseBlock())

	analysisPath := AbsPath(st.newAnalysisPath(o.subdir, "driver"))
	dt.analyzeAndSave(analysisPath, buildDriverAnalyzePhaseBlock(analysisPath))

	schema := driverDecisionSchema()
	dd := agentDecide(dt, "respond", buildDriverRespondPhaseBlock(analysisPath, prettyJSON(schema), driverDecisionExample), schema, driverDecisionExample, decodeDriverDecisionFor(st))

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
		"iteration":     st.DriverIteration,
		"choice":        dec.Choice,
		"rationale":     dec.Rationale,
		"task":          dec.Task,
		"analysis_path": analysisPath,
	})
	logStep(fmt.Sprintf("driver iteration %d: chose %q", st.DriverIteration, dec.Choice), "DRIVER")
	return dec, dt
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

// buildDriverReadPhaseBlock is the phase-1 instruction appended to the
// full driver prompt on the read-inputs call: the model reads everything,
// emits no decision, and warms the session the rest of the turn resumes.
func buildDriverReadPhaseBlock() string {
	var b strings.Builder
	b.WriteString("<driver_read_inputs>\n")
	b.WriteString(driverReadMarker + "\n\n")
	b.WriteString("This is the first call of a three-call driver turn. In this call you MUST NOT produce a JSON decision: the Output contract of your role applies only to the third call. Your sole job now is to read all the inputs of this turn:\n\n")
	b.WriteString("1. Read the artifact index at the path stated above.\n")
	b.WriteString("2. Study in full every document the index lists that your MANDATORY DECISION PROCEDURE requires you to consider for this turn.\n")
	b.WriteString("3. Reply with a short prose confirmation (no JSON): the documents you read and the run state as you see it.\n\n")
	b.WriteString("The next two calls happen in this same session: you will then analyze the data and save your reasoning to a file, and only afterwards produce the JSON decision.\n")
	b.WriteString("</driver_read_inputs>\n")
	return b.String()
}

// buildDriverAnalyzePhaseBlock is the phase-2 instruction: analyze with
// the intention of producing the decision, and save the analysis and
// reasoning to the pregenerated file.
func buildDriverAnalyzePhaseBlock(analysisPath string) string {
	var b strings.Builder
	b.WriteString("<driver_analyze>\n")
	b.WriteString(driverAnalyzeMarker + "\n\n")
	b.WriteString("Using everything you read in the previous call, now:\n\n")
	b.WriteString("1. Analyze the data with the intention of producing your JSON decision: walk your MANDATORY DECISION PROCEDURE step by step against the actual state (run state, artifact index contents, decision history). Do NOT emit the JSON decision in this call.\n")
	b.WriteString("2. Save your complete analysis and reasoning to this exact path:\n")
	b.WriteString("  " + analysisPath + "\n")
	b.WriteString("The file MUST exist and be non-empty when you finish. It must contain: what you read; the key facts you rely on; each decision-procedure step you evaluated and its outcome; and the transition you intend to emit (subworkflow id or finish) with the rationale that forces it.\n")
	b.WriteString("</driver_analyze>\n")
	return b.String()
}

// buildDriverRespondPhaseBlock is the phase-3 instruction: return the
// JSON-structured response the analysis concluded, under the decision
// schema enforced at generation time.
func buildDriverRespondPhaseBlock(analysisPath, schemaText, example string) string {
	var b strings.Builder
	b.WriteString("<driver_respond>\n")
	b.WriteString(driverRespondMarker + "\n\n")
	b.WriteString("Your analysis is saved at:\n  " + analysisPath + "\n\n")
	b.WriteString("Now produce the JSON decision your analysis concluded. The decision MUST be forced by that analysis - do not invent a new transition now.\n\n")
	b.WriteString("Output MUST be valid JSON only, conforming to this schema:\n")
	b.WriteString(schemaText + "\n\n")
	b.WriteString("Example of a valid response:\n")
	b.WriteString(example + "\n")
	b.WriteString("</driver_respond>\n")
	return b.String()
}

// buildFinishReassessmentBlock assembles the mandatory reassessment
// instruction. It is self-contained: it carries the driver's full finish
// response, the two-branch protocol, and the reassessment schema + example
// (the contract shown to the model equals the contract enforced at
// generation time). In session mode the role prompt and the state bundle
// are already in the conversation from the turn's read-inputs call; in
// fallback mode the full driver prompt is prepended by agentTurn.invoke.
func buildFinishReassessmentBlock(finish *Decision) string {
	var b strings.Builder
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
	b.WriteString("Your response to this reassessment is final: a subworkflow selection is executed immediately, and a confirmed finish terminates the run.\n\n")
	b.WriteString("Output MUST be valid JSON only, conforming to this schema:\n")
	b.WriteString(prettyJSON(finishReassessmentSchema()) + "\n\n")
	b.WriteString("Example of a valid response:\n")
	b.WriteString(finishReassessmentExample + "\n")
	b.WriteString("</finish_reassessment>\n")
	return b.String()
}

// runFinishReassessment is the single mandatory reassessment of a "finish"
// decision: it reuses the session acquired by the turn's read-inputs call,
// so the driver second-guesses its own decision with everything it read
// and analyzed still in context. The validated reassessment decision is
// persisted for the record; the returned decision is the final decision
// for the turn.
//
// The orchestrator enforces what it knows to be true: in BOOTSTRAP state
// (zero completed executions) a finish confirmation is structurally
// impossible - the run has not started, so it cannot be complete. Such a
// response is rejected and the driver is asked again.
func (o *Orchestrator) runFinishReassessment(st *WorkflowState, dt *agentTurn, finish *Decision) *Decision {
	fr := agentDecide(dt, "reassess", buildFinishReassessmentBlock(finish), finishReassessmentSchema(), finishReassessmentExample, decodeFinishReassessmentFor(st))

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
	dt.context.Tracer.trace("driver_finish_reassessment", map[string]any{
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
