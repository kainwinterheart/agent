// =========================
// CONTENT PROCESSORS (Tier 2)
// =========================
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// markdownDocHook allows overriding MarkdownDocumentGenerator for testing.
var markdownDocHook func(content interface{}, stageName string, subdir []string) string

// BuildArchitectInput builds architect input from a domain and integration ownership.
func BuildArchitectInput(domain map[string]interface{}, integrationOwnership []interface{}) (string, string) {
	spec, ok := domain["domain_specification"].(string)
	if !ok || spec == "" {
		return "", ""
	}
	if !strings.HasSuffix(spec, "\n") {
		spec += "\n"
	}
	domainID := strings.ToLower(strings.TrimSpace(domain["id"].(string)))
	var integrations []interface{}
	for _, item := range integrationOwnership {
		if m, ok := item.(map[string]interface{}); ok {
			ownerID := strings.ToLower(strings.TrimSpace(m["owner_domain_id"].(string)))
			if ownerID == domainID {
				integrations = append(integrations, m)
			}
		}
	}
	if len(integrations) > 0 {
		spec += "\nAdditionally, you are EXPECTED TO HANDLE integration of the following capabilities into the overall system:\n"
		for i, item := range integrations {
			if m, ok := item.(map[string]interface{}); ok {
				cap := m["capability"].(string)
				artifacts, _ := m["integration_artifacts"].([]interface{})
				suffix := ""
				if len(artifacts) > 0 {
					suffix = ":"
				}
				spec += fmt.Sprintf("%d. %s%s\n", i+1, cap, suffix)
				for j, sub := range artifacts {
					if s, ok := sub.(string); ok {
						spec += fmt.Sprintf("\t%d. %s\n", j+1, s)
					}
				}
			}
		}
	}
	more := ""
	if items, ok := domain["expected_architecture_outcomes"].([]interface{}); ok {
		more += "\nExpected architecture outcomes:\n"
		for _, item := range items {
			if s, ok := item.(string); ok {
				more += fmt.Sprintf("* %s\n", s)
			}
		}
	}
	if items, ok := domain["produced_artifacts"].([]interface{}); ok {
		more += "\nDetailed expectations:\n"
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				purpose := m["purpose"].(string)
				if !strings.HasSuffix(purpose, ".") {
					purpose += "."
				}
				more += fmt.Sprintf("* %s: %s %s\n", m["artifact_name"], purpose, m["expected_content"])
			}
		}
	}
	if items, ok := domain["constraints"].([]interface{}); ok {
		more += "\nConstraints:\n"
		for _, item := range items {
			if s, ok := item.(string); ok {
				more += fmt.Sprintf("* %s\n", s)
			}
		}
	}
	if items, ok := domain["consumed_artifacts"].([]interface{}); ok && len(items) > 0 {
		more += "\nKnowledge REQUIRED to build context:\n"
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				more += fmt.Sprintf("* %s: %s\n", m["artifact_name"], m["purpose"])
			}
		}
	}
	more += "\n"
	if text, ok := domain["responsibility"].(string); ok {
		more += fmt.Sprintf("Responsibility: %s\n", text)
	}
	if text, ok := domain["scope"].(string); ok {
		more += fmt.Sprintf("Scope: %s\n", text)
	}
	return spec, more
}

