package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStaticDefinitions_GraphIntegrity verifies the static wiring defined in
// src/definitions.go:
//   - every declared input has a producer (no dangling inputs)
//   - every non-terminal output is consumed by at least one agent
//     (no orphan artifacts)
//   - every content agent is used by exactly one subworkflow (no dead agents,
//     and artifact type = producing role stays unambiguous)
//   - subworkflow structure (metadata, 1-3 steps, producer first, resolvable
//     agents, artifact kinds, reviewers terminal)
func TestStaticDefinitions_GraphIntegrity(t *testing.T) {
	producers := map[string]string{} // artifact type -> producing agent
	for i := range AgentDefinitions {
		d := &AgentDefinitions[i]
		if d.Decision {
			continue
		}
		if len(d.Outputs) != 1 || d.Outputs[0] != d.Name {
			t.Errorf("content agent %s must produce exactly its own type, got %v", d.Name, d.Outputs)
		}
		producers[d.Name] = d.Name
	}

	// 1. No dangling inputs: every declared input type is produced.
	for i := range AgentDefinitions {
		d := &AgentDefinitions[i]
		for _, in := range d.Inputs {
			if _, ok := producers[in]; !ok {
				t.Errorf("agent %s declares input %q which no agent produces", d.Name, in)
			}
		}
	}

	// 2. No orphan outputs: every non-terminal content output is consumed by
	//    at least one agent's typed inputs.
	consumers := map[string]int{}
	for i := range AgentDefinitions {
		for _, in := range AgentDefinitions[i].Inputs {
			consumers[in]++
		}
	}
	for i := range AgentDefinitions {
		d := &AgentDefinitions[i]
		if d.Decision || d.OutputTerminal {
			continue
		}
		for _, out := range d.Outputs {
			if consumers[out] == 0 {
				t.Errorf("artifact type %q produced by %s is never consumed (orphan); mark OutputTerminal if it is a terminal deliverable", out, d.Name)
			}
		}
	}

	// 3. Every content agent is used by exactly one subworkflow; decision
	//    agents are control plane, never steps.
	uses := map[string]int{}
	for i := range SubworkflowDefinitions {
		for j := range SubworkflowDefinitions[i].Steps {
			uses[SubworkflowDefinitions[i].Steps[j].Agent]++
		}
	}
	for i := range AgentDefinitions {
		d := &AgentDefinitions[i]
		if d.Decision {
			if uses[d.Name] != 0 {
				t.Errorf("decision agent %s must not be a subworkflow step", d.Name)
			}
			continue
		}
		if uses[d.Name] != 1 {
			t.Errorf("content agent %s is used %d times in subworkflows, want exactly 1", d.Name, uses[d.Name])
		}
	}

	// 4. Subworkflow structure.
	seen := map[string]bool{}
	for i := range SubworkflowDefinitions {
		sw := &SubworkflowDefinitions[i]
		if seen[sw.ID] {
			t.Errorf("duplicate subworkflow id %s", sw.ID)
		}
		seen[sw.ID] = true
		if sw.ID == "" || sw.Name == "" || sw.Description == "" || sw.Produces == "" {
			t.Errorf("subworkflow %q has empty metadata", sw.ID)
		}
		if len(sw.Steps) < 1 || len(sw.Steps) > 3 {
			t.Errorf("subworkflow %s has %d steps; must be 1-3", sw.ID, len(sw.Steps))
		}
		for j, st := range sw.Steps {
			if agentDefinition(st.Agent) == nil {
				t.Errorf("subworkflow %s step %d references unknown agent %s", sw.ID, j, st.Agent)
				continue
			}
			if st.ArtifactKind == "" {
				t.Errorf("subworkflow %s step %d has no artifact kind", sw.ID, j)
			}
			if j == 0 && st.Reviewer {
				t.Errorf("subworkflow %s: first step must be the producer", sw.ID)
			}
		}
	}

	// 5. Reviewer documents are control-plane inputs (loop decider / driver),
	//    so reviewer agents must be terminal.
	for i := range SubworkflowDefinitions {
		for _, st := range SubworkflowDefinitions[i].Steps {
			if st.Reviewer && !agentDefinition(st.Agent).OutputTerminal {
				t.Errorf("reviewer agent %s must be OutputTerminal", st.Agent)
			}
		}
	}
}

// TestStaticDefinitions_Document verifies the dumper: every agent and
// subworkflow is rendered, and the document can be written to (and read back
// from) a trace artifact path.
func TestStaticDefinitions_Document(t *testing.T) {
	doc := StaticDefinitionsDocument()
	for i := range AgentDefinitions {
		if !strings.Contains(doc, "### "+AgentDefinitions[i].Name+" ") {
			t.Errorf("document missing agent %s", AgentDefinitions[i].Name)
		}
	}
	for i := range SubworkflowDefinitions {
		if !strings.Contains(doc, "### "+SubworkflowDefinitions[i].ID+" —") {
			t.Errorf("document missing subworkflow %s", SubworkflowDefinitions[i].ID)
		}
	}
	for _, section := range []string{"## Agents", "## Subworkflows", "## Wiring Graph"} {
		if !strings.Contains(doc, section) {
			t.Errorf("document missing section %q", section)
		}
	}

	path := filepath.Join(t.TempDir(), ".state", "static_definitions.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read dumped definitions: %v", err)
	}
	if string(data) != doc {
		t.Error("dumped document differs from the rendered document")
	}
}
