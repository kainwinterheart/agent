package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

var schemaBaseDir string

func initSchemaBaseDir() {
	root := findModuleRoot()
	if root != "" {
		candidate := filepath.Join(root, "pkg", "loader", "schemas")
		if _, err := os.Stat(candidate); err == nil {
			abs, _ := filepath.Abs(filepath.Dir(candidate))
			schemaBaseDir = abs
			return
		}
		candidate = filepath.Join(root, "schemas")
		if _, err := os.Stat(candidate); err == nil {
			abs, _ := filepath.Abs(filepath.Dir(candidate))
			schemaBaseDir = abs
			return
		}
	}
	cwd, _ := os.Getwd()
	abs, _ := filepath.Abs(cwd)
	schemaBaseDir = abs
}

func init() {
	initSchemaBaseDir()
}

type SchemaID string

const (
	ARCH_SCHEMA_ID                              SchemaID = "arch"
	PLAN_SCHEMA_ID                              SchemaID = "plan"
	CODER_SCHEMA_ID                             SchemaID = "coder"
	ARCH_REVIEW_SCHEMA_ID                       SchemaID = "arch_review"
	PLAN_REVIEW_SCHEMA_ID                       SchemaID = "plan_review"
	CODE_REVIEW_SCHEMA_ID                       SchemaID = "code_review"
	TECH_LEAD_FINAL_SCHEMA_ID                   SchemaID = "tech_lead_final"
	ARCH_FINAL_SCHEMA_ID                        SchemaID = "arch_final"
	PRODUCT_MANAGER_SCHEMA_ID                   SchemaID = "product_manager"
	PM_SYNTHESIZER_SCHEMA_ID                    SchemaID = "pm_synthesizer"
	PM_EXPANSION_CLEANUP_SCHEMA_ID              SchemaID = "pm_expansion_cleanup"
	NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA_ID      SchemaID = "non_coder_next_steps_cleanup"
	PM_REVIEW_SCHEMA_ID                         SchemaID = "pm_review"
	SYSTEM_DECOMPOSITION_SCHEMA_ID              SchemaID = "system_decomposition"
	SYSTEM_DECOMPOSITION_REVIEW_SCHEMA_ID       SchemaID = "system_decomposition_review"
	DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA_ID      SchemaID = "design_to_implement_phrasing"
	INVESTIGATION_CLASSIFIER_SCHEMA_ID          SchemaID = "investigation_classifier"
	INVESTIGATOR_PLAN_SCHEMA_ID                 SchemaID = "investigator_plan"
	INVESTIGATOR_FINDINGS_SCHEMA_ID             SchemaID = "investigator_findings"
	INVESTIGATION_REPORT_SCHEMA_ID              SchemaID = "investigation_report"
	GAP_ANALYSIS_REVIEW_SCHEMA_ID               SchemaID = "gap_analysis_review"
	FACT_CHECKING_REVIEW_SCHEMA_ID              SchemaID = "fact_checking_review"
	STRUCTURAL_REVIEW_SCHEMA_ID                 SchemaID = "structural_review"
	INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA_ID SchemaID = "investigation_plan_quality_review"
	SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA_ID      SchemaID = "synthesis_consistency_review"
)

var (
	archNextActionsDesc string
	codeNextActionsDesc string
	planNextActionsDesc string
)

type schemaLoader struct {
	mu    sync.RWMutex
	cache map[SchemaID]map[string]interface{}
}

var globalSchemaLoader = &schemaLoader{
	cache: make(map[SchemaID]map[string]interface{}),
}

var (
	ARCH_SCHEMA                              map[string]interface{}
	PLAN_SCHEMA                              map[string]interface{}
	CODER_SCHEMA                             map[string]interface{}
	ARCH_REVIEW_SCHEMA                       map[string]interface{}
	PLAN_REVIEW_SCHEMA                       map[string]interface{}
	CODE_REVIEW_SCHEMA                       map[string]interface{}
	TECH_LEAD_FINAL_SCHEMA                   map[string]interface{}
	ARCH_FINAL_SCHEMA                        map[string]interface{}
	PRODUCT_MANAGER_SCHEMA                   map[string]interface{}
	PM_SYNTHESIZER_SCHEMA                    map[string]interface{}
	PM_EXPANSION_CLEANUP_SCHEMA              map[string]interface{}
	NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA      map[string]interface{}
	PM_REVIEW_SCHEMA                         map[string]interface{}
	SYSTEM_DECOMPOSITION_SCHEMA              map[string]interface{}
	SYSTEM_DECOMPOSITION_REVIEW_SCHEMA       map[string]interface{}
	DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA      map[string]interface{}
	INVESTIGATION_CLASSIFIER_SCHEMA          map[string]interface{}
	INVESTIGATOR_PLAN_SCHEMA                 map[string]interface{}
	INVESTIGATOR_FINDINGS_SCHEMA             map[string]interface{}
	INVESTIGATION_REPORT_SCHEMA              map[string]interface{}
	GAP_ANALYSIS_REVIEW_SCHEMA               map[string]interface{}
	FACT_CHECKING_REVIEW_SCHEMA              map[string]interface{}
	STRUCTURAL_REVIEW_SCHEMA                 map[string]interface{}
	INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA map[string]interface{}
	SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA      map[string]interface{}
)

