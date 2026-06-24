package state

import dt "agent-go/gen"

type PayloadData struct {
	revisionPrompt       string
	docSuffix            string
	revisionInvPrefix    string
	initialPromptContext string

	codeReview                     dt.CodeReviewJson
	decompositionReview            dt.SystemDecompositionReviewJson
	investigationPlanQualityReview dt.InvestigationPlanQualityReviewJson
	gapAnalysisReview              dt.GapAnalysisReviewJson
	archReview                     dt.ArchReviewJson
	planReview                     dt.PlanReviewJson
	archFinal                      dt.ArchFinalJson
	factReview                     dt.FactCheckingReviewJson
	consistencyReview              dt.SynthesisConsistencyReviewJson

	techLeadFinalReview    *dt.TechLeadFinalJson
	hasTechLeadFinalReview bool

	investigationReport  *dt.InvestigationReportJson
	investigatorFindings *dt.InvestigatorFindingsJson
	findingsList         []dt.InvestigatorFindingsJson

	coderOutputs []dt.CoderJson
	allChanges   map[string]string
	iterCount    int

	workstreamElem dt.InvestigatorPlanJsonworkstreamsElem

	tlIter int
}

func (p *PayloadData) RevisionPrompt() string { return p.revisionPrompt }

func (p *PayloadData) DocSuffix() string { return p.docSuffix }

func (p *PayloadData) RevisionInvPrefix() string { return p.revisionInvPrefix }

func (p *PayloadData) InitialPromptContext() string { return p.initialPromptContext }

func (p *PayloadData) CodeReview() *dt.CodeReviewJson { return &p.codeReview }

func (p *PayloadData) DecompositionReview() *dt.SystemDecompositionReviewJson {
	return &p.decompositionReview
}

func (p *PayloadData) InvestigationPlanQualityReview() *dt.InvestigationPlanQualityReviewJson {
	return &p.investigationPlanQualityReview
}

func (p *PayloadData) GapAnalysisReview() *dt.GapAnalysisReviewJson { return &p.gapAnalysisReview }

func (p *PayloadData) ArchReview() *dt.ArchReviewJson { return &p.archReview }

func (p *PayloadData) PlanReview() *dt.PlanReviewJson { return &p.planReview }

func (p *PayloadData) ArchFinal() *dt.ArchFinalJson { return &p.archFinal }

func (p *PayloadData) TechLeadFinalReview() *dt.TechLeadFinalJson { return p.techLeadFinalReview }

func (p *PayloadData) HasTechLeadFinalReview() bool { return p.hasTechLeadFinalReview }

func (p *PayloadData) FactReview() *dt.FactCheckingReviewJson { return &p.factReview }

func (p *PayloadData) ConsistencyReview() *dt.SynthesisConsistencyReviewJson {
	return &p.consistencyReview
}

func (p *PayloadData) InvestigationReport() *dt.InvestigationReportJson {
	return p.investigationReport
}

func (p *PayloadData) InvestigatorFindings() *dt.InvestigatorFindingsJson {
	return p.investigatorFindings
}

func (p *PayloadData) FindingsList() []dt.InvestigatorFindingsJson {
	return p.findingsList
}

func (p *PayloadData) CoderOutputs() []dt.CoderJson { return p.coderOutputs }

func (p *PayloadData) AllChanges() map[string]string { return p.allChanges }

func (p *PayloadData) IterCount() int { return p.iterCount }

func (p *PayloadData) WorkstreamElem() *dt.InvestigatorPlanJsonworkstreamsElem {
	return &p.workstreamElem
}

func (p *PayloadData) TlIter() int { return p.tlIter }

func (p *PayloadData) HasCoderOutputs() bool { return len(p.coderOutputs) > 0 }

type PayloadDataBuilder struct {
	revisionPrompt                 string
	docSuffix                      string
	revisionInvPrefix              string
	initialPromptContext           string
	codeReview                     dt.CodeReviewJson
	decompositionReview            dt.SystemDecompositionReviewJson
	investigationPlanQualityReview dt.InvestigationPlanQualityReviewJson
	gapAnalysisReview              dt.GapAnalysisReviewJson
	archReview                     dt.ArchReviewJson
	planReview                     dt.PlanReviewJson
	archFinal                      dt.ArchFinalJson
	techLeadFinalReview            *dt.TechLeadFinalJson
	hasTechLeadFinalReview         bool
	factReview                     dt.FactCheckingReviewJson
	consistencyReview              dt.SynthesisConsistencyReviewJson
	investigationReport            *dt.InvestigationReportJson
	investigatorFindings           *dt.InvestigatorFindingsJson
	findingsList                   []dt.InvestigatorFindingsJson
	coderOutputs                   []dt.CoderJson
	allChanges                     map[string]string
	iterCount                      int
	workstreamElem                 dt.InvestigatorPlanJsonworkstreamsElem
	tlIter                         int
}

func NewPayloadDataBuilder() *PayloadDataBuilder {
	return &PayloadDataBuilder{}
}

