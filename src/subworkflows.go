package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-go/pkg/loader"
)

// SubworkflowStep is one agent position inside a subworkflow. The agent's
// static definition documents the input types its role is expected to read;
// at runtime the agent receives the artifact index path and selects the
// documents to study itself.
type SubworkflowStep struct {
	Agent        *Agent
	ArtifactKind string // base "what this file is" description of the produced document
	Reviewer     bool   // reviewer steps feed the loop decider
}

// Subworkflow is a short, independent sequence of closely tied agents: a producer
// and, optionally, its reviewers. It is the unit the workflow driver picks.
type Subworkflow struct {
	ID          string
	Name        string
	Description string // what it does, shown to the driver
	Produces    string // what documents it produces, shown to the driver
	Steps       []SubworkflowStep
}

func (sw *Subworkflow) HasReviewers() bool {
	for i := range sw.Steps {
		if sw.Steps[i].Reviewer {
			return true
		}
	}
	return false
}

// Subworkflows is the registry of all subworkflows the driver may pick.
var Subworkflows []Subworkflow

// DriverAgent is the single workflow-driver decision agent.
var DriverAgent *Agent

// LoopDeciderAgent is the decision agent that reads a subworkflow's round
// documents and decides whether the producer/reviewer loop runs again.
var LoopDeciderAgent *Agent

// initSubworkflows builds the runtime agents and subworkflows from the
// static definitions (AgentDefinitions, SubworkflowDefinitions). Must run
// after loader.InitPrompts().
func initSubworkflows() {
	agents := make(map[string]*Agent, len(AgentDefinitions))
	for i := range AgentDefinitions {
		def := &AgentDefinitions[i]
		var a *Agent
		if def.Decision {
			switch def.Name {
			case "workflow_driver":
				// Placeholder without a decision contract: the driver's
				// schema enumerates the subworkflow registry, which is
				// populated below. The agent is finalized after Subworkflows
				// is set - building the schema earlier would capture an
				// empty registry and constrain the driver to "finish".
				a = NewAgent(def.Name, loader.GetPrompt(loader.WORKFLOW_DRIVER_PROMPT_ID))
			case "loop_decider":
				schema := loopDecisionSchema()
				a = NewAgent(def.Name, loader.RenderDecisionPrompt(loader.LOOP_DECIDER_PROMPT_ID, schema, loopDecisionExample)).WithSchema(schema)
			default:
				panic("unknown decision agent " + def.Name)
			}
		} else {
			a = NewAgent(def.Name, loader.GetPrompt(loader.PromptID(def.Name)))
		}
		a.Inputs = def.Inputs
		a.Outputs = def.Outputs
		a.OutputTerminal = def.OutputTerminal
		agents[def.Name] = a
	}

	subworkflows := make([]Subworkflow, 0, len(SubworkflowDefinitions))
	for i := range SubworkflowDefinitions {
		def := &SubworkflowDefinitions[i]
		steps := make([]SubworkflowStep, 0, len(def.Steps))
		for _, sd := range def.Steps {
			a, ok := agents[sd.Agent]
			if !ok {
				panic("subworkflow " + def.ID + " references unknown agent " + sd.Agent)
			}
			steps = append(steps, SubworkflowStep{Agent: a, ArtifactKind: sd.ArtifactKind, Reviewer: sd.Reviewer})
		}
		subworkflows = append(subworkflows, Subworkflow{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Produces:    def.Produces,
			Steps:       steps,
		})
	}
	Subworkflows = subworkflows

	// The subworkflow registry is now populated: finalize the driver agent
	// with its decision contract - the schema that enumerates every
	// subworkflow id plus "finish", and the role prompt rendered against
	// that schema.
	placeholder := agents["workflow_driver"]
	driverSchema := driverDecisionSchema()
	driver := NewAgent(placeholder.Name, loader.RenderDecisionPrompt(loader.WORKFLOW_DRIVER_PROMPT_ID, driverSchema, driverDecisionExample)).WithSchema(driverSchema)
	driver.Inputs = placeholder.Inputs
	driver.Outputs = placeholder.Outputs
	driver.OutputTerminal = placeholder.OutputTerminal
	agents["workflow_driver"] = driver

	DriverAgent = agents["workflow_driver"]
	LoopDeciderAgent = agents["loop_decider"]
}

// getSubworkflow looks up a subworkflow by id (case-insensitive).
func getSubworkflow(id string) *Subworkflow {
	id = strings.ToLower(strings.TrimSpace(id))
	for i := range Subworkflows {
		if Subworkflows[i].ID == id {
			return &Subworkflows[i]
		}
	}
	return nil
}

