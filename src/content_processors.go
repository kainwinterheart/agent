package main

import (
	"fmt"
	"github.com/benbjohnson/immutable"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	dt "agent-go/gen"
	td "agent-go/test_data"
)

func BuildArchitectInput(domain dt.SystemDecompositiondecompositiondomainsElem, integrationOwnership []dt.SystemDecompositiondecompositionintegrationownershipElem) (string, string) {
	spec := domain.DomainSpecification()
	if spec == "" {
		return "", ""
	}
	if !strings.HasSuffix(spec, "\n") {
		spec += "\n"
	}
	domainID := strings.ToLower(strings.TrimSpace(domain.Id()))
	var integrations []dt.SystemDecompositiondecompositionintegrationownershipElem
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
			if item.IntegrationArtifacts().Len() > 0 {
				suffix = ":"
			}
			spec += fmt.Sprintf("%d. %s%s\n", i+1, cap, suffix)
			var sub string
			subItr := item.IntegrationArtifacts().Iterator()
			j := 0
			subItr.First()
			for !subItr.Done() {
				_, sub = subItr.Next()
				spec += fmt.Sprintf("\t%d. %s\n", j+1, sub)
				j++
			}
		}
	}
	more := ""
	if domain.ExpectedArchitectureOutcomes().Len() > 0 {
		more += "\nExpected architecture outcomes:\n"
		var s string
		oaItr := domain.ExpectedArchitectureOutcomes().Iterator()
		oaItr.First()
		for !oaItr.Done() {
			_, s = oaItr.Next()
			more += fmt.Sprintf("* %s\n", s)
		}
	}
	if domain.ProducedArtifacts().Len() > 0 {
		more += "\nDetailed expectations:\n"
		var item dt.SystemDecompositiondecompositiondomainsElemproducedartifactsElem
		paItr := domain.ProducedArtifacts().Iterator()
		paItr.First()
		for !paItr.Done() {
			_, item = paItr.Next()
			purpose := item.Purpose()
			if !strings.HasSuffix(purpose, ".") {
				purpose += "."
			}
			more += fmt.Sprintf("* %s: %s %s\n", item.ArtifactName(), purpose, item.ExpectedContent())
		}
	}
	if domain.Constraints().Len() > 0 {
		more += "\nConstraints:\n"
		var s string
		cItr := domain.Constraints().Iterator()
		cItr.First()
		for !cItr.Done() {
			_, s = cItr.Next()
			more += fmt.Sprintf("* %s\n", s)
		}
	}
	if domain.ConsumedArtifacts().Len() > 0 {
		more += "\nKnowledge REQUIRED to build context:\n"
		var item dt.SystemDecompositiondecompositiondomainsElemconsumedartifactsElem
		caItr := domain.ConsumedArtifacts().Iterator()
		caItr.First()
		for !caItr.Done() {
			_, item = caItr.Next()
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

	case *dt.PmSynthesizer:
		return renderPmSynthesizer(v)

	case dt.PmSynthesizer:
		return renderPmSynthesizer(&v)

	case *dt.SystemDecomposition:
		return renderDecomposition(v)

	case dt.SystemDecomposition:
		return renderDecomposition(&v)

	case *dt.Arch:
		return renderArchitecture(v)

	case dt.Arch:
		return renderArchitecture(&v)

	case *dt.Plan:
		return renderPlan(v)

	case dt.Plan:
		return renderPlan(&v)

	case *dt.InvestigatorFindings:
		return renderInvestigatorFindings(v)

	case dt.InvestigatorFindings:
		return renderInvestigatorFindings(&v)

	case *dt.InvestigationReport:
		return renderInvestigationReport(v)

	case dt.InvestigationReport:
		return renderInvestigationReport(&v)

	case *dt.InvestigationClassifier:
		return renderInvestigationClassifier(v)

	case dt.InvestigationClassifier:
		return renderInvestigationClassifier(&v)

	case *dt.InvestigatorPlan:
		return renderInvestigatorPlan(v)

	case dt.InvestigatorPlan:
		return renderInvestigatorPlan(&v)

	default:
		return ""
	}
}

