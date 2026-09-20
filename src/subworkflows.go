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
				schema := driverDecisionSchema()
				a = NewAgent(def.Name, loader.RenderDecisionPrompt(loader.WORKFLOW_DRIVER_PROMPT_ID, schema, driverDecisionExample), def.Timeout).WithSchema(schema)
			case "loop_decider":
				schema := loopDecisionSchema()
				a = NewAgent(def.Name, loader.RenderDecisionPrompt(loader.LOOP_DECIDER_PROMPT_ID, schema, loopDecisionExample), def.Timeout).WithSchema(schema)
			default:
				panic("unknown decision agent " + def.Name)
			}
		} else {
			a = NewAgent(def.Name, loader.GetPrompt(loader.PromptID(def.Name)), def.Timeout)
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

// runLoopDecider asks the loop decider agent whether the producer/reviewer
// loop should run another round. The decider is pointed at the artifact index
// and reads this execution's section itself. The decider's response is
// strictly validated JSON; the retry loop is unbounded.
func (o *Orchestrator) runLoopDecider(sw *Subworkflow, dec *Decision, st *WorkflowState, round int, context *Context) LoopDecision {
	var b strings.Builder
	b.WriteString(o.loopDeciderAgent.RolePrompt)
	b.WriteString("\n\n")

	b.WriteString("<task>\nORIGINAL TASK:\n" + wrapText(st.Task) + "\n</task>\n\n")

	b.WriteString("SUBWORKFLOW TASK (from the workflow driver):\n" + dec.Task + "\n\n")

	b.WriteString(fmt.Sprintf("SUBWORKFLOW: %s (%s)\nROUND JUST COMPLETED: %d\n\n", sw.ID, sw.Name, round))

	b.WriteString("ARTIFACT INDEX:\n")
	b.WriteString("The system maintains an index of every document produced so far at:\n")
	b.WriteString("  " + indexPath(o.subdir) + "\n")
	b.WriteString("Read the section for this subworkflow execution and study every document listed in it in full before deciding. The documents of the round that just completed are the last entries of that section.\n\n")

	return runJSONDecision(o.loopDeciderAgent, b.String(), decodeLoopDecision, loopDecisionExample, context)
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
