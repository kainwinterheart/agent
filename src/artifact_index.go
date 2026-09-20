package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// The artifact index is the single view of past work that agents receive. It
// is maintained exclusively by the orchestrator — never by an LLM — as a pure
// function of the workflow state, and is atomically rewritten after every
// state mutation. One section per subworkflow execution, each headed by the
// driver's task for that execution, lists every document produced within it.
// Agents are handed only the index path and select the documents to read
// themselves.

// indexPath is the absolute location of the artifact index inside the session
// directory.
func indexPath(subdir string) string {
	return AbsPath(filepath.Join(subdir, "artifacts", "INDEX.md"))
}

// renderIndex renders the artifact index from the workflow state. Completed
// executions come from the history; an in-progress execution (st.Active)
// renders its section from the artifacts already produced in its iteration.
func renderIndex(st *WorkflowState) string {
	var b strings.Builder
	b.WriteString("# Artifact Index\n\n")
	b.WriteString("This index is maintained by the system, not by any agent. Each section describes one subworkflow execution: the task the workflow driver gave it, and the documents its agents produced. Use it to locate the documents you need.\n\n")

	sections := 0
	appendSection := func(iteration int, swID, task string) {
		var docs []Artifact
		for i := range st.Artifacts {
			if st.Artifacts[i].Iteration == iteration {
				docs = append(docs, st.Artifacts[i])
			}
		}
		b.WriteString(fmt.Sprintf("## Iteration %d — subworkflow: %s\n\n", iteration, swID))
		b.WriteString("Task: " + task + "\n\n")
		b.WriteString("Documents:\n")
		if len(docs) == 0 {
			b.WriteString("- (none yet)\n")
		}
		for i := range docs {
			a := &docs[i]
			b.WriteString(fmt.Sprintf("- %s — %s (agent: %s, round %d)\n", a.Path, a.Description, a.Agent, a.Round))
		}
		b.WriteString("\n")
		sections++
	}

	for i := range st.History {
		h := &st.History[i]
		if h.Subworkflow == "finish" {
			continue
		}
		appendSection(h.Iteration, h.Subworkflow, h.Task)
	}
	if st.Active != nil {
		appendSection(st.Active.Iteration, st.Active.Subworkflow, st.Active.Task)
	}

	if sections == 0 {
		b.WriteString("No subworkflow executions yet.\n")
	}
	return b.String()
}

// writeIndex atomically rewrites the artifact index from the current state.
func writeIndex(st *WorkflowState, subdir string) {
	ensureParentDir(indexPath(subdir))
	AtomicWrite(indexPath(subdir), renderIndex(st))
}