func renderPmSynthesizer(ps *dt.PmSynthesizer) string {
	var b strings.Builder
	b.WriteString("# Task Specification\n\n")
	if ts := ps.TaskSpecification(); ts != "" {
		b.WriteString(ts)
		if !strings.HasSuffix(ts, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if files := ps.Files(); files.Len() > 0 {
		b.WriteString("## Mentioned files\n\n")
		var f string
		filesItr := files.Iterator()
		filesItr.First()
		for !filesItr.Done() {
			_, f = filesItr.Next()
			b.WriteString(fmt.Sprintf("- %s\n\n", f))
		}
	}
	if pn := ps.ProperNouns(); pn.Len() > 0 {
		b.WriteString("## Mentioned proper nouns\n\n")
		var p string
		pnItr := pn.Iterator()
		pnItr.First()
		for !pnItr.Done() {
			_, p = pnItr.Next()
			b.WriteString(fmt.Sprintf("- %s\n\n", p))
		}
	}
	if facts := ps.Facts(); facts.Len() > 0 {
		b.WriteString("## Stated facts\n\n")
		var f string
		factsItr := facts.Iterator()
		factsItr.First()
		for !factsItr.Done() {
			_, f = factsItr.Next()
			b.WriteString(fmt.Sprintf("- %s\n\n", f))
		}
	}
	if mbnd := ps.MissingButNecessaryDetails(); mbnd.Len() > 0 {
		b.WriteString("## Additional considerations\n\n")
		var v string
		mbndItr := mbnd.Iterator()
		mbndItr.First()
		for !mbndItr.Done() {
			_, v = mbndItr.Next()
			b.WriteString(fmt.Sprintf("- %s\n\n", v))
		}
	}
	if se := ps.SpeculativeExpansions(); se.Len() > 0 {
		b.WriteString("## Out of scope\n\n")
		var v string
		seItr := se.Iterator()
		seItr.First()
		for !seItr.Done() {
			_, v = seItr.Next()
			b.WriteString(fmt.Sprintf("- %s\n\n", v))
		}
	}
	return b.String()
}

func immutableListToSlice[T any](list *immutable.List[T]) []T {
	if list == nil {
		return []T{}
	}
	result := make([]T, 0, list.Len())
	itr := list.Iterator()
	itr.First()
	for !itr.Done() {
		_, v := itr.Next()
		result = append(result, v)
	}
	return result
}

func renderDecomposition(decomp *dt.SystemDecomposition) string {
	var b strings.Builder
	b.WriteString("# Decomposition\n\n")
	decompVal := decomp.Decomposition()
	if decompVal.Domains().Len() > 0 {
		b.WriteString("## Domains\n\n")
		domItr := decompVal.Domains().Iterator()
		i := 0
		domItr.First()
		var domain dt.SystemDecompositiondecompositiondomainsElem
		for !domItr.Done() {
			_, domain = domItr.Next()
			if spec, extra := BuildArchitectInput(domain, immutableListToSlice(decompVal.IntegrationOwnership())); spec != "" {
				b.WriteString(fmt.Sprintf("### Domain %d\n\n", i+1))
				b.WriteString(spec)
				if extra != "" {
					b.WriteString(extra)
				}
				b.WriteString("\n\n")
			}
			i++
		}
	}
	return b.String()
}

func renderArchitecture(arch *dt.Arch) string {
	var b strings.Builder
	b.WriteString("# Architecture\n\n")

	archInner := arch.Architecture()

	if overview := archInner.Overview(); overview != "" {
		b.WriteString("## Overview\n\n")
		b.WriteString(overview)
		b.WriteString("\n\n")
	}

	if comps := archInner.Components(); comps.Len() > 0 {
		b.WriteString("## Components\n\n")
		compsItr := comps.Iterator()
		compsItr.First()
		var comp dt.ArcharchitecturecomponentsElem
		for !compsItr.Done() {
			_, comp = compsItr.Next()
			name := comp.Name()
			b.WriteString(fmt.Sprintf("### %s\n\n", name))
			if resp := comp.Responsibility(); resp != "" {
				b.WriteString(fmt.Sprintf("**Responsibility**: %s\n\n", resp))
			}
			if bg := comp.Background(); bg != "" {
				b.WriteString(fmt.Sprintf("**Background**: %s\n\n", bg))
			}
		}
	}

	if dataFlow := archInner.DataFlow(); dataFlow.Len() > 0 {
		b.WriteString("## Data Flow\n\n")
		var df string
		dataFlowItr := dataFlow.Iterator()
		dataFlowItr.First()
		for !dataFlowItr.Done() {
			_, df = dataFlowItr.Next()
			b.WriteString(fmt.Sprintf("- %s\n\n", df))
		}
	}

	if techChoices := archInner.TechChoices(); techChoices.Len() > 0 {
		b.WriteString("## Tech Choices\n\n")
		var tc string
		techChoicesItr := techChoices.Iterator()
		techChoicesItr.First()
		for !techChoicesItr.Done() {
			_, tc = techChoicesItr.Next()
			b.WriteString(fmt.Sprintf("- %s\n\n", tc))
		}
	}

	if constraints := archInner.Constraints(); constraints.Len() > 0 {
		b.WriteString("## Constraints\n\n")
		var con string
		constItr := constraints.Iterator()
		constItr.First()
		for !constItr.Done() {
			_, con = constItr.Next()
			b.WriteString(fmt.Sprintf("- %s\n\n", con))
		}
	}

	return b.String()
}

func renderPlan(plan *dt.Plan) string {
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

	if files := planInner.Files(); files.Len() > 0 {
		b.WriteString("## Files\n\n")
		var f dt.PlanplanfilesElem
		filesItr := files.Iterator()
		filesItr.First()
		for !filesItr.Done() {
			_, f = filesItr.Next()
			b.WriteString(fmt.Sprintf("### %s\n\n", f.Path()))
			if purpose := f.Purpose(); purpose != "" {
				b.WriteString(fmt.Sprintf("**Purpose**: %s\n\n", purpose))
			}
			if bg := f.Background(); bg != "" {
				b.WriteString(fmt.Sprintf("**Background**: %s\n\n", bg))
			}
		}
	}

	if steps := planInner.Steps(); steps.Len() > 0 {
		b.WriteString("## Steps\n\n")
		var s dt.PlanplanstepsElem
		stepsItr := steps.Iterator()
		stepsItr.First()
		for !stepsItr.Done() {
			_, s = stepsItr.Next()
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

func renderInvestigatorFindings(findings *dt.InvestigatorFindings) string {
	var b strings.Builder
	b.WriteString("# Investigation Findings\n\n")

	if wo := findings.WorkstreamObjective(); wo != "" {
		b.WriteString(wo)
		b.WriteString("\n\n")
	}

	if concs := findings.Conclusions(); concs.Len() > 0 {
		b.WriteString("Conclusions:\n")
		var c string
		concsItr := concs.Iterator()
		concsItr.First()
		for !concsItr.Done() {
			_, c = concsItr.Next()
			b.WriteString(fmt.Sprintf("* %s\n", c))
		}
		b.WriteString("\n")
	}

	if evs := findings.SupportingEvidence(); evs.Len() > 0 {
		b.WriteString("Supporting Evidence:\n")
		var e dt.InvestigatorFindingssupportingevidenceElem
		evsItr := evs.Iterator()
		evsItr.First()
		for !evsItr.Done() {
			_, e = evsItr.Next()
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

	if uq := findings.UnansweredQuestions(); uq.Len() > 0 {
		b.WriteString("Unanswered Questions:\n")
		var q string
		uqItr := uq.Iterator()
		uqItr.First()
		for !uqItr.Done() {
			_, q = uqItr.Next()
			b.WriteString(fmt.Sprintf("* %s\n", q))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderInvestigationReport(report *dt.InvestigationReport) string {
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
		if pc != "" || cf.Len() > 0 || et.Len() > 0 {
			b.WriteString("## Root Cause Analysis\n\n")
			if pc != "" {
				b.WriteString("### Primary Cause\n\n")
				b.WriteString(pc)
				b.WriteString("\n\n")
			}
			if cf.Len() > 0 {
				b.WriteString("### Contributing Factors\n\n")
				var f string
				cfItr := cf.Iterator()
				cfItr.First()
				for !cfItr.Done() {
					_, f = cfItr.Next()
					b.WriteString(fmt.Sprintf("* %s\n", f))
				}
				b.WriteString("\n")
			}
			if et.Len() > 0 {
				b.WriteString("### Evidence Trail\n\n")
				var e string
				etItr := et.Iterator()
				etItr.First()
				for !etItr.Done() {
					_, e = etItr.Next()
					b.WriteString(fmt.Sprintf("* %s\n", e))
				}
				b.WriteString("\n")
			}
		}
	}

	if tl := report.TimelineReconstruction(); tl.Len() > 0 {
		b.WriteString("## Timeline Reconstruction\n\n")
		var t dt.InvestigationReporttimelinereconstructionElem
		tlItr := tl.Iterator()
		tlItr.First()
		for !tlItr.Done() {
			_, t = tlItr.Next()
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

	if corr := report.CorrelationFindings(); corr.Len() > 0 {
		b.WriteString("## Correlation Findings\n\n")
		var item dt.InvestigationReportcorrelationfindingsElem
		corrItr := corr.Iterator()
		corrItr.First()
		for !corrItr.Done() {
			_, item = corrItr.Next()
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

	if hr := report.HypothesisTestResults(); hr.Len() > 0 {
		b.WriteString("## Hypothesis Test Results\n\n")
		var item dt.InvestigationReporthypothesistestresultsElem
		hrItr := hr.Iterator()
		hrItr.First()
		for !hrItr.Done() {
			_, item = hrItr.Next()
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

	if gaps := report.KnownGapsAndUnknowns(); gaps.Len() > 0 {
		b.WriteString("## Known Gaps and Unknowns\n\n")
		var g string
		gapsItr := gaps.Iterator()
		gapsItr.First()
		for !gapsItr.Done() {
			_, g = gapsItr.Next()
			b.WriteString(fmt.Sprintf("* %s\n", g))
		}
		b.WriteString("\n")
	}

	if recs := report.Recommendations(); recs.Len() > 0 {
		b.WriteString("## Recommendations\n\n")
		var item dt.InvestigationReportrecommendationsElem
		recsItr := recs.Iterator()
		recsItr.First()
		for !recsItr.Done() {
			_, item = recsItr.Next()
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

func renderInvestigationClassifier(class *dt.InvestigationClassifier) string {
	var b strings.Builder
	b.WriteString("# Investigation Classification\n\n")
	b.WriteString(fmt.Sprintf("**Type**: %s\n\n", class.AType()))
	if reasoning := class.Reasoning(); reasoning != "" {
		b.WriteString(fmt.Sprintf("**Reasoning**: %s\n\n", reasoning))
	}
	return b.String()
}

func renderInvestigatorPlan(plan *dt.InvestigatorPlan) string {
	var b strings.Builder
	b.WriteString("# Investigation Plan\n\n")
	workstreams := plan.Workstreams()
	var ws dt.InvestigatorPlanworkstreamsElem
	wsItr := workstreams.Iterator()
	i := 0
	wsItr.First()
	for !wsItr.Done() {
		_, ws = wsItr.Next()
		i++
		b.WriteString(fmt.Sprintf("## Workstream %d\n\n", i))
		if obj := ws.Objective(); obj != "" {
			b.WriteString(obj + "\n\n")
		}
		if ds := ws.DataSources(); ds.Len() > 0 {
			b.WriteString("Data Sources:\n")
			var s string
			dsItr := ds.Iterator()
			dsItr.First()
			for !dsItr.Done() {
				_, s = dsItr.Next()
				b.WriteString(fmt.Sprintf("* %s\n", s))
			}
			b.WriteString("\n")
		}
		if hyps := ws.Hypotheses(); hyps.Len() > 0 {
			b.WriteString("Hypotheses:\n")
			var h string
			hypsItr := hyps.Iterator()
			hypsItr.First()
			for !hypsItr.Done() {
				_, h = hypsItr.Next()
				b.WriteString(fmt.Sprintf("* %s\n", h))
			}
			b.WriteString("\n")
		}
		if methods := ws.InvestigationMethods(); methods.Len() > 0 {
			b.WriteString("Investigation Methods:\n")
			var m string
			methodsItr := methods.Iterator()
			methodsItr.First()
			for !methodsItr.Done() {
				_, m = methodsItr.Next()
				b.WriteString(fmt.Sprintf("* %s\n", m))
			}
			b.WriteString("\n")
		}
		if delivs := ws.ExpectedDeliverables(); delivs.Len() > 0 {
			b.WriteString("Expected Deliverables:\n")
			var d string
			delivsItr := delivs.Iterator()
			delivsItr.First()
			for !delivsItr.Done() {
				_, d = delivsItr.Next()
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

func MarkdownDocumentGenerator(content interface{}, stageName string, subdir []string, context *Context) string {
	markdownContent := RenderMarkdownContent(content)
	stageNameClean := regexp.MustCompile(`[0-9]+$`).ReplaceAllString(stageName, "")
	context.Tracer.trace("write_markdown_doc", td.NewActionDetailsBuilder(nil).WithContent(&markdownContent).WithStageName(&stageNameClean).Build())
	if context.markdownDocHook != nil {
		return context.markdownDocHook(content, stageName, subdir)
	}
	return writeMarkdownDocument(stageName, markdownContent, subdir)
}