func (b *PayloadDataBuilder) Build() *PayloadData {
	return &PayloadData{
		revisionPrompt:                 b.revisionPrompt,
		docSuffix:                      b.docSuffix,
		revisionInvPrefix:              b.revisionInvPrefix,
		initialPromptContext:           b.initialPromptContext,
		codeReview:                     b.codeReview,
		decompositionReview:            b.decompositionReview,
		investigationPlanQualityReview: b.investigationPlanQualityReview,
		gapAnalysisReview:              b.gapAnalysisReview,
		archReview:                     b.archReview,
		planReview:                     b.planReview,
		archFinal:                      b.archFinal,
		techLeadFinalReview:            b.techLeadFinalReview,
		hasTechLeadFinalReview:         b.hasTechLeadFinalReview,
		factReview:                     b.factReview,
		consistencyReview:              b.consistencyReview,
		investigationReport:            b.investigationReport,
		investigatorFindings:           b.investigatorFindings,
		findingsList:                   b.findingsList,
		coderOutputs:                   b.coderOutputs,
		allChanges:                     b.allChanges,
		iterCount:                      b.iterCount,
		workstreamElem:                 b.workstreamElem,
		tlIter:                         b.tlIter,
	}
}

func (b *PayloadDataBuilder) WithRevisionPrompt(v string) *PayloadDataBuilder {
	b.revisionPrompt = v
	return b
}

func (b *PayloadDataBuilder) WithDocSuffix(v string) *PayloadDataBuilder {
	b.docSuffix = v
	return b
}

func (b *PayloadDataBuilder) WithRevisionInvPrefix(v string) *PayloadDataBuilder {
	b.revisionInvPrefix = v
	return b
}

func (b *PayloadDataBuilder) WithInitialPromptContext(v string) *PayloadDataBuilder {
	b.initialPromptContext = v
	return b
}

func (b *PayloadDataBuilder) WithCodeReview(v dt.CodeReviewJson) *PayloadDataBuilder {
	b.codeReview = v
	return b
}

func (b *PayloadDataBuilder) WithDecompositionReview(v dt.SystemDecompositionReviewJson) *PayloadDataBuilder {
	b.decompositionReview = v
	return b
}

func (b *PayloadDataBuilder) WithInvestigationPlanQualityReview(v dt.InvestigationPlanQualityReviewJson) *PayloadDataBuilder {
	b.investigationPlanQualityReview = v
	return b
}

func (b *PayloadDataBuilder) WithGapAnalysisReview(v dt.GapAnalysisReviewJson) *PayloadDataBuilder {
	b.gapAnalysisReview = v
	return b
}

func (b *PayloadDataBuilder) WithArchReview(v dt.ArchReviewJson) *PayloadDataBuilder {
	b.archReview = v
	return b
}

func (b *PayloadDataBuilder) WithPlanReview(v dt.PlanReviewJson) *PayloadDataBuilder {
	b.planReview = v
	return b
}

func (b *PayloadDataBuilder) WithArchFinal(v dt.ArchFinalJson) *PayloadDataBuilder {
	b.archFinal = v
	return b
}

func (b *PayloadDataBuilder) WithTechLeadFinalReview(v *dt.TechLeadFinalJson) *PayloadDataBuilder {
	b.techLeadFinalReview = v
	b.hasTechLeadFinalReview = v != nil
	return b
}

func (b *PayloadDataBuilder) WithFactReview(v dt.FactCheckingReviewJson) *PayloadDataBuilder {
	b.factReview = v
	return b
}

func (b *PayloadDataBuilder) WithConsistencyReview(v dt.SynthesisConsistencyReviewJson) *PayloadDataBuilder {
	b.consistencyReview = v
	return b
}

func (b *PayloadDataBuilder) WithInvestigationReport(v *dt.InvestigationReportJson) *PayloadDataBuilder {
	b.investigationReport = v
	return b
}

func (b *PayloadDataBuilder) WithInvestigatorFindings(v *dt.InvestigatorFindingsJson) *PayloadDataBuilder {
	b.investigatorFindings = v
	return b
}

func (b *PayloadDataBuilder) WithFindingsList(v []dt.InvestigatorFindingsJson) *PayloadDataBuilder {
	b.findingsList = v
	return b
}

func (b *PayloadDataBuilder) WithCoderOutputs(v []dt.CoderJson) *PayloadDataBuilder {
	b.coderOutputs = v
	return b
}

func (b *PayloadDataBuilder) WithAllChanges(v map[string]string) *PayloadDataBuilder {
	b.allChanges = v
	return b
}

func (b *PayloadDataBuilder) WithIterCount(v int) *PayloadDataBuilder {
	b.iterCount = v
	return b
}

func (b *PayloadDataBuilder) WithWorkstreamElem(v dt.InvestigatorPlanJsonworkstreamsElem) *PayloadDataBuilder {
	b.workstreamElem = v
	return b
}

func (b *PayloadDataBuilder) WithTlIter(v int) *PayloadDataBuilder {
	b.tlIter = v
	return b
}

