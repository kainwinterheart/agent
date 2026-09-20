package loader

import (
	"strings"
	"testing"
	"text/template"
)

// TestLoadPrompts_FromArbitraryWorkingDirectory pins the embed-based loading:
// the prompts must load (and render) from a working directory that has no
// relation to the source tree.
func TestLoadPrompts_FromArbitraryWorkingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())

	pl := &promptLoader{cache: make(map[PromptID]*template.Template)}
	if err := pl.loadAll(); err != nil {
		t.Fatalf("loadAll from unrelated working directory: %v", err)
	}

	allIDs := []PromptID{
		WORKFLOW_DRIVER_PROMPT_ID, LOOP_DECIDER_PROMPT_ID,
		PRODUCT_MANAGER_PROMPT_ID, PM_REVIEW_PROMPT_ID,
		INVESTIGATION_CLASSIFIER_PROMPT_ID,
		SYSTEM_DECOMPOSITION_PROMPT_ID, SYSTEM_DECOMPOSITION_REVIEW_PROMPT_ID,
		ARCH_PROMPT_ID, ARCH_REVIEW_PROMPT_ID,
		PLAN_PROMPT_ID, PLAN_REVIEW_PROMPT_ID,
		CODER_PROMPT_ID, CODE_REVIEW_PROMPT_ID,
		TECH_LEAD_FINAL_PROMPT_ID, ARCH_FINAL_PROMPT_ID,
		INVESTIGATOR_PLANNER_PROMPT_ID, INVESTIGATOR_EXECUTOR_PROMPT_ID, SYNTHESIS_PROMPT_ID,
		FACT_CHECKING_REVIEW_PROMPT_ID, GAP_ANALYSIS_REVIEW_PROMPT_ID,
		STRUCTURE_REVIEW_PROMPT_ID, INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT_ID,
		SYNTHESIS_CONSISTENCY_REVIEW_PROMPT_ID,
	}
	for _, id := range allIDs {
		tmpl, ok := pl.cache[id]
		if !ok {
			t.Errorf("prompt %s not loaded", id)
			continue
		}
		var buf strings.Builder
		if err := tmpl.Execute(&buf, nil); err != nil {
			t.Errorf("prompt %s does not render: %v", id, err)
		} else if strings.TrimSpace(buf.String()) == "" {
			t.Errorf("prompt %s renders empty", id)
		}
	}
}
