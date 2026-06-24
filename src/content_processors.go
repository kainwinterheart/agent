package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	dt "agent-go/gen"
)

var markdownDocHook func(content interface{}, stageName string, subdir []string) string

func BuildArchitectInput(domain dt.SystemDecompositionJsondecompositiondomainsElem, integrationOwnership []dt.SystemDecompositionJsondecompositionintegrationownershipElem) (string, string) {
	spec := domain.DomainSpecification()
	if spec == "" {
		return "", ""
	}
	if !strings.HasSuffix(spec, "\n") {
		spec += "\n"
	}
	domainID := strings.ToLower(strings.TrimSpace(domain.Id()))
	var integrations []dt.SystemDecompositionJsondecompositionintegrationownershipElem
	for _, item := range integrationOwnership {
		ownerID := strings.ToLower(strings.TrimSpace(item.OwnerDomainId()))
		if ownerID == domainID {
			integrations = append(integrations, item)
		}
	}
	if len(integrations) > 0 {
		spec += "\nAdditionally, you are EXPECTED TO HANDLE integration of the following capabilities into the overall system:\n"
		for i, item := range integrations {
			cap := item.Capability()
			suffix := ""
			if len(item.IntegrationArtifacts()) > 0 {
				suffix = ":"
			}
			spec += fmt.Sprintf("%d. %s%s\n", i+1, cap, suffix)
			for j, sub := range item.IntegrationArtifacts() {
				spec += fmt.Sprintf("\t%d. %s\n", j+1, sub)
			}
		}
	}
	more := ""
	if len(domain.ExpectedArchitectureOutcomes()) > 0 {
		more += "\nExpected architecture outcomes:\n"
		for _, s := range domain.ExpectedArchitectureOutcomes() {
			more += fmt.Sprintf("* %s\n", s)
		}
	}
	if len(domain.ProducedArtifacts()) > 0 {
		more += "\nDetailed expectations:\n"
		for _, item := range domain.ProducedArtifacts() {
			purpose := item.Purpose()
			if !strings.HasSuffix(purpose, ".") {
				purpose += "."
			}
			more += fmt.Sprintf("* %s: %s %s\n", item.ArtifactName(), purpose, item.ExpectedContent())
		}
	}
	if len(domain.Constraints()) > 0 {
		more += "\nConstraints:\n"
		for _, s := range domain.Constraints() {
			more += fmt.Sprintf("* %s\n", s)
		}
	}
	if len(domain.ConsumedArtifacts()) > 0 {
		more += "\nKnowledge REQUIRED to build context:\n"
		for _, item := range domain.ConsumedArtifacts() {
			more += fmt.Sprintf("* %s: %s\n", item.ArtifactName(), item.Purpose())
		}
	}
	more += "\n"
	if domain.Responsibility() != "" {
		more += fmt.Sprintf("Responsibility: %s\n", domain.Responsibility())
	}
	if domain.Scope() != "" {
		more += fmt.Sprintf("Scope: %s\n", domain.Scope())
	}
	return spec, more
}

func RenderMarkdownContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v

	case *dt.PmSynthesizerJson:
		return renderPmSynthesizer(v)

	case dt.PmSynthesizerJson:
		return renderPmSynthesizer(&v)

	case *dt.SystemDecompositionJson:
		return renderDecomposition(v)

	case dt.SystemDecompositionJson:
		return renderDecomposition(&v)

	case *dt.ArchJson:
		return renderArchitecture(v)

	case dt.ArchJson:
		return renderArchitecture(&v)

	case *dt.PlanJson:
		return renderPlan(v)

	case dt.PlanJson:
		return renderPlan(&v)

	case *dt.InvestigatorFindingsJson:
		return renderInvestigatorFindings(v)

	case dt.InvestigatorFindingsJson:
		return renderInvestigatorFindings(&v)

	case *dt.InvestigationReportJson:
		return renderInvestigationReport(v)

	case dt.InvestigationReportJson:
		return renderInvestigationReport(&v)

	case *dt.InvestigationClassifierJson:
		return renderInvestigationClassifier(v)

	case dt.InvestigationClassifierJson:
		return renderInvestigationClassifier(&v)

	case *dt.InvestigatorPlanJson:
		return renderInvestigatorPlan(v)

	case dt.InvestigatorPlanJson:
		return renderInvestigatorPlan(&v)

	default:
		return ""
	}
}