func DecompositionReviewPayload(review *dt.SystemDecompositionReviewJson) *PayloadData {
	return NewPayloadDataBuilder().WithDecompositionReview(*review).Build()
}

func InvestigationPlanQualityReviewPayload(review *dt.InvestigationPlanQualityReviewJson) *PayloadData {
	return NewPayloadDataBuilder().WithInvestigationPlanQualityReview(*review).Build()
}

func GapAnalysisReviewPayload(review *dt.GapAnalysisReviewJson) *PayloadData {
	return NewPayloadDataBuilder().WithGapAnalysisReview(*review).Build()
}

func ArchReviewPayload(review *dt.ArchReviewJson) *PayloadData {
	return NewPayloadDataBuilder().WithArchReview(*review).Build()
}

func PlanReviewPayload(review *dt.PlanReviewJson) *PayloadData {
	return NewPayloadDataBuilder().WithPlanReview(*review).Build()
}

func ArchFinalPayload(review *dt.ArchFinalJson) *PayloadData {
	return NewPayloadDataBuilder().WithArchFinal(*review).Build()
}

func CodeReviewWithPlanPayload(review *dt.CodeReviewJson, tlIter int) *PayloadData {
	return NewPayloadDataBuilder().
		WithCodeReview(*review).
		WithTlIter(tlIter).
		Build()
}

func CodeReviewOrchestratorPayload(
	coderOutputs []dt.CoderJson,
	allChanges map[string]string,
	revisionInvPrefix string,
	initialPromptContext string,
	iterCount int,
	techLeadFinalReview *dt.TechLeadFinalJson,
	docSuffix string,
) *PayloadData {
	return NewPayloadDataBuilder().
		WithCoderOutputs(coderOutputs).
		WithAllChanges(allChanges).
		WithRevisionInvPrefix(revisionInvPrefix).
		WithInitialPromptContext(initialPromptContext).
		WithIterCount(iterCount).
		WithTechLeadFinalReview(techLeadFinalReview).
		WithDocSuffix(docSuffix).
		Build()
}

func CodeImplementationPayload(
	techLeadFinalReview *dt.TechLeadFinalJson,
	docSuffix string,
	tlIter int,
) *PayloadData {
	return NewPayloadDataBuilder().
		WithTechLeadFinalReview(techLeadFinalReview).
		WithDocSuffix(docSuffix).
		WithTlIter(tlIter).
		Build()
}

func InvestigationSynthesisPayload(
	report *dt.InvestigationReportJson,
	findingsList []dt.InvestigatorFindingsJson,
	consistencyReview *dt.SynthesisConsistencyReviewJson,
) *PayloadData {
	return NewPayloadDataBuilder().
		WithInvestigationReport(report).
		WithFindingsList(findingsList).
		WithConsistencyReview(*consistencyReview).
		Build()
}

func InvestigationConsistencyReviewOrchestratorPayload(
	report *dt.InvestigationReportJson,
	findingsList []dt.InvestigatorFindingsJson,
	consistencyReview *dt.SynthesisConsistencyReviewJson,
) *PayloadData {
	return NewPayloadDataBuilder().
		WithInvestigationReport(report).
		WithFindingsList(findingsList).
		WithConsistencyReview(*consistencyReview).
		Build()
}

func InvestigationFactReviewOrchestratorPayload(
	gapReview *dt.GapAnalysisReviewJson,
	factReview *dt.FactCheckingReviewJson,
	findings *dt.InvestigatorFindingsJson,
	workstreamElem *dt.InvestigatorPlanJsonworkstreamsElem,
) *PayloadData {
	return NewPayloadDataBuilder().
		WithGapAnalysisReview(*gapReview).
		WithFactReview(*factReview).
		WithInvestigatorFindings(findings).
		WithWorkstreamElem(*workstreamElem).
		Build()
}

func TechLeadReviewPayload(tlIter int, techLeadFinalReview *dt.TechLeadFinalJson) *PayloadData {
	return NewPayloadDataBuilder().
		WithTlIter(tlIter).
		WithTechLeadFinalReview(techLeadFinalReview).
		Build()
}

func TechLeadReviewOrchestratorPayload(
	techLeadFinalReview *dt.TechLeadFinalJson,
	docSuffix string,
	tlIter int,
) *PayloadData {
	return NewPayloadDataBuilder().
		WithTechLeadFinalReview(techLeadFinalReview).
		WithDocSuffix(docSuffix).
		WithTlIter(tlIter).
		Build()
}

func PMExpansionCleanupPayload(docSuffix string) *PayloadData {
	return NewPayloadDataBuilder().WithDocSuffix(docSuffix).Build()
}

func SimpleStringPayload(prompt string) *PayloadData {
	return NewPayloadDataBuilder().WithRevisionPrompt(prompt).Build()
}

func ConvertFindingsList(raw []interface{}) []dt.InvestigatorFindingsJson {
	result := make([]dt.InvestigatorFindingsJson, 0, len(raw))
	for _, item := range raw {
		if item == nil {
			continue
		}
		result = append(result, item.(dt.InvestigatorFindingsJson))
	}
	return result
}
