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