// RenderMarkdownContent renders markdown content for a given stage and content.
// Exported so tests can call it without duplicating rendering logic.
func RenderMarkdownContent(content interface{}, stageNameRaw string) string {
	stageNameClean := regexp.MustCompile(`[0-9]+$`).ReplaceAllString(stageNameRaw, "")
	markdownContent := ""

	if stageNameClean == "code_summary" {
		if s, ok := content.(string); ok {
			markdownContent = s
		}
	} else if stageNameClean == "product_manager_final" {
		actualContent, _ := content.(map[string]interface{})
		markdownContent += fmt.Sprintf("# Task Specification\n\n%s\n\n", actualContent["task_specification"])
		if files, ok := actualContent["files"].([]interface{}); ok && len(files) > 0 {
			markdownContent += "## Mentioned files\n\n"
			for _, f := range files {
				if s, ok := f.(string); ok {
					markdownContent += fmt.Sprintf("- %s\n\n", s)
				}
			}
		}
		if pn, ok := actualContent["proper_nouns"].([]interface{}); ok && len(pn) > 0 {
			markdownContent += "## Mentioned proper nouns\n\n"
			for _, p := range pn {
				if s, ok := p.(string); ok {
					markdownContent += fmt.Sprintf("- %s\n\n", s)
				}
			}
		}
		if facts, ok := actualContent["facts"].([]interface{}); ok && len(facts) > 0 {
			markdownContent += "## Stated facts\n\n"
			for _, f := range facts {
				if s, ok := f.(string); ok {
					markdownContent += fmt.Sprintf("- %s\n\n", s)
				}
			}
		}
		if mbnd, ok := actualContent["missing_but_necessary_details"].([]interface{}); ok && len(mbnd) > 0 {
			markdownContent += "## Additional considerations\n\n"
			for _, v := range mbnd {
				if s, ok := v.(string); ok {
					markdownContent += fmt.Sprintf("- %s\n\n", s)
				}
			}
		}
		if se, ok := actualContent["speculative_expansions"].([]interface{}); ok && len(se) > 0 {
			markdownContent += "## Out of scope\n\n"
			for _, v := range se {
				if s, ok := v.(string); ok {
					markdownContent += fmt.Sprintf("- %s\n\n", s)
				}
			}
		}
	} else if stageNameClean == "decomposition_final" {
		actualContent, _ := content.(map[string]interface{})
		decomp, _ := actualContent["decomposition"].(map[string]interface{})
		markdownContent += "# Decomposition\n\n"
		if domains, ok := decomp["domains"].([]interface{}); ok {
			markdownContent += "## Domains\n\n"
			for i, domain := range domains {
				if dm, ok := domain.(map[string]interface{}); ok {
					integrationOwnership, _ := decomp["integration_ownership"].([]interface{})
					if spec, extra := BuildArchitectInput(dm, integrationOwnership); spec != "" {
						markdownContent += fmt.Sprintf("### Domain %d\n\n", i+1)
						markdownContent += spec + extra + "\n\n"
					}
				}
			}
		}
	} else if stageNameClean == "architecture_after_reviews" {
		actualContent, _ := content.(map[string]interface{})
		arch, _ := actualContent["architecture"].(map[string]interface{})
		markdownContent += "# Architecture\n\n"
		markdownContent += "## Overview\n\n"
		markdownContent += fmt.Sprintf("%s\n\n", arch["overview"])
		if components, ok := arch["components"].([]interface{}); ok {
			markdownContent += "## Components\n\n"
			for _, comp := range components {
				if cm, ok := comp.(map[string]interface{}); ok {
					name, _ := cm["name"].(string)
					resp, _ := cm["responsibility"].(string)
					bg, _ := cm["background"].(string)
					if resp == "N/A" || bg == "N/A" {
						logStep("Missing fields in component dict: falling back to defaults", "MARKDOWN")
					}
					markdownContent += fmt.Sprintf("### %s\n\n", name)
					markdownContent += fmt.Sprintf("**Responsibility**: %s\n\n", resp)
					markdownContent += fmt.Sprintf("**Background**: %s\n\n", bg)
				}
			}
		}
		if dataFlow, ok := arch["data_flow"].([]interface{}); ok {
			markdownContent += "## Data Flow\n\n"
			for _, flow := range dataFlow {
				if s, ok := flow.(string); ok {
					markdownContent += fmt.Sprintf("- %s\n\n", s)
				}
			}
		}
		if techChoices, ok := arch["tech_choices"].([]interface{}); ok {
			markdownContent += "## Tech Choices\n\n"
			for _, tc := range techChoices {
				if s, ok := tc.(string); ok {
					markdownContent += fmt.Sprintf("- %s\n\n", s)
				}
			}
		}
		if constraints, ok := arch["constraints"].([]interface{}); ok {
			markdownContent += "## Constraints\n\n"
			for _, c := range constraints {
				if s, ok := c.(string); ok {
					markdownContent += fmt.Sprintf("- %s\n\n", s)
				}
			}
		}
	} else if stageNameClean == "tech_plan_after_reviews" {
		actualContent, _ := content.(map[string]interface{})
		plan, _ := actualContent["plan"].(map[string]interface{})
		markdownContent += "# Implementation plan\n\n"
		markdownContent += "## Summary\n\n"
		markdownContent += fmt.Sprintf("%s\n\n", plan["summary"])
		if files, ok := plan["files"].([]interface{}); ok {
			markdownContent += "## Files\n\n"
			for _, fi := range files {
				if fm, ok := fi.(map[string]interface{}); ok {
					path, _ := fm["path"].(string)
					purpose, _ := fm["purpose"].(string)
					background, _ := fm["background"].(string)
					markdownContent += fmt.Sprintf("### %s\n\n", path)
					markdownContent += fmt.Sprintf("**Purpose**: %s\n\n", purpose)
					markdownContent += fmt.Sprintf("**Background**: %s\n\n", background)
				}
			}
		}
		if steps, ok := plan["steps"].([]interface{}); ok {
			markdownContent += "## Steps\n\n"
			for _, step := range steps {
				if sm, ok := step.(map[string]interface{}); ok {
					idVal := ""
					if s, ok := sm["id"].(string); ok {
						idVal = s
					} else if f, ok := sm["id"].(float64); ok {
						idVal = fmt.Sprintf("%.0f", f)
					}
					desc, _ := sm["description"].(string)
					markdownContent += fmt.Sprintf("### Step %s\n\n", idVal)
					markdownContent += fmt.Sprintf("%s\n\n", desc)
				}
			}
		}
	} else if stageNameClean == "investigation_plan" {
		workstreams, _ := content.(map[string]interface{})["workstreams"].([]interface{})
		markdownContent += "# Investigation Plan\n\n"
		for i, ws := range workstreams {
			if wm, ok := ws.(map[string]interface{}); ok {
				markdownContent += fmt.Sprintf("## Workstream %d\n\n", i+1)
				if obj, ok := wm["objective"].(string); ok && obj != "" {
					markdownContent += fmt.Sprintf("%s\n\n", obj)
				}
				if ds, ok := wm["data_sources"].([]interface{}); ok && len(ds) > 0 {
					markdownContent += "Data Sources:\n"
					for _, d := range ds {
						if s, ok := d.(string); ok {
							markdownContent += fmt.Sprintf("* %s\n", s)
						}
					}
					markdownContent += "\n"
				}
				if hyp, ok := wm["hypotheses"].([]interface{}); ok && len(hyp) > 0 {
					markdownContent += "Hypotheses:\n"
					for _, h := range hyp {
						if s, ok := h.(string); ok {
							markdownContent += fmt.Sprintf("* %s\n", s)
						}
					}
					markdownContent += "\n"
				}
				if methods, ok := wm["investigation_methods"].([]interface{}); ok && len(methods) > 0 {
					markdownContent += "Investigation Methods:\n"
					for _, m := range methods {
						if s, ok := m.(string); ok {
							markdownContent += fmt.Sprintf("* %s\n", s)
						}
					}
					markdownContent += "\n"
				}
				if deliv, ok := wm["expected_deliverables"].([]interface{}); ok && len(deliv) > 0 {
					markdownContent += "Expected Deliverables:\n"
					for _, d := range deliv {
						if s, ok := d.(string); ok {
							markdownContent += fmt.Sprintf("* %s\n", s)
						}
					}
					markdownContent += "\n"
				}
			}
		}
	} else if strings.HasPrefix(stageNameClean, "investigation_workstream_") {
		markdownContent += "# Investigation Findings\n\n"
		if wm, ok := content.(map[string]interface{}); ok {
			if obj, ok := wm["workstream_objective"].(string); ok && obj != "" {
				markdownContent += fmt.Sprintf("%s\n\n", obj)
			}
			if conclusions, ok := wm["conclusions"].([]interface{}); ok && len(conclusions) > 0 {
				markdownContent += "Conclusions:\n"
				for _, c := range conclusions {
					if s, ok := c.(string); ok {
						markdownContent += fmt.Sprintf("* %s\n", s)
					}
				}
				markdownContent += "\n"
			}
			if evidence, ok := wm["supporting_evidence"].([]interface{}); ok && len(evidence) > 0 {
				markdownContent += "Supporting Evidence:\n"
				for _, item := range evidence {
					if em, ok := item.(map[string]interface{}); ok {
						eype, _ := em["evidence_type"].(string)
						edesc, _ := em["evidence_description"].(string)
						sref, _ := em["source_reference"].(string)
						markdownContent += fmt.Sprintf("* **[%s]** %s (ref: %s)\n", eype, edesc, sref)
					} else if s, ok := item.(string); ok {
						markdownContent += fmt.Sprintf("* %s\n", s)
					}
				}
				markdownContent += "\n"
			}
			if conf, ok := wm["confidence_level"].(string); ok && conf != "" {
				markdownContent += fmt.Sprintf("Confidence Level: %s\n\n", conf)
			}
			if unanswered, ok := wm["unanswered_questions"].([]interface{}); ok && len(unanswered) > 0 {
				markdownContent += "Unanswered Questions:\n"
				for _, q := range unanswered {
					if s, ok := q.(string); ok {
						markdownContent += fmt.Sprintf("* %s\n", s)
					}
				}
				markdownContent += "\n"
			}
		}
	} else if stageNameClean == "investigation_report_final" {
		if report, ok := content.(map[string]interface{}); ok {
			markdownContent += "# Investigation Report\n\n"
			if es, ok := report["executive_summary"].(string); ok && es != "" {
				markdownContent += fmt.Sprintf("%s\n\n", es)
			}
			if rca, ok := report["root_cause_analysis"].(map[string]interface{}); ok {
				markdownContent += "## Root Cause Analysis\n\n"
				if pc, ok := rca["primary_cause"].(string); ok && pc != "" {
					markdownContent += fmt.Sprintf("### Primary Cause\n\n%s\n\n", pc)
				}
				if cf, ok := rca["contributing_factors"].([]interface{}); ok && len(cf) > 0 {
					markdownContent += "### Contributing Factors\n\n"
					for _, f := range cf {
						if s, ok := f.(string); ok {
							markdownContent += fmt.Sprintf("* %s\n", s)
						}
					}
					markdownContent += "\n"
				}
				if et, ok := rca["evidence_trail"].([]interface{}); ok && len(et) > 0 {
					markdownContent += "### Evidence Trail\n\n"
					for _, item := range et {
						if s, ok := item.(string); ok {
							markdownContent += fmt.Sprintf("* %s\n", s)
						}
					}
					markdownContent += "\n"
				}
			}
			if tl, ok := report["timeline_reconstruction"].([]interface{}); ok {
				markdownContent += "## Timeline Reconstruction\n\n"
				for _, entry := range tl {
					if em, ok := entry.(map[string]interface{}); ok {
						ts, _ := em["timestamp"].(string)
						ev, _ := em["event"].(string)
						markdownContent += fmt.Sprintf("### %s\n\n%s\n\n", ts, ev)
					}
				}
			}
			if ci, ok := report["customer_impact_assessment"].(map[string]interface{}); ok {
				markdownContent += "## Customer Impact Assessment\n\n"
				if au, ok := ci["affected_users"].(string); ok && au != "" {
					markdownContent += fmt.Sprintf("### Affected Users\n\n%s\n\n", au)
				}
				if sev, ok := ci["severity"].(string); ok && sev != "" {
					markdownContent += fmt.Sprintf("### Severity\n\n%s\n\n", sev)
				}
				if dur, ok := ci["duration"].(string); ok && dur != "" {
					markdownContent += fmt.Sprintf("### Duration\n\n%s\n\n", dur)
				}
				if root, ok := ci["root_cause"].(string); ok && root != "" {
					markdownContent += fmt.Sprintf("### Root Cause\n\n%s\n\n", root)
				}
				if rec, ok := ci["remediation_steps"].([]interface{}); ok && len(rec) > 0 {
					markdownContent += "### Remediation Steps\n\n"
					for _, r := range rec {
						if s, ok := r.(string); ok {
							markdownContent += fmt.Sprintf("* %s\n", s)
						}
					}
					markdownContent += "\n"
				}
			}
			if corr, ok := report["correlation_findings"].([]interface{}); ok {
				markdownContent += "## Correlation Findings\n\n"
				for _, item := range corr {
					if im, ok := item.(map[string]interface{}); ok {
						obs, _ := im["observation"].(string)
						strength, _ := im["correlation_strength"].(string)
						causal, _ := im["causal_claim"].(string)
						markdownContent += fmt.Sprintf("* **Observation**: %s\n", obs)
						if strength != "" {
							markdownContent += fmt.Sprintf("  **Strength**: %s\n", strength)
						}
						if causal != "" {
							markdownContent += fmt.Sprintf("  **Causal Claim**: %s\n", causal)
						}
						markdownContent += "\n"
					}
				}
			}
			if hr, ok := report["hypothesis_test_results"].([]interface{}); ok {
				markdownContent += "## Hypothesis Test Results\n\n"
				for _, item := range hr {
					if im, ok := item.(map[string]interface{}); ok {
						hyp, _ := im["hypothesis"].(string)
						test, _ := im["test_performed"].(string)
						result, _ := im["result"].(string)
						conclusion, _ := im["conclusion"].(string)
						markdownContent += fmt.Sprintf("* **Hypothesis**: %s\n", hyp)
						if test != "" {
							markdownContent += fmt.Sprintf("  **Test**: %s\n", test)
						}
						if result != "" {
							markdownContent += fmt.Sprintf("  **Result**: %s\n", result)
						}
						if conclusion != "" {
							markdownContent += fmt.Sprintf("  **Conclusion**: %s\n", conclusion)
						}
						markdownContent += "\n"
					}
				}
			}
			if gaps, ok := report["known_gaps_and_unknowns"].([]interface{}); ok {
				markdownContent += "## Known Gaps and Unknowns\n\n"
				for _, g := range gaps {
					if s, ok := g.(string); ok {
						markdownContent += fmt.Sprintf("* %s\n", s)
					}
				}
				markdownContent += "\n"
			}
			if recs, ok := report["recommendations"].([]interface{}); ok {
				markdownContent += "## Recommendations\n\n"
				for _, item := range recs {
					if im, ok := item.(map[string]interface{}); ok {
						priority, _ := im["priority"].(string)
						action, _ := im["action"].(string)
						rationale, _ := im["rationale"].(string)
						markdownContent += fmt.Sprintf("* **[%s]** %s", priority, action)
						if rationale != "" {
							markdownContent += fmt.Sprintf(" — %s", rationale)
						}
						markdownContent += "\n"
					} else if s, ok := item.(string); ok {
						markdownContent += fmt.Sprintf("* %s\n", s)
					}
				}
				markdownContent += "\n"
			}
		}
	} else if stageNameClean == "investigation_classification" {
		if class, ok := content.(map[string]interface{}); ok {
			markdownContent += "# Investigation Classification\n\n"
			tt, _ := class["type"].(string)
			reasoning, _ := class["reasoning"].(string)
			markdownContent += fmt.Sprintf("**Type**: %s\n\n", tt)
			if reasoning != "" {
				markdownContent += fmt.Sprintf("**Reasoning**: %s\n\n", reasoning)
			}
		}
	}
	return markdownContent
}

