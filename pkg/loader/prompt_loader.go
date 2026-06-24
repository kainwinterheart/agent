package loader

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

var promptBaseDir string

func initPromptBaseDir() {
	root := findModuleRoot()
	if root != "" {
		candidate := filepath.Join(root, "pkg", "loader", "prompts")
		if _, err := os.Stat(candidate); err == nil {
			abs, _ := filepath.Abs(filepath.Dir(candidate))
			promptBaseDir = abs
			return
		}
		candidate = filepath.Join(root, "prompts")
		if _, err := os.Stat(candidate); err == nil {
			abs, _ := filepath.Abs(filepath.Dir(candidate))
			promptBaseDir = abs
			return
		}
	}
	cwd, _ := os.Getwd()
	abs, _ := filepath.Abs(cwd)
	promptBaseDir = abs
}

func init() {
	initPromptBaseDir()
}

type PromptID string

const (
	ARCH_PROMPT_ID                              PromptID = "arch"
	PLAN_PROMPT_ID                              PromptID = "plan"
	CODER_PROMPT_ID                             PromptID = "coder"
	ARCH_REVIEW_PROMPT_ID                       PromptID = "arch_review"
	PLAN_REVIEW_PROMPT_ID                       PromptID = "plan_review"
	CODE_REVIEW_PROMPT_ID                       PromptID = "code_review"
	TECH_LEAD_FINAL_PROMPT_ID                   PromptID = "tech_lead_final"
	ARCH_FINAL_PROMPT_ID                        PromptID = "arch_final"
	PRODUCT_MANAGER_PROMPT_ID                   PromptID = "product_manager"
	PM_SYNTHESIZER_PROMPT_ID                    PromptID = "pm_synthesizer"
	PM_EXPANSION_CLEANUP_PROMPT_ID              PromptID = "pm_expansion_cleanup"
	PM_REVIEW_PROMPT_ID                         PromptID = "pm_review"
	SYSTEM_DECOMPOSITION_PROMPT_ID              PromptID = "system_decomposition"
	SYSTEM_DECOMPOSITION_REVIEW_PROMPT_ID       PromptID = "system_decomposition_review"
	DESIGN_TO_IMPLEMENT_PHRASING_PROMPT_ID      PromptID = "design_to_implement_phrasing"
	INVESTIGATION_CLASSIFIER_PROMPT_ID          PromptID = "investigation_classifier"
	INVESTIGATOR_PLANNER_PROMPT_ID              PromptID = "investigator_planner"
	INVESTIGATOR_EXECUTOR_PROMPT_ID             PromptID = "investigator_executor"
	SYNTHESIS_PROMPT_ID                         PromptID = "synthesis"
	GAP_ANALYSIS_REVIEW_PROMPT_ID               PromptID = "gap_analysis_review"
	FACT_CHECKING_REVIEW_PROMPT_ID              PromptID = "fact_checking_review"
	STRUCTURE_REVIEW_PROMPT_ID                  PromptID = "structure_review"
	INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT_ID PromptID = "investigation_plan_quality_review"
	SYNTHESIS_CONSISTENCY_REVIEW_PROMPT_ID      PromptID = "synthesis_consistency_review"
	NON_CODER_NEXT_STEPS_CLEANUP_PROMPT_ID      PromptID = "non_coder_next_steps_cleanup"
)

type promptLoader struct {
	mu    sync.RWMutex
	cache map[PromptID]*template.Template
}

var globalPromptLoader = &promptLoader{
	cache: make(map[PromptID]*template.Template),
}

var (
	Followup       string
	ReviewerResume string
)

func InitPromptLoader() {
	if err := globalPromptLoader.loadAll(); err != nil {
		panic(fmt.Sprintf("failed to initialize prompt templates: %v", err))
	}
}

func (pl *promptLoader) loadAll() error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	baseDir := promptBaseDir
	sharedDir := filepath.Join(baseDir, "prompts", "shared")
	promptDir := filepath.Join(baseDir, "prompts")

	var sharedBuilder strings.Builder
	sharedFiles := []string{
		"noTools.txt",
		"INVESTIGATION_NO_TOOLS.txt",
		"COMMON_DAG_EXECUTION_MODEL.txt",
		"mustVerify.txt",
	}
	for _, fname := range sharedFiles {
		data, err := os.ReadFile(filepath.Join(sharedDir, fname))
		if err != nil {
			return fmt.Errorf("read shared fragment %s: %w", fname, err)
		}
		sharedBuilder.WriteString(strings.TrimSpace(string(data)))
	}
	sharedTemplates := sharedBuilder.String()

	followupPath := filepath.Join(sharedDir, "FOLLOWUP.txt")
	data, err := os.ReadFile(followupPath)
	if err != nil {
		return fmt.Errorf("read followup fragment: %w", err)
	}
	Followup = strings.TrimSpace(string(data))

	rpPath := filepath.Join(promptDir, "reviewer_resume.txt")
	rpData, err := os.ReadFile(rpPath)
	if err != nil {
		return fmt.Errorf("read reviewer_resume: %w", err)
	}
	ReviewerResume = string(rpData)

	allIDs := []PromptID{
		ARCH_PROMPT_ID, PLAN_PROMPT_ID, CODER_PROMPT_ID,
		ARCH_REVIEW_PROMPT_ID, PLAN_REVIEW_PROMPT_ID, CODE_REVIEW_PROMPT_ID,
		TECH_LEAD_FINAL_PROMPT_ID, ARCH_FINAL_PROMPT_ID,
		PRODUCT_MANAGER_PROMPT_ID, PM_SYNTHESIZER_PROMPT_ID,
		PM_EXPANSION_CLEANUP_PROMPT_ID, PM_REVIEW_PROMPT_ID,
		SYSTEM_DECOMPOSITION_PROMPT_ID, SYSTEM_DECOMPOSITION_REVIEW_PROMPT_ID,
		DESIGN_TO_IMPLEMENT_PHRASING_PROMPT_ID,
		INVESTIGATION_CLASSIFIER_PROMPT_ID,
		INVESTIGATOR_PLANNER_PROMPT_ID, INVESTIGATOR_EXECUTOR_PROMPT_ID, SYNTHESIS_PROMPT_ID,
		GAP_ANALYSIS_REVIEW_PROMPT_ID, FACT_CHECKING_REVIEW_PROMPT_ID,
		STRUCTURE_REVIEW_PROMPT_ID, INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT_ID,
		SYNTHESIS_CONSISTENCY_REVIEW_PROMPT_ID,
		NON_CODER_NEXT_STEPS_CLEANUP_PROMPT_ID,
	}

	for _, id := range allIDs {
		filename := string(id) + ".txt"
		tmplPath := filepath.Join(promptDir, filename)

		data, err := os.ReadFile(tmplPath)
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

type promptTemplateData struct {
	Schema string
}

func GetPrompt(id PromptID, schemaExample string) string {
	globalPromptLoader.mu.RLock()
	tmpl, ok := globalPromptLoader.cache[id]
	globalPromptLoader.mu.RUnlock()

	if !ok {
		panic(fmt.Sprintf("prompt %q not loaded", id))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, promptTemplateData{Schema: schemaExample}); err != nil {
		panic(fmt.Sprintf("execute prompt %q: %v", id, err))
	}

	return buf.String()
}
