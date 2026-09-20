package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Artifact is a single Markdown document produced by an agent, registered in
// the workflow state. Content lives on disk; the state only tracks location
// and provenance. Document content is never parsed or validated.
type Artifact struct {
	ID          string `json:"id"`
	Iteration   int    `json:"iteration"` // driver iteration (subworkflow execution) that produced it
	Subworkflow string `json:"subworkflow"`
	Agent       string `json:"agent"`
	Round       int    `json:"round"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

// HistoryEntry records one driver turn: what was decided and how it went.
type HistoryEntry struct {
	Iteration    int      `json:"iteration"`
	Subworkflow  string   `json:"subworkflow"`
	DecisionPath string   `json:"decision_path"`
	Rationale    string   `json:"rationale,omitempty"`
	Task         string   `json:"task,omitempty"`
	Outcome      string   `json:"outcome"`
	Artifacts    []string `json:"artifacts,omitempty"`
}

// ActiveExecution tracks the subworkflow that is currently running, so the
// artifact index can show its section (headed by the driver's task) before
// the execution is recorded in history. It is persisted with the state and
// cleared when the execution completes; on resume a leftover active execution
// is folded into history.
type ActiveExecution struct {
	Iteration   int    `json:"iteration"`
	Subworkflow string `json:"subworkflow"`
	Task        string `json:"task"`
}

// WorkflowState is the single authoritative state of a run. It is persisted to
// <subdir>/.state/workflow_state.json after every driver turn so a run can be
// resumed.
type WorkflowState struct {
	Task            string           `json:"task"`
	Seq             int              `json:"seq"`
	DriverIteration int              `json:"driver_iteration"`
	Artifacts       []Artifact       `json:"artifacts"`
	History         []HistoryEntry   `json:"history"`
	Active          *ActiveExecution `json:"active,omitempty"`
	Finished        bool             `json:"finished"`
	Outcome         string           `json:"outcome,omitempty"`
}

func stateFilePath(subdir string) string {
	return filepath.Join(subdir, ".state", "workflow_state.json")
}

func newWorkflowState(task, subdir string) *WorkflowState {
	return &WorkflowState{Task: task, Artifacts: []Artifact{}, History: []HistoryEntry{}}
}

func loadWorkflowState(subdir string) (*WorkflowState, error) {
	data, err := os.ReadFile(stateFilePath(subdir))
	if err != nil {
		return nil, fmt.Errorf("load workflow state: %w", err)
	}
	var st WorkflowState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parse workflow state %s: %w", stateFilePath(subdir), err)
	}
	if st.Task == "" {
		return nil, fmt.Errorf("workflow state %s has an empty task", stateFilePath(subdir))
	}
	if st.Artifacts == nil {
		st.Artifacts = []Artifact{}
	}
	if st.History == nil {
		st.History = []HistoryEntry{}
	}
	return &st, nil
}

func (st *WorkflowState) save(subdir string) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workflow state: %w", err)
	}
	AtomicWrite(stateFilePath(subdir), string(data))
	return nil
}

// NextSeq returns a fresh, strictly increasing sequence number for pregenerating
// unique file names and artifact ids.
func (st *WorkflowState) NextSeq() int {
	st.Seq++
	return st.Seq
}

func (st *WorkflowState) ArtifactByID(id string) (*Artifact, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for i := range st.Artifacts {
		if st.Artifacts[i].ID == id {
			return &st.Artifacts[i], true
		}
	}
	return nil, false
}

// AddArtifact registers a new artifact, enforcing id uniqueness.
func (st *WorkflowState) AddArtifact(a Artifact) {
	if _, exists := st.ArtifactByID(a.ID); exists {
		panic(fmt.Sprintf("duplicate artifact id %q", a.ID))
	}
	st.Artifacts = append(st.Artifacts, a)
}
