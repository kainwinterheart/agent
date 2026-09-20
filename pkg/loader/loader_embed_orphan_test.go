package loader

import (
	"io/fs"
	"path"
	"strings"
	"testing"
)

// testAllIDs mirrors the allIDs list in loadAll (prompt_loader.go). The
// duplication follows the pattern established in loader_embed_test.go and is
// deliberate until the D-04 de-duplication lands.
var testAllIDs = []PromptID{
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

// testSharedFiles mirrors the sharedFiles list in loadAll (prompt_loader.go).
var testSharedFiles = []string{
	"noTools.txt",
	"INVESTIGATION_NO_TOOLS.txt",
	"COMMON_DAG_EXECUTION_MODEL.txt",
	"mustVerify.txt",
	"output_contract.txt",
	"review_verdict.txt",
}

// TestEmbeddedPromptFilesAllListed pins D-02 (defect register in
// roadmap/2026-09-20_commit_c9831d9_correctness_review.md): //go:embed prompts
// embeds the whole tree, but loadAll reads only the files listed in
// allIDs/sharedFiles, so an extra .txt on disk is embedded-but-ignored
// silently (only the listed-but-missing direction fails at init). This test
// walks the embedded FS and fails on any .txt file that is not listed.
func TestEmbeddedPromptFilesAllListed(t *testing.T) {
	listed := make(map[string]bool, len(testAllIDs)+len(testSharedFiles))
	for _, id := range testAllIDs {
		listed[path.Join("prompts", string(id)+".txt")] = true
	}
	for _, fname := range testSharedFiles {
		listed[path.Join("prompts", "shared", fname)] = true
	}

	var orphans []string
	err := fs.WalkDir(promptsFS, "prompts", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".txt") {
			return nil
		}
		if !listed[p] {
			orphans = append(orphans, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded prompts tree: %v", err)
	}
	for _, p := range orphans {
		t.Errorf("embedded prompt file %s is not listed in allIDs/sharedFiles: it is embedded but silently ignored by loadAll", p)
	}
}