func renderPmSynthesizer(ps *dt.PmSynthesizerJson) string {
	var b strings.Builder
	b.WriteString("# Task Specification\n\n")
	if ts := ps.TaskSpecification(); ts != "" {
		b.WriteString(ts)
		if !strings.HasSuffix(ts, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if files := ps.Files(); len(files) > 0 {
		b.WriteString("## Mentioned files\n\n")
		for _, f := range files {
			b.WriteString(fmt.Sprintf("- %s\n\n", f))
		}
	}
	if pn := ps.ProperNouns(); len(pn) > 0 {
		b.WriteString("## Mentioned proper nouns\n\n")
		for _, p := range pn {
			b.WriteString(fmt.Sprintf("- %s\n\n", p))
		}
	}
	if facts := ps.Facts(); len(facts) > 0 {
		b.WriteString("## Stated facts\n\n")
		for _, f := range facts {
			b.WriteString(fmt.Sprintf("- %s\n\n", f))
		}
	}
	if mbnd := ps.MissingButNecessaryDetails(); len(mbnd) > 0 {
		b.WriteString("## Additional considerations\n\n")
		for _, v := range mbnd {
			b.WriteString(fmt.Sprintf("- %s\n\n", v))
		}
	}
	if se := ps.SpeculativeExpansions(); len(se) > 0 {
		b.WriteString("## Out of scope\n\n")
		for _, v := range se {
			b.WriteString(fmt.Sprintf("- %s\n\n", v))
		}
	}
	return b.String()
}

func renderDecomposition(decomp *dt.SystemDecompositionJson) string {
	var b strings.Builder
	b.WriteString("# Decomposition\n\n")
	decompVal := decomp.Decomposition()
	if len(decompVal.Domains()) > 0 {
		b.WriteString("## Domains\n\n")
		for i, domain := range decompVal.Domains() {
			if spec, extra := BuildArchitectInput(domain, decompVal.IntegrationOwnership()); spec != "" {
				b.WriteString(fmt.Sprintf("### Domain %d\n\n", i+1))
				b.WriteString(spec)
				if extra != "" {
					b.WriteString(extra)
				}
				b.WriteString("\n\n")
			}
		}
	}
	return b.String()
}

func renderArchitecture(arch *dt.ArchJson) string {
	var b strings.Builder
	b.WriteString("# Architecture\n\n")

	archInner := arch.Architecture()

	if overview := archInner.Overview(); overview != "" {
		b.WriteString("## Overview\n\n")
		b.WriteString(overview)
		b.WriteString("\n\n")
	}

	if comps := archInner.Components(); len(comps) > 0 {
		b.WriteString("## Components\n\n")
		for _, c := range comps {
			name := c.Name()
			b.WriteString(fmt.Sprintf("### %s\n\n", name))
			if resp := c.Responsibility(); resp != "" {
				b.WriteString(fmt.Sprintf("**Responsibility**: %s\n\n", resp))
			}
			if bg := c.Background(); bg != "" {
				b.WriteString(fmt.Sprintf("**Background**: %s\n\n", bg))
			}
		}
	}

	if dataFlow := archInner.DataFlow(); len(dataFlow) > 0 {
		b.WriteString("## Data Flow\n\n")
		for _, df := range dataFlow {
			b.WriteString(fmt.Sprintf("- %s\n\n", df))
		}
	}

	if techChoices := archInner.TechChoices(); len(techChoices) > 0 {
		b.WriteString("## Tech Choices\n\n")
		for _, tc := range techChoices {
			b.WriteString(fmt.Sprintf("- %s\n\n", tc))
		}
	}

	if constraints := archInner.Constraints(); len(constraints) > 0 {
		b.WriteString("## Constraints\n\n")
		for _, c := range constraints {
			b.WriteString(fmt.Sprintf("- %s\n\n", c))
		}
	}

	return b.String()
}

func renderPlan(plan *dt.PlanJson) string {
	var b strings.Builder
	b.WriteString("# Implementation plan\n\n")

	planInner := plan.Plan()

	if summary := planInner.Summary(); summary != "" {
		b.WriteString("## Summary\n\n")
		b.WriteString(summary)
		if !strings.HasSuffix(summary, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if files := planInner.Files(); len(files) > 0 {
		b.WriteString("## Files\n\n")
		for _, f := range files {
			b.WriteString(fmt.Sprintf("### %s\n\n", f.Path()))
			if purpose := f.Purpose(); purpose != "" {
				b.WriteString(fmt.Sprintf("**Purpose**: %s\n\n", purpose))
			}
			if bg := f.Background(); bg != "" {
				b.WriteString(fmt.Sprintf("**Background**: %s\n\n", bg))
			}
		}
	}

	if steps := planInner.Steps(); len(steps) > 0 {
		b.WriteString("## Steps\n\n")
		for _, s := range steps {
			b.WriteString(fmt.Sprintf("### Step %d\n\n", s.Id()))
			b.WriteString(s.Description())
			if !strings.HasSuffix(s.Description(), "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func renderInvestigatorFindings(findings *dt.InvestigatorFindingsJson) string {
	var b strings.Builder
	b.WriteString("# Investigation Findings\n\n")

	if wo := findings.WorkstreamObjective(); wo != "" {
		b.WriteString(wo)
		b.WriteString("\n\n")
	}

	if concs := findings.Conclusions(); len(concs) > 0 {
		b.WriteString("Conclusions:\n")
		for _, c := range concs {
			b.WriteString(fmt.Sprintf("* %s\n", c))
		}
		b.WriteString("\n")
	}

	if evs := findings.SupportingEvidence(); len(evs) > 0 {
		b.WriteString("Supporting Evidence:\n")
		for _, e := range evs {
			et := e.EvidenceType()
			ed := e.EvidenceDescription()
			sr := e.SourceReference()
			b.WriteString(fmt.Sprintf("* **[%s]** %s", et, ed))
			if sr != "" {
				b.WriteString(fmt.Sprintf(" (ref: %s)", sr))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if cl := findings.ConfidenceLevel(); cl != "" {
		b.WriteString(fmt.Sprintf("Confidence Level: %s\n\n", cl))
	}

	if uq := findings.UnansweredQuestions(); len(uq) > 0 {
		b.WriteString("Unanswered Questions:\n")
		for _, q := range uq {
			b.WriteString(fmt.Sprintf("* %s\n", q))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderInvestigationReport(report *dt.InvestigationReportJson) string {
	var b strings.Builder

	if es := report.ExecutiveSummary(); es != "" {
		b.WriteString("# Investigation Report\n\n")
		b.WriteString(es)
		if !strings.HasSuffix(es, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if rca := report.RootCauseAnalysis(); true {
		pc := rca.PrimaryCause()
		cf := rca.ContributingFactors()
		et := rca.EvidenceTrail()
		if pc != "" || len(cf) > 0 || len(et) > 0 {
			b.WriteString("## Root Cause Analysis\n\n")
			if pc != "" {
				b.WriteString("### Primary Cause\n\n")
				b.WriteString(pc)
				b.WriteString("\n\n")
			}
			if len(cf) > 0 {
				b.WriteString("### Contributing Factors\n\n")
				for _, f := range cf {
					b.WriteString(fmt.Sprintf("* %s\n", f))
				}
				b.WriteString("\n")
			}
			if len(et) > 0 {
				b.WriteString("### Evidence Trail\n\n")
				for _, e := range et {
					b.WriteString(fmt.Sprintf("* %s\n", e))
				}
				b.WriteString("\n")
			}
		}
	}

	if tl := report.TimelineReconstruction(); len(tl) > 0 {
		b.WriteString("## Timeline Reconstruction\n\n")
		for _, t := range tl {
			b.WriteString(fmt.Sprintf("### %s\n\n", t.Timestamp()))
			b.WriteString(t.Event())
			if !strings.HasSuffix(t.Event(), "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	if ci := report.CustomerImpactAssessment(); true {
		affected := ci.AffectedUsers()
		severity := ci.Severity()
		duration := ci.Duration()
		if affected != "" || severity != "" || duration != "" {
			b.WriteString("## Customer Impact Assessment\n\n")
			if affected != "" {
				b.WriteString("### Affected Users\n\n")
				b.WriteString(affected)
				b.WriteString("\n\n")
			}
			if severity != "" {
				b.WriteString("### Severity\n\n")
				b.WriteString(severity)
				b.WriteString("\n\n")
			}
			if duration != "" {
				b.WriteString("### Duration\n\n")
				b.WriteString(duration)
				b.WriteString("\n\n")
			}
		}
	}

	if corr := report.CorrelationFindings(); len(corr) > 0 {
		b.WriteString("## Correlation Findings\n\n")
		for _, item := range corr {
			obs := item.Observation()
			strength := item.CorrelationStrength()
			causal := item.CausalClaim()
			b.WriteString(fmt.Sprintf("* **Observation**: %s\n", obs))
			if strength != "" {
				b.WriteString(fmt.Sprintf("  **Strength**: %s\n", strength))
			}
			if causal != "" {
				b.WriteString(fmt.Sprintf("  **Causal Claim**: %s\n", causal))
			}
			b.WriteString("\n")
		}
	}

	if hr := report.HypothesisTestResults(); len(hr) > 0 {
		b.WriteString("## Hypothesis Test Results\n\n")
		for _, item := range hr {
			hyp := item.Hypothesis()
			test := item.TestPerformed()
			result := item.Result()
			conclusion := item.Conclusion()
			b.WriteString(fmt.Sprintf("* **Hypothesis**: %s\n", hyp))
			if test != "" {
				b.WriteString(fmt.Sprintf("  **Test**: %s\n", test))
			}
			if result != "" {
				b.WriteString(fmt.Sprintf("  **Result**: %s\n", result))
			}
			if conclusion != "" {
				b.WriteString(fmt.Sprintf("  **Conclusion**: %s\n", conclusion))
			}
			b.WriteString("\n")
		}
	}

	if gaps := report.KnownGapsAndUnknowns(); len(gaps) > 0 {
		b.WriteString("## Known Gaps and Unknowns\n\n")
		for _, g := range gaps {
			b.WriteString(fmt.Sprintf("* %s\n", g))
		}
		b.WriteString("\n")
	}

	if recs := report.Recommendations(); len(recs) > 0 {
		b.WriteString("## Recommendations\n\n")
		for _, item := range recs {
			priority := item.Priority()
			action := item.Action()
			rationale := item.Rationale()
			b.WriteString(fmt.Sprintf("* **[%s]** %s", priority, action))
			if rationale != "" {
				b.WriteString(fmt.Sprintf(" — %s", rationale))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderInvestigationClassifier(class *dt.InvestigationClassifierJson) string {
	var b strings.Builder
	b.WriteString("# Investigation Classification\n\n")
	b.WriteString(fmt.Sprintf("**Type**: %s\n\n", class.AType()))
	if reasoning := class.Reasoning(); reasoning != "" {
		b.WriteString(fmt.Sprintf("**Reasoning**: %s\n\n", reasoning))
	}
	return b.String()
}

func renderInvestigatorPlan(plan *dt.InvestigatorPlanJson) string {
	var b strings.Builder
	b.WriteString("# Investigation Plan\n\n")
	workstreams := plan.Workstreams()
	for i, ws := range workstreams {
		b.WriteString(fmt.Sprintf("## Workstream %d\n\n", i+1))
		if obj := ws.Objective(); obj != "" {
			b.WriteString(obj + "\n\n")
		}
		if ds := ws.DataSources(); len(ds) > 0 {
			b.WriteString("Data Sources:\n")
			for _, s := range ds {
				b.WriteString(fmt.Sprintf("* %s\n", s))
			}
			b.WriteString("\n")
		}
		if hyps := ws.Hypotheses(); len(hyps) > 0 {
			b.WriteString("Hypotheses:\n")
			for _, h := range hyps {
				b.WriteString(fmt.Sprintf("* %s\n", h))
			}
			b.WriteString("\n")
		}
		if methods := ws.InvestigationMethods(); len(methods) > 0 {
			b.WriteString("Investigation Methods:\n")
			for _, m := range methods {
				b.WriteString(fmt.Sprintf("* %s\n", m))
			}
			b.WriteString("\n")
		}
		if delivs := ws.ExpectedDeliverables(); len(delivs) > 0 {
			b.WriteString("Expected Deliverables:\n")
			for _, d := range delivs {
				b.WriteString(fmt.Sprintf("* %s\n", d))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

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

func MarkdownDocumentGenerator(content interface{}, stageName string, subdir []string) string {
	markdownContent := RenderMarkdownContent(content)
	trace("write_markdown_doc", map[string]interface{}{
		"content":    markdownContent,
		"stage_name": regexp.MustCompile(`[0-9]+$`).ReplaceAllString(stageName, ""),
	})
	if markdownDocHook != nil {
		return markdownDocHook(content, stageName, subdir)
	}
	return writeMarkdownDocument(stageName, markdownContent, subdir)
}