func InitSchemas() {
	if err := globalSchemaLoader.loadAll(); err != nil {
		panic(fmt.Sprintf("failed to initialize schema templates: %v", err))
	}
	ARCH_SCHEMA = globalSchemaLoader.cache[ARCH_SCHEMA_ID]
	PLAN_SCHEMA = globalSchemaLoader.cache[PLAN_SCHEMA_ID]
	CODER_SCHEMA = globalSchemaLoader.cache[CODER_SCHEMA_ID]
	ARCH_REVIEW_SCHEMA = globalSchemaLoader.cache[ARCH_REVIEW_SCHEMA_ID]
	PLAN_REVIEW_SCHEMA = globalSchemaLoader.cache[PLAN_REVIEW_SCHEMA_ID]
	CODE_REVIEW_SCHEMA = globalSchemaLoader.cache[CODE_REVIEW_SCHEMA_ID]
	TECH_LEAD_FINAL_SCHEMA = globalSchemaLoader.cache[TECH_LEAD_FINAL_SCHEMA_ID]
	ARCH_FINAL_SCHEMA = globalSchemaLoader.cache[ARCH_FINAL_SCHEMA_ID]
	PRODUCT_MANAGER_SCHEMA = globalSchemaLoader.cache[PRODUCT_MANAGER_SCHEMA_ID]
	PM_SYNTHESIZER_SCHEMA = globalSchemaLoader.cache[PM_SYNTHESIZER_SCHEMA_ID]
	PM_EXPANSION_CLEANUP_SCHEMA = globalSchemaLoader.cache[PM_EXPANSION_CLEANUP_SCHEMA_ID]
	NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA = globalSchemaLoader.cache[NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA_ID]
	PM_REVIEW_SCHEMA = globalSchemaLoader.cache[PM_REVIEW_SCHEMA_ID]
	SYSTEM_DECOMPOSITION_SCHEMA = globalSchemaLoader.cache[SYSTEM_DECOMPOSITION_SCHEMA_ID]
	SYSTEM_DECOMPOSITION_REVIEW_SCHEMA = globalSchemaLoader.cache[SYSTEM_DECOMPOSITION_REVIEW_SCHEMA_ID]
	DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA = globalSchemaLoader.cache[DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA_ID]
	INVESTIGATION_CLASSIFIER_SCHEMA = globalSchemaLoader.cache[INVESTIGATION_CLASSIFIER_SCHEMA_ID]
	INVESTIGATOR_PLAN_SCHEMA = globalSchemaLoader.cache[INVESTIGATOR_PLAN_SCHEMA_ID]
	INVESTIGATOR_FINDINGS_SCHEMA = globalSchemaLoader.cache[INVESTIGATOR_FINDINGS_SCHEMA_ID]
	INVESTIGATION_REPORT_SCHEMA = globalSchemaLoader.cache[INVESTIGATION_REPORT_SCHEMA_ID]
	GAP_ANALYSIS_REVIEW_SCHEMA = globalSchemaLoader.cache[GAP_ANALYSIS_REVIEW_SCHEMA_ID]
	FACT_CHECKING_REVIEW_SCHEMA = globalSchemaLoader.cache[FACT_CHECKING_REVIEW_SCHEMA_ID]
	STRUCTURAL_REVIEW_SCHEMA = globalSchemaLoader.cache[STRUCTURAL_REVIEW_SCHEMA_ID]
	INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA = globalSchemaLoader.cache[INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA_ID]
	SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA = globalSchemaLoader.cache[SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA_ID]
}

