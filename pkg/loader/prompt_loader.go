package loader

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

// The prompt templates and their shared fragments are embedded into the
// binary: they are static program data, and loading them must not depend on
// the working directory or on the source tree being present at runtime.
//
//go:embed prompts
var promptsFS embed.FS

type PromptID string

const (
	WORKFLOW_DRIVER_PROMPT_ID                   PromptID = "workflow_driver"
	LOOP_DECIDER_PROMPT_ID                      PromptID = "loop_decider"
	PRODUCT_MANAGER_PROMPT_ID                   PromptID = "product_manager"
	PM_REVIEW_PROMPT_ID                         PromptID = "pm_review"
	INVESTIGATION_CLASSIFIER_PROMPT_ID          PromptID = "investigation_classifier"
	SYSTEM_DECOMPOSITION_PROMPT_ID              PromptID = "system_decomposition"
	SYSTEM_DECOMPOSITION_REVIEW_PROMPT_ID       PromptID = "system_decomposition_review"
	ARCH_PROMPT_ID                              PromptID = "arch"
	ARCH_REVIEW_PROMPT_ID                       PromptID = "arch_review"
	PLAN_PROMPT_ID                              PromptID = "plan"
	PLAN_REVIEW_PROMPT_ID                       PromptID = "plan_review"
	CODER_PROMPT_ID                             PromptID = "coder"
	CODE_REVIEW_PROMPT_ID                       PromptID = "code_review"
	TECH_LEAD_FINAL_PROMPT_ID                   PromptID = "tech_lead_final"
	ARCH_FINAL_PROMPT_ID                        PromptID = "arch_final"
	INVESTIGATOR_PLANNER_PROMPT_ID              PromptID = "investigator_planner"
	INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT_ID PromptID = "investigation_plan_quality_review"
	STRUCTURE_REVIEW_PROMPT_ID                  PromptID = "structure_review"
	INVESTIGATOR_EXECUTOR_PROMPT_ID             PromptID = "investigator_executor"
	FACT_CHECKING_REVIEW_PROMPT_ID              PromptID = "fact_checking_review"
	GAP_ANALYSIS_REVIEW_PROMPT_ID               PromptID = "gap_analysis_review"
	SYNTHESIS_PROMPT_ID                         PromptID = "synthesis"
	SYNTHESIS_CONSISTENCY_REVIEW_PROMPT_ID      PromptID = "synthesis_consistency_review"
)

type promptLoader struct {
	mu    sync.RWMutex
	cache map[PromptID]*template.Template
}

var globalPromptLoader = &promptLoader{
	cache: make(map[PromptID]*template.Template),
}

// InitPromptLoader reads and parses all prompt templates together with the shared
// fragments they may reference.
func InitPromptLoader() {
	if err := globalPromptLoader.loadAll(); err != nil {
		panic(fmt.Sprintf("failed to initialize prompt templates: %v", err))
	}
}

func (pl *promptLoader) loadAll() error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	sharedDir := "prompts/shared"
	promptDir := "prompts"

	var sharedBuilder strings.Builder
	sharedFiles := []string{
		"noTools.txt",
		"INVESTIGATION_NO_TOOLS.txt",
		"COMMON_DAG_EXECUTION_MODEL.txt",
		"mustVerify.txt",
		"output_contract.txt",
		"review_verdict.txt",
	}
	for _, fname := range sharedFiles {
		data, err := promptsFS.ReadFile(filepath.Join(sharedDir, fname))
		if err != nil {
			return fmt.Errorf("read shared fragment %s: %w", fname, err)
		}
		sharedBuilder.WriteString(strings.TrimSpace(string(data)))
		sharedBuilder.WriteString("\n")
	}
	sharedTemplates := sharedBuilder.String()

	allIDs := []PromptID{
		WORKFLOW_DRIVER_PROMPT_ID,
		LOOP_DECIDER_PROMPT_ID,
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
		filename := string(id) + ".txt"
		tmplPath := filepath.Join(promptDir, filename)

		data, err := promptsFS.ReadFile(tmplPath)
		if err != nil {
			return fmt.Errorf("read prompt file %s: %w", filename, err)
		}

		mainContent := strings.TrimSpace(string(data))
		fullTemplate := sharedTemplates + mainContent
		tmpl, err := template.New(filename).Parse(fullTemplate)
		if err != nil {
			return fmt.Errorf("parse prompt %s: %w", filename, err)
		}

		pl.cache[id] = tmpl
	}

	return nil
}

// GetPrompt renders the role prompt template identified by id with no
// template data.
func GetPrompt(id PromptID) string {
	return RenderPrompt(id, nil)
}

// RenderPrompt renders the prompt template identified by id with the given
// data.
func RenderPrompt(id PromptID, data any) string {
	globalPromptLoader.mu.RLock()
	tmpl, ok := globalPromptLoader.cache[id]
	globalPromptLoader.mu.RUnlock()

	if !ok {
		panic(fmt.Sprintf("prompt %q not loaded", id))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("execute prompt %q: %v", id, err))
	}

	return buf.String()
}

// RenderDecisionPrompt renders a decision-agent prompt, injecting the JSON
// schema (pretty-printed) and the example response document.
func RenderDecisionPrompt(id PromptID, schema map[string]any, example string) string {
	b, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal schema for prompt %q: %v", id, err))
	}
	return RenderPrompt(id, struct {
		Schema  string
		Example string
	}{Schema: string(b), Example: example})
}
