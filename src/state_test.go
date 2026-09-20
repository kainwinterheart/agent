package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st := newWorkflowState("do the thing", dir)
	st.AddArtifact(Artifact{ID: "001-coder", Iteration: 1, Subworkflow: "implement", Agent: "coder", Round: 1, Description: "d", Path: "/x.md"})
	st.DriverIteration = 3
	st.Active = &ActiveExecution{Iteration: 2, Subworkflow: "spec", Task: "refine the spec"}
	st.History = append(st.History, HistoryEntry{Iteration: 1, Subworkflow: "classify", Task: "classify the task", Outcome: "completed"})

	if err := st.save(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadWorkflowState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Task != "do the thing" {
		t.Errorf("task = %q", loaded.Task)
	}
	if loaded.DriverIteration != 3 {
		t.Errorf("driver iteration = %d", loaded.DriverIteration)
	}
	if len(loaded.Artifacts) != 1 || loaded.Artifacts[0].ID != "001-coder" {
		t.Errorf("artifacts = %+v", loaded.Artifacts)
	}
	if len(loaded.History) != 1 || loaded.History[0].Outcome != "completed" || loaded.History[0].Task != "classify the task" {
		t.Errorf("history = %+v", loaded.History)
	}
	if loaded.Active == nil || loaded.Active.Task != "refine the spec" || loaded.Active.Iteration != 2 {
		t.Errorf("active = %+v", loaded.Active)
	}
	if loaded.Artifacts[0].Iteration != 1 {
		t.Errorf("artifact iteration = %d, want 1", loaded.Artifacts[0].Iteration)
	}
}

func TestLoadState_MissingFile(t *testing.T) {
	_, err := loadWorkflowState(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing state file")
	}
}

func TestLoadState_EmptyTaskRejected(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".state"), 0o755)
	os.WriteFile(stateFilePath(dir), []byte(`{"task":"","seq":1}`), 0o644)
	if _, err := loadWorkflowState(dir); err == nil {
		t.Fatal("expected error for empty task in state")
	}
}

func TestNextSeq_StrictlyIncreasing(t *testing.T) {
	st := newWorkflowState("t", "")
	prev := st.NextSeq()
	for i := 0; i < 10; i++ {
		cur := st.NextSeq()
		if cur != prev+1 {
			t.Fatalf("seq not increasing: %d -> %d", prev, cur)
		}
		prev = cur
	}
}

func TestAddArtifact_DuplicatePanics(t *testing.T) {
	st := newWorkflowState("t", "")
	st.AddArtifact(Artifact{ID: "001-x", Path: "/a"})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate artifact id")
		}
	}()
	st.AddArtifact(Artifact{ID: "001-x", Path: "/b"})
}

func TestArtifactByID_CaseInsensitive(t *testing.T) {
	st := newWorkflowState("t", "")
	st.AddArtifact(Artifact{ID: "002-arch", Path: "/a"})
	if _, ok := st.ArtifactByID(" 002-ARCH "); !ok {
		t.Fatal("lookup should be case/space-insensitive")
	}
	if _, ok := st.ArtifactByID("999-none"); ok {
		t.Fatal("lookup should miss unknown ids")
	}
}