func (sl *schemaLoader) loadAll() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	baseDir := schemaBaseDir
	sharedDir := filepath.Join(baseDir, "schemas", "shared")
	promptsDir := filepath.Join(promptBaseDir, "prompts", "shared")

	promptFiles := map[string]string{
		"arch_next_actions.txt": "arch",
		"code_next_actions.txt": "code",
		"plan_next_actions.txt": "plan",
	}
	for fname, key := range promptFiles {
		data, err := os.ReadFile(filepath.Join(promptsDir, fname))
		if err != nil {
			return fmt.Errorf("read shared prompt %s: %w", fname, err)
		}
		val := string(data)
		switch key {
		case "arch":
			archNextActionsDesc = val
		case "code":
			codeNextActionsDesc = val
		case "plan":
			planNextActionsDesc = val
		}
	}

	var sharedBuilder strings.Builder
	sharedFiles := []string{"approved.json", "issue_item.json", "next_steps.json", "review_base.json", "review_base_desc.json", "review_base_no_ns.json"}
	for _, fname := range sharedFiles {
		data, err := os.ReadFile(filepath.Join(sharedDir, fname))
		if err != nil {
			return fmt.Errorf("read shared fragment %s: %w", fname, err)
		}
		sharedBuilder.WriteString(strings.TrimSpace(string(data)))
		sharedBuilder.WriteString("\n")
	}
	sharedTemplates := sharedBuilder.String()

	allIDs := []SchemaID{
		ARCH_SCHEMA_ID, PLAN_SCHEMA_ID, CODER_SCHEMA_ID,
		ARCH_REVIEW_SCHEMA_ID, PLAN_REVIEW_SCHEMA_ID, CODE_REVIEW_SCHEMA_ID,
		TECH_LEAD_FINAL_SCHEMA_ID, ARCH_FINAL_SCHEMA_ID,
		PRODUCT_MANAGER_SCHEMA_ID, PM_SYNTHESIZER_SCHEMA_ID,
		PM_EXPANSION_CLEANUP_SCHEMA_ID, NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA_ID,
		PM_REVIEW_SCHEMA_ID,
		SYSTEM_DECOMPOSITION_SCHEMA_ID, SYSTEM_DECOMPOSITION_REVIEW_SCHEMA_ID,
		DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA_ID,
		INVESTIGATION_CLASSIFIER_SCHEMA_ID,
		INVESTIGATOR_PLAN_SCHEMA_ID, INVESTIGATOR_FINDINGS_SCHEMA_ID, INVESTIGATION_REPORT_SCHEMA_ID,
		GAP_ANALYSIS_REVIEW_SCHEMA_ID, FACT_CHECKING_REVIEW_SCHEMA_ID,
		STRUCTURAL_REVIEW_SCHEMA_ID, INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA_ID,
		SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA_ID,
	}

	for _, id := range allIDs {
		filename := string(id) + ".json"
		tmplPath := filepath.Join(baseDir, "schemas", filename)

		data, err := os.ReadFile(tmplPath)
		if err != nil {
			return fmt.Errorf("read schema file %s: %w", filename, err)
		}

		mainContent := strings.TrimSpace(string(data))
		fullTemplate := sharedTemplates + mainContent

		funcs := template.FuncMap{
			"makeConfig": func(categoryDesc, sevReasonDesc, messageDesc, nextActionsDesc string, includeCategory bool) map[string]interface{} {
				return map[string]interface{}{
					"CategoryDesc":    categoryDesc,
					"SevReasonDesc":   sevReasonDesc,
					"MessageDesc":     messageDesc,
					"NextActionsDesc": nextActionsDesc,
					"IncludeCategory": includeCategory,
				}
			},
			"archNextActionsDesc": func() string { return archNextActionsDesc },
			"codeNextActionsDesc": func() string { return codeNextActionsDesc },
			"planNextActionsDesc": func() string { return planNextActionsDesc },
			"escapeJSON": func(s string) string {
				b, _ := json.Marshal(s)
				return string(b)[1 : len(b)-1]
			},
			"dict": func(kvs ...interface{}) map[string]interface{} {
				m := make(map[string]interface{})
				for i := 0; i < len(kvs); i += 2 {
					if key, ok := kvs[i].(string); ok {
						m[key] = kvs[i+1]
					}
				}
				return m
			},
		}

		tmpl, err := template.New(filename).Funcs(funcs).Parse(fullTemplate)
		if err != nil {
			return fmt.Errorf("parse schema template %s: %w", filename, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, struct{}{}); err != nil {
			return fmt.Errorf("execute schema template %s: %w", filename, err)
		}

		var schema map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
			return fmt.Errorf("unmarshal schema %s: %w (rendered: %s)", filename, err, buf.String())
		}
		sl.cache[id] = schema
	}

	return nil
}

func GetSchema(id SchemaID) (map[string]interface{}, error) {
	globalSchemaLoader.mu.RLock()
	schema, ok := globalSchemaLoader.cache[id]
	globalSchemaLoader.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("schema %q not loaded", id)
	}
	return schema, nil
}

func ListSchemas() []SchemaID {
	return []SchemaID{
		ARCH_SCHEMA_ID, PLAN_SCHEMA_ID, CODER_SCHEMA_ID,
		ARCH_REVIEW_SCHEMA_ID, PLAN_REVIEW_SCHEMA_ID, CODE_REVIEW_SCHEMA_ID,
		TECH_LEAD_FINAL_SCHEMA_ID, ARCH_FINAL_SCHEMA_ID,
		PRODUCT_MANAGER_SCHEMA_ID, PM_SYNTHESIZER_SCHEMA_ID,
		PM_EXPANSION_CLEANUP_SCHEMA_ID, NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA_ID,
		PM_REVIEW_SCHEMA_ID,
		SYSTEM_DECOMPOSITION_SCHEMA_ID, SYSTEM_DECOMPOSITION_REVIEW_SCHEMA_ID,
		DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA_ID,
		INVESTIGATION_CLASSIFIER_SCHEMA_ID,
		INVESTIGATOR_PLAN_SCHEMA_ID, INVESTIGATOR_FINDINGS_SCHEMA_ID, INVESTIGATION_REPORT_SCHEMA_ID,
		GAP_ANALYSIS_REVIEW_SCHEMA_ID, FACT_CHECKING_REVIEW_SCHEMA_ID,
		STRUCTURAL_REVIEW_SCHEMA_ID, INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA_ID,
		SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA_ID,
	}
}

func findModuleRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return ""
}