func subworkflowIDs() []string {
	ids := make([]string, 0, len(Subworkflows))
	for i := range Subworkflows {
		ids = append(ids, Subworkflows[i].ID)
	}
	return ids
}

// runSubworkflow executes one subworkflow: rounds of its steps, gated by the
// loop decider. The round loop is unbounded — it repeats as long as the
// decider, after reading the round's documents, decides to repeat. Returns the
// outcome and the artifacts produced.
func (o *Orchestrator) runSubworkflow(sw *Subworkflow, dec *Decision, st *WorkflowState, context *Context) (string, []Artifact) {
	var produced []Artifact
	iteration := st.DriverIteration

	// Mark the execution active so the artifact index shows its section
	// (headed by the driver's task) while it runs; persist so a crash mid-run
	// leaves a resumable state. Cleared when the execution completes.
	st.Active = &ActiveExecution{Iteration: iteration, Subworkflow: sw.ID, Task: dec.Task}
	if err := st.save(o.subdir); err != nil {
		panic("persist active execution: " + err.Error())
	}
	writeIndex(st, o.subdir)
	defer func() { st.Active = nil }()

	for round := 1; ; round++ {
		var roundReviews []Artifact
		autoChangesBlock := ""

		for si := range sw.Steps {
			step := &sw.Steps[si]
			id, relPath := st.newArtifactPath(o.subdir, sw.ID, step.Agent.Name, round)
			outPath := AbsPath(relPath)
			ensureParentDir(outPath)

			prompt := o.buildStepPrompt(sw, step, st, dec, outPath, round, autoChangesBlock)
			o.runAgentUntilComplete(step.Agent, prompt, outPath, context)

			art := Artifact{
				ID:          id,
				Iteration:   iteration,
				Subworkflow: sw.ID,
				Agent:       step.Agent.Name,
				Round:       round,
				Description: step.ArtifactKind,
				Path:        outPath,
			}
			st.AddArtifact(art)
			if err := st.save(o.subdir); err != nil {
				panic("persist artifact: " + err.Error())
			}
			writeIndex(st, o.subdir)
			produced = append(produced, art)
			if step.Reviewer {
				roundReviews = append(roundReviews, art)
			}

			// After the coder finishes, collect the real disk changes for the
			// code review's automated verification block.
			if sw.ID == "implement" && step.Agent.Name == "coder" {
				if o.Watcher != nil || context.watchmanHook != nil {
					autoChangesBlock = changesPrompt(safeFlush(o.Watcher, context))
				}
			}
		}

		if !sw.HasReviewers() {
			return "completed", produced
		}

		// The loop decider reads this execution's section of the artifact
		// index and decides whether another round is warranted.
		ld := o.runLoopDecider(sw, dec, st, round, context)
		if !ld.Repeat {
			return fmt.Sprintf("completed: loop stopped after %d round(s); decider: %s", round, truncate(ld.Reason, 300)), produced
		}
	}
}

// The loop decider's phase markers head the three phase instructions of
// its multi-turn invocation. The mock runtime and the tests key off them
// to identify phase calls; they are deliberately distinct from any section
// header in the role prompt.
const (
	loopDeciderReadMarker    = "LOOP DECIDER PHASE 1 OF 3: READ INPUTS"
	loopDeciderAnalyzeMarker = "LOOP DECIDER PHASE 2 OF 3: ANALYZE AND SAVE"
	loopDeciderRespondMarker = "LOOP DECIDER PHASE 3 OF 3: RESPOND"
)

// buildLoopDeciderBundle assembles the loop decider's full state bundle:
// role, task, subworkflow task, the round that just completed, and the
// artifact index pointer. It is sent in full on the read-inputs call (which
// starts the turn's session) and, in fallback mode, is prepended to every
// later phase call so each call is self-contained.
func (o *Orchestrator) buildLoopDeciderBundle(sw *Subworkflow, dec *Decision, st *WorkflowState, round int) string {
	var b strings.Builder
	b.WriteString(o.loopDeciderAgent.RolePrompt)
	b.WriteString("\n\n")

	b.WriteString("<task>\nORIGINAL TASK:\n" + wrapText(st.Task) + "\n</task>\n\n")

	b.WriteString("SUBWORKFLOW TASK (from the workflow driver):\n" + dec.Task + "\n\n")

	b.WriteString(fmt.Sprintf("SUBWORKFLOW: %s (%s)\nROUND JUST COMPLETED: %d\n\n", sw.ID, sw.Name, round))

	b.WriteString("ARTIFACT INDEX:\n")
	b.WriteString("The system maintains an index of every document produced so far at:\n")
	b.WriteString("  " + indexPath(o.subdir) + "\n")
	b.WriteString(fmt.Sprintf("The section for this subworkflow execution is the one headed \"Iteration %d — subworkflow: %s\". The documents of the round that just completed are the last entries of that section.\n\n", st.DriverIteration, sw.ID))

	return b.String()
}

