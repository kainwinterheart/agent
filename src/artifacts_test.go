package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewArtifactPath_UniqueAndStructured(t *testing.T) {
	st := newWorkflowState("t", "")
	id1, p1 := st.newArtifactPath("/sess", "implement", "coder", 1)
	id2, p2 := st.newArtifactPath("/sess", "implement", "coder", 2)
	id3, p3 := st.newArtifactPath("/sess", "spec", "product_manager", 1)

	if id1 == id2 || id1 == id3 || id2 == id3 {
		t.Fatalf("ids not unique: %s %s %s", id1, id2, id3)
	}
	if p1 == p2 || p1 == p3 {
		t.Fatalf("paths not unique: %s %s %s", p1, p2, p3)
	}
	if filepath.Ext(p1) != ".md" {
		t.Errorf("path should be markdown: %s", p1)
	}
	if !strings.Contains(p1, "artifacts") {
		t.Errorf("path should live under artifacts/: %s", p1)
	}
	if strings.Contains(filepath.Base(p1), "_r") {
		t.Errorf("round 1 path should have no round suffix: %s", p1)
	}
	if !strings.Contains(filepath.Base(p2), "_r2") {
		t.Errorf("round 2 path should carry round suffix: %s", p2)
	}
	if !strings.Contains(id1, "coder") || !strings.Contains(id3, "product_manager") {
		t.Errorf("id should embed agent name: %s, %s", id1, id3)
	}
}

func TestNewDecisionPath_Unique(t *testing.T) {
	st := newWorkflowState("t", "")
	p1 := st.newDecisionPath("/sess")
	p2 := st.newDecisionPath("/sess")
	if p1 == p2 {
		t.Fatal("decision paths not unique")
	}
	if !strings.Contains(p1, "decisions") {
		t.Errorf("decision path should live under decisions/: %s", p1)
	}
}
