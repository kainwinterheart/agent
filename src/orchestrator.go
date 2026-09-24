package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agent-go/wman"
	"github.com/datadog/zstd"
)

// Orchestrator is the top-level controller: it runs the workflow driver in a
// loop, dispatches the subworkflow the driver picks, and persists state after
// every driver turn.
type Orchestrator struct {
	driverAgent      *Agent
	loopDeciderAgent *Agent
	Watcher          *wman.Watchman
	subdir           string

	// contextFactory creates the per-turn service context; tests replace it to
	// inject mock seams.
	contextFactory func() *Context
}

// NewOrchestrator prepares a run rooted at subdir. The subworkflow registry and
// DriverAgent must already be initialised (initSubworkflows). The filesystem
// watcher is started only when the wman script is available; without it the
// automated change-detection block is simply omitted.
func NewOrchestrator(subdir string) *Orchestrator {
	o := &Orchestrator{subdir: subdir, contextFactory: NewContext}
	o.driverAgent = DriverAgent
	o.loopDeciderAgent = LoopDeciderAgent
	if wmanAvailable() {
		o.Watcher = wman.NewWatchman(subdir)
		o.Watcher.Start()
	} else {
		logStep("watchman script not found; automated change detection disabled", "ORCHESTRATOR")
	}
	return o
}

// wmanAvailable reports whether the filesystem watcher script can be started.
func wmanAvailable() bool {
	wmanPath, _ := filepath.Abs(filepath.Join("wman", "wman.sh"))
	if envPath := os.Getenv("AC_WMAN_PATH"); envPath != "" {
		wmanPath = envPath
	}
	if _, err := os.Stat(wmanPath); err != nil {
		return false
	}
	if _, err := exec.LookPath("sh"); err != nil {
		return false
	}
	return true
}

// Run executes the dynamic workflow until the driver decides to finish. All
// loops are unbounded: the driver is consulted again after every subworkflow,
// and each subworkflow's review loop repeats as long as its loop decider
// orders another round.
func (o *Orchestrator) Run(task string, subdir string) error {
	var st *WorkflowState
	var err error
	if task == "" {
		st, err = loadWorkflowState(subdir)
		if err != nil {
			return err
		}
	} else {
		st = newWorkflowState(task, subdir)
	}
	// Fold an interrupted subworkflow execution (crash mid-run) back into the
	// history so the artifact index stays complete after a resume.
	if st.Active != nil {
		var ids []string
		for i := range st.Artifacts {
			if st.Artifacts[i].Iteration == st.Active.Iteration {
				ids = append(ids, st.Artifacts[i].ID)
			}
		}
		st.History = append(st.History, HistoryEntry{
			Iteration:   st.Active.Iteration,
			Subworkflow: st.Active.Subworkflow,
			Task:        st.Active.Task,
			Outcome:     "interrupted (resumed)",
			Artifacts:   ids,
		})
		st.Active = nil
		logStep(fmt.Sprintf("resumed: folded interrupted execution %d (%s) into history", st.History[len(st.History)-1].Iteration, st.History[len(st.History)-1].Subworkflow), "ORCHESTRATOR")
	}
	if st.Finished {
		logStep("run already finished: "+st.Outcome, "ORCHESTRATOR")
		return nil
	}
	if err := st.save(subdir); err != nil {
		return err
	}
	writeIndex(st, subdir)

	// Record the static wiring that governs this run in the session trace.
	AtomicWrite(filepath.Join(subdir, ".state", "static_definitions.md"), StaticDefinitionsDocument())

	var traceWriter *zstd.Writer
	if dest := os.Getenv("AGENT_TRACE_FILE"); dest != "" {
		f, ferr := os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if ferr != nil {
			return ferr
		}
		defer f.Close()
		traceWriter = zstd.NewWriterLevel(f, zstd.BestCompression)
		defer traceWriter.Close()
	}

	for !st.Finished {
		st.DriverIteration++
		context := o.contextFactory()

		dec, turn := o.runDriver(st, context)

		// A "finish" response never terminates the run by itself: the
		// driver's full response is sent back to it - reusing the session
		// acquired by this turn's read-inputs call - for a mandatory
		// reassessment, which either confirms the finish or picks the
		// subworkflow that was actually needed.
		var finishDecision *Decision
		if dec.Choice == "finish" {
			finishDecision = dec
			dec = o.runFinishReassessment(st, turn, finishDecision)
		}

		var outcome string
		var produced []Artifact
		if dec.Choice == "finish" {
			st.Finished = true
			st.Outcome = dec.Rationale
			outcome = "finished"
		} else {
			sw := getSubworkflow(dec.Choice)
			outcome, produced = o.runSubworkflow(sw, dec, st, context)
		}

		entry := HistoryEntry{
			Iteration:    st.DriverIteration,
			Subworkflow:  dec.Choice,
			DecisionPath: dec.Path,
			Rationale:    dec.Rationale,
			Task:         dec.Task,
			Outcome:      outcome,
		}
		for i := range produced {
			entry.Artifacts = append(entry.Artifacts, produced[i].ID)
		}
		if finishDecision != nil {
			entry.FinishDecisionPath = finishDecision.Path
			entry.FinishConfirmed = dec.Choice == "finish"
		}
		st.History = append(st.History, entry)

		if err := st.save(subdir); err != nil {
			return err
		}
		writeIndex(st, subdir)
		logStep(fmt.Sprintf("iteration %d: %s -> %s", st.DriverIteration, dec.Choice, outcome), "ORCHESTRATOR")
		context.Tracer.trace("iteration_done", map[string]any{
			"iteration": st.DriverIteration, "subworkflow": dec.Choice, "outcome": outcome,
		})
		if traceWriter != nil {
			writeEvents(traceWriter, context.Tracer.Events)
		}
	}

	if err := o.writeSummary(st); err != nil {
		return err
	}
	logStep("run finished: "+st.Outcome, "ORCHESTRATOR")
	return nil
}

// writeSummary renders the human entry point for the session: the driver's final
// outcome plus the full artifact index.
func (o *Orchestrator) writeSummary(st *WorkflowState) error {
	var b strings.Builder
	b.WriteString("# Run Summary\n\n")
	b.WriteString("## Task\n\n")
	b.WriteString(st.Task)
	b.WriteString("\n\n")
	b.WriteString("## Outcome\n\n")
	b.WriteString(st.Outcome)
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Driver iterations used: %d\n\n", st.DriverIteration))
	b.WriteString("## Artifacts\n\n")
	if len(st.Artifacts) == 0 {
		b.WriteString("None.\n")
	}
	for i := range st.Artifacts {
		a := &st.Artifacts[i]
		b.WriteString(fmt.Sprintf("- `%s` — %s\n", a.Path, a.Description))
	}
	b.WriteString("\n## Decision History\n\n")
	for i := range st.History {
		h := &st.History[i]
		b.WriteString(fmt.Sprintf("%d. `%s` -> %s\n", h.Iteration, h.Subworkflow, h.Outcome))
	}
	AtomicWrite(o.subdir+"/SUMMARY.md", b.String())
	return nil
}