// runLoopDecider asks the loop decider agent whether the producer/reviewer
// loop should run another round, as a multi-turn conversation: a fresh
// session reads this execution's index section and the round's documents;
// the same session then analyzes them and saves the analysis to a file; and
// only then returns the strictly validated JSON verdict. There is no
// reassessment for the decider. See agentTurn.
func (o *Orchestrator) runLoopDecider(sw *Subworkflow, dec *Decision, st *WorkflowState, round int, context *Context) LoopDecision {
	turn := newAgentTurn(o, o.loopDeciderAgent, context, o.buildLoopDeciderBundle(sw, dec, st, round))
	turn.readInputs(buildDeciderReadPhaseBlock())

	analysisPath := AbsPath(st.newAnalysisPath(o.subdir, "loop_decider"))
	turn.analyzeAndSave(analysisPath, buildDeciderAnalyzePhaseBlock(analysisPath))

	schema := loopDecisionSchema()
	return agentDecide(turn, "respond", buildDeciderRespondPhaseBlock(analysisPath, prettyJSON(schema), loopDecisionExample), schema, loopDecisionExample, decodeLoopDecision)
}

// buildDeciderReadPhaseBlock is the phase-1 instruction appended to the
// full decider bundle on the read-inputs call: the model reads the
// execution's documents, emits no verdict, and warms the session the rest
// of the turn resumes.
func buildDeciderReadPhaseBlock() string {
	var b strings.Builder
	b.WriteString("<loop_decider_read_inputs>\n")
	b.WriteString(loopDeciderReadMarker + "\n\n")
	b.WriteString("This is the first call of a three-call decider turn. In this call you MUST NOT produce a JSON verdict: the Output contract of your role applies only to the third call. Your sole job now is to read all the inputs of this turn:\n\n")
	b.WriteString("1. Read the artifact index at the path stated above.\n")
	b.WriteString("2. Study in full every document listed in this execution's section, with particular attention to the documents of the round that just completed (the last entries of the section).\n")
	b.WriteString("3. Reply with a short prose confirmation (no JSON): the documents you read and the round's state as you see it.\n\n")
	b.WriteString("The next two calls happen in this same session: you will then analyze the data and save your reasoning to a file, and only afterwards produce the JSON verdict.\n")
	b.WriteString("</loop_decider_read_inputs>\n")
	return b.String()
}

// buildDeciderAnalyzePhaseBlock is the phase-2 instruction: analyze with
// the intention of producing the verdict, and save the analysis and
// reasoning to the pregenerated file.
func buildDeciderAnalyzePhaseBlock(analysisPath string) string {
	var b strings.Builder
	b.WriteString("<loop_decider_analyze>\n")
	b.WriteString(loopDeciderAnalyzeMarker + "\n\n")
	b.WriteString("Using everything you read in the previous call, now:\n\n")
	b.WriteString("1. Analyze the data with the intention of producing your JSON verdict: walk your DECISION ORDER step by step against the actual evidence (the round's producer artifact, the reviewer verdicts, and the prior rounds when this is a revision). Do NOT emit the JSON verdict in this call.\n")
	b.WriteString("2. Save your complete analysis and reasoning to this exact path:\n")
	b.WriteString("  " + analysisPath + "\n")
	b.WriteString("The file MUST exist and be non-empty when you finish. It must contain: what you read; the key facts you rely on (verdicts, findings, evidence locations); each decision-order step you evaluated and its outcome; and the repeat decision you intend to emit with the reason that forces it.\n")
	b.WriteString("</loop_decider_analyze>\n")
	return b.String()
}

