package main

import (
	"fmt"
	"path/filepath"
)

// newArtifactPath pregenerates a unique artifact id and output file path for an
// agent document. The path is handed to the agent in its prompt before the agent
// runs.
func (st *WorkflowState) newArtifactPath(subdir, subworkflowID, agent string, round int) (id, path string) {
	seq := st.NextSeq()
	id = fmt.Sprintf("%03d-%s", seq, agent)
	name := fmt.Sprintf("%03d-%s_%s", seq, subworkflowID, agent)
	if round > 1 {
		name += fmt.Sprintf("_r%d", round)
	}
	path = filepath.Join(subdir, "artifacts", name+".md")
	return id, path
}

// newDecisionPath pregenerates a unique path for the driver's decision document.
func (st *WorkflowState) newDecisionPath(subdir string) string {
	seq := st.NextSeq()
	return filepath.Join(subdir, "decisions", fmt.Sprintf("%03d-driver.md", seq))
}

// newReassessmentPath pregenerates a unique path for the driver's finish
// reassessment decision document.
func (st *WorkflowState) newReassessmentPath(subdir string) string {
	seq := st.NextSeq()
	return filepath.Join(subdir, "decisions", fmt.Sprintf("%03d-driver_finish_reassessment.md", seq))
}

// newAnalysisPath pregenerates a unique path for a multi-turn agent's
// per-turn analysis document - the file the agent saves its analysis and
// reasoning to in the analyze phase of its invocation (label: "driver" for
// the workflow driver, "loop_decider" for the loop decider). Like the
// decision documents it is a control-plane artifact: it is never registered
// in the artifact index.
func (st *WorkflowState) newAnalysisPath(subdir, label string) string {
	seq := st.NextSeq()
	return filepath.Join(subdir, "analysis", fmt.Sprintf("%03d-%s.md", seq, label))
}