// writeMarkdownDocument writes the rendered markdown content to a file in the
// document_stores directory. It returns the path to the written file.
func writeMarkdownDocument(stageName, markdownContent string, subdir []string) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_%s.md", timestamp, stageName)
	targetDir := filepath.Join(BuildPath(subdir, "document_stores"))
	if _, err := os.Stat(targetDir); err == nil {
		if entries, _ := os.ReadDir(targetDir); len(entries) > 0 {
			for _, f := range entries {
				if strings.HasSuffix(f.Name(), fmt.Sprintf("_%s.md", stageName)) {
					logStep(fmt.Sprintf("%s already exists", filepath.Join(targetDir, f.Name())), "MARKDOWN")
					return filepath.Join(targetDir, f.Name())
				}
			}
		}
	}
	targetFilepath := filepath.Join(targetDir, filename)

	if targetDir != "" {
		os.MkdirAll(targetDir, 0o755)
	}
	logStep(fmt.Sprintf("Writing %s...", targetFilepath), "MARKDOWN")
	os.WriteFile(targetFilepath, []byte(markdownContent), 0o644)
	return targetFilepath
}

func MarkdownDocumentGenerator(content interface{}, stageNameRaw string, subdir []string) string {
	stageName := regexp.MustCompile(`[0-9]+$`).ReplaceAllString(stageNameRaw, "")
	markdownContent := RenderMarkdownContent(content, stageNameRaw)
	trace("write_markdown_doc", map[string]interface{}{
		"content":    markdownContent,
		"stage_name": stageName,
	})
	if markdownDocHook != nil {
		return markdownDocHook(content, stageName, subdir)
	}
	return writeMarkdownDocument(stageName, markdownContent, subdir)
}