// buildDeciderRespondPhaseBlock is the phase-3 instruction: return the
// JSON-structured verdict the analysis concluded, under the verdict schema
// enforced at generation time.
func buildDeciderRespondPhaseBlock(analysisPath, schemaText, example string) string {
	var b strings.Builder
	b.WriteString("<loop_decider_respond>\n")
	b.WriteString(loopDeciderRespondMarker + "\n\n")
	b.WriteString("Your analysis is saved at:\n  " + analysisPath + "\n\n")
	b.WriteString("Now produce the JSON verdict your analysis concluded. The verdict MUST be forced by that analysis - do not invent a new one now.\n\n")
	b.WriteString("Output MUST be valid JSON only, conforming to this schema:\n")
	b.WriteString(schemaText + "\n\n")
	b.WriteString("Example of a valid response:\n")
	b.WriteString(example + "\n")
	b.WriteString("</loop_decider_respond>\n")
	return b.String()
}

// buildStepPrompt assembles the full prompt for one agent step: role, task,
// subworkflow task, the artifact index path (the agent selects which documents
// to study), optional automated verification block, and the output requirement
// with the pregenerated path.
func (o *Orchestrator) buildStepPrompt(sw *Subworkflow, step *SubworkflowStep, st *WorkflowState, dec *Decision, outPath string, round int, autoChangesBlock string) string {
	var b strings.Builder
	b.WriteString(step.Agent.RolePrompt)
	b.WriteString("\n\n")

	b.WriteString("<task>\nORIGINAL TASK:\n" + wrapText(st.Task) + "\n</task>\n\n")

	b.WriteString("SUBWORKFLOW TASK (from the workflow driver):\n" + dec.Task + "\n\n")

	if round > 1 {
		b.WriteString(fmt.Sprintf("NOTE: This is revision round %d of the subworkflow. Earlier rounds were reviewed and the loop decider ordered another round; the previous round's review documents are in this execution's section of the artifact index and explain what must change.\n\n", round))
	}

	b.WriteString("ARTIFACT INDEX:\n")
	b.WriteString("The system maintains an index of every document produced so far at:\n")
	b.WriteString("  " + indexPath(o.subdir) + "\n")
	b.WriteString("Each section describes one subworkflow execution: the task given to it and the documents its agents produced. Read the index, identify the documents your role and the subworkflow task require, and study each of them in full before producing your output.\n\n")

	if autoChangesBlock != "" {
		b.WriteString(autoChangesBlock + "\n\n")
	}

	b.WriteString("OUTPUT REQUIREMENTS:\n")
	b.WriteString("* Write your final output as a Markdown document to this exact path:\n")
	b.WriteString("  " + outPath + "\n")
	b.WriteString("* The file MUST exist and be non-empty when you finish.\n")
	b.WriteString("* The document MUST follow the predefined structure from your role, section by section.\n")
	return b.String()
}

// runAgentUntilComplete invokes a content agent until its output file passes
// validation (exists, non-empty — nothing about the content is checked),
// feeding the validation problem back as retry feedback. The loop is
// unbounded.
func (o *Orchestrator) runAgentUntilComplete(agent *Agent, prompt, outPath string, context *Context) {
	cur := prompt
	attempt := 0
	for {
		attempt++
		context.Tracer.trace("agent_invoke", map[string]any{
			"agent": agent.Name, "attempt": attempt, "output_path": outPath, "prompt": cur,
		})

		var problem string
		if _, runErr := agent.Run(cur, context); runErr != nil {
			problem = "the agent run itself failed: " + runErr.Error()
		} else if verr := validateOutputFile(outPath); verr != nil {
			problem = verr.Error()
		} else {
			return
		}

		logStep(fmt.Sprintf("%s attempt %d incomplete: %s", agent.Name, attempt, problem), "AGENT")
		context.Tracer.trace("agent_retry", map[string]any{"agent": agent.Name, "attempt": attempt, "problem": problem})
		cur = prompt + feedbackBlock(problem)
	}
}

// validateOutputFile is the entire validation of a content agent's deliverable:
// the file must exist and be non-empty. Content and structure are not checked.
func validateOutputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("output file %s does not exist", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("output file %s is empty", path)
	}
	return nil
}

// ensureParentDir creates the parent directory of a pregenerated output path so
// the agent can write its document without knowing the layout in advance.
func ensureParentDir(path string) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			panic(fmt.Sprintf("ensure_parent_dir: %v", err))
		}
	}
}

func feedbackBlock(problem string) string {
	return fmt.Sprintf("\n\n<feedback>\nYOUR PREVIOUS ATTEMPT WAS INCOMPLETE.\nProblem: %s\n\nFix the problem: produce the complete Markdown document and write it to the exact output path given under OUTPUT REQUIREMENTS. The file must exist and be non-empty when you finish.\n</feedback>\n", problem)
}
