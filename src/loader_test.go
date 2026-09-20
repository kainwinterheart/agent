package main

import (
	"strings"
	"testing"

	"agent-go/pkg/loader"
)

func TestLoader_AllPromptsLoaded(t *testing.T) {
	prompts := map[string]string{
		"product_manager":                   loader.PRODUCT_MANAGER_PROMPT,
		"pm_review":                         loader.PM_REVIEW_PROMPT,
		"investigation_classifier":          loader.INVESTIGATION_CLASSIFIER_PROMPT,
		"system_decomposition":              loader.SYSTEM_DECOMPOSITION_PROMPT,
		"system_decomposition_review":       loader.SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
		"arch":                              loader.ARCH_PROMPT,
		"arch_review":                       loader.ARCH_REVIEW_PROMPT,
		"plan":                              loader.PLAN_PROMPT,
		"plan_review":                       loader.PLAN_REVIEW_PROMPT,
		"coder":                             loader.CODER_PROMPT,
		"code_review":                       loader.CODE_REVIEW_PROMPT,
		"tech_lead_final":                   loader.TECH_LEAD_FINAL_PROMPT,
		"arch_final":                        loader.ARCH_FINAL_PROMPT,
		"investigator_planner":              loader.INVESTIGATOR_PLANNER_PROMPT,
		"investigation_plan_quality_review": loader.INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT,
		"structure_review":                  loader.STRUCTURE_REVIEW_PROMPT,
		"investigator_executor":             loader.INVESTIGATOR_EXECUTOR_PROMPT,
		"fact_checking_review":              loader.FACT_CHECKING_REVIEW_PROMPT,
		"gap_analysis_review":               loader.GAP_ANALYSIS_REVIEW_PROMPT,
		"synthesis":                         loader.SYNTHESIS_PROMPT,
		"synthesis_consistency_review":      loader.SYNTHESIS_CONSISTENCY_REVIEW_PROMPT,
	}
	for name, p := range prompts {
		if strings.TrimSpace(p) == "" {
			t.Errorf("prompt %s is empty", name)
		}
	}
}

func TestLoader_OutputContractPresent(t *testing.T) {
	// Every content-agent role prompt must carry the shared file-output
	// contract, since every content agent's deliverable is a file on disk.
	if !strings.Contains(loader.CODER_PROMPT, "Final output contract") {
		t.Error("coder prompt missing output contract")
	}
	if !strings.Contains(loader.SYNTHESIS_PROMPT, "Final output contract") {
		t.Error("synthesis prompt missing output contract")
	}
}

func TestLoader_ReviewerPromptsCarryVerdictRequirement(t *testing.T) {
	reviewers := []string{
		loader.PM_REVIEW_PROMPT,
		loader.SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
		loader.ARCH_REVIEW_PROMPT,
		loader.PLAN_REVIEW_PROMPT,
		loader.CODE_REVIEW_PROMPT,
		loader.TECH_LEAD_FINAL_PROMPT,
		loader.ARCH_FINAL_PROMPT,
		loader.INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT,
		loader.STRUCTURE_REVIEW_PROMPT,
		loader.FACT_CHECKING_REVIEW_PROMPT,
		loader.GAP_ANALYSIS_REVIEW_PROMPT,
		loader.SYNTHESIS_CONSISTENCY_REVIEW_PROMPT,
	}
	for i, p := range reviewers {
		if !strings.Contains(p, "Verdict requirement") {
			t.Errorf("reviewer prompt %d missing verdict requirement", i)
		}
		if !strings.Contains(p, "**Verdict:** APPROVED") {
			t.Errorf("reviewer prompt %d missing the exact verdict line format", i)
		}
	}
}

func TestLoader_DecisionPromptsUseJSONContract(t *testing.T) {
	// Decision agents (driver, loop decider) respond with strictly validated
	// JSON, not Markdown files: their prompts carry the JSON contract instead
	// of the file-output contract.
	for name, p := range map[string]string{
		"workflow_driver": DriverAgent.RolePrompt,
		"loop_decider":    LoopDeciderAgent.RolePrompt,
	} {
		if strings.Contains(p, "Final output contract") {
			t.Errorf("%s prompt must not carry the file-output contract", name)
		}
		if !strings.Contains(p, "Output MUST be valid JSON only") {
			t.Errorf("%s prompt missing the JSON output contract", name)
		}
	}
}

func TestSubworkflowRegistry(t *testing.T) {
	if len(Subworkflows) == 0 {
		t.Fatal("subworkflow registry empty")
	}
	seen := map[string]bool{}
	for i := range Subworkflows {
		sw := &Subworkflows[i]
		if seen[sw.ID] {
			t.Errorf("duplicate subworkflow id %s", sw.ID)
		}
		seen[sw.ID] = true
		if sw.ID == "" || sw.Name == "" || sw.Description == "" || sw.Produces == "" {
			t.Errorf("subworkflow %q has empty metadata", sw.ID)
		}
		if len(sw.Steps) < 1 || len(sw.Steps) > 3 {
			t.Errorf("subworkflow %s has %d steps; subworkflows must be short (1-3 agents)", sw.ID, len(sw.Steps))
		}
		for j := range sw.Steps {
			step := &sw.Steps[j]
			if step.Agent == nil || step.Agent.Name == "" {
				t.Errorf("subworkflow %s step %d has no agent", sw.ID, j)
			}
			if step.Agent.RolePrompt == "" {
				t.Errorf("subworkflow %s step %d agent has empty role prompt", sw.ID, j)
			}
			if step.ArtifactKind == "" {
				t.Errorf("subworkflow %s step %d has no artifact description", sw.ID, j)
			}
			// The first step is the producer; reviewers come after it.
			if j == 0 && step.Reviewer {
				t.Errorf("subworkflow %s: first step must be the producer", sw.ID)
			}
		}
	}
	// The spec-required example groups must exist.
	for _, id := range []string{"implement", "architect", "arch_final", "investigate"} {
		if getSubworkflow(id) == nil {
			t.Errorf("required subworkflow %q missing", id)
		}
	}
}
