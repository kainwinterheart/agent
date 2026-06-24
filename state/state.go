package state

import (
	dt "agent-go/gen"
	"github.com/benbjohnson/immutable"
)

type DomainState struct {
	architecture          *dt.ArchJson
	plan                  *dt.PlanJson
	codeOutputs           *immutable.List[dt.CoderJson]
	codeSummary           string
	finalFeedback         *dt.ArchFinalJson
	finalReviewPassed     bool
	wrappedTask           string
	wrappedCoderTask      string
	pmFilepath            string
	allChanges            *immutable.Map[string, string]
	codeSummaries         *immutable.List[string]
	mergedSummaries       string
	techLeadRevisionCycle bool
	techLeadReviewIter    int
	techLeadReviewResult  *dt.TechLeadFinalJson
}

func NewDomainStateBuilder(ds *DomainState) *DomainStateBuilder {
	if ds == nil {
		return &DomainStateBuilder{}
	}
	return &DomainStateBuilder{
		architecture:          ds.architecture,
		plan:                  ds.plan,
		codeOutputs:           ds.codeOutputs,
		codeSummary:           ds.codeSummary,
		finalFeedback:         ds.finalFeedback,
		finalReviewPassed:     ds.finalReviewPassed,
		wrappedTask:           ds.wrappedTask,
		wrappedCoderTask:      ds.wrappedCoderTask,
		pmFilepath:            ds.pmFilepath,
		allChanges:            ds.allChanges,
		codeSummaries:         ds.codeSummaries,
		mergedSummaries:       ds.mergedSummaries,
		techLeadRevisionCycle: ds.techLeadRevisionCycle,
		techLeadReviewIter:    ds.techLeadReviewIter,
		techLeadReviewResult:  ds.techLeadReviewResult,
	}
}

type DomainStateBuilder struct {
	architecture          *dt.ArchJson
	plan                  *dt.PlanJson
	codeOutputs           *immutable.List[dt.CoderJson]
	codeSummary           string
	finalFeedback         *dt.ArchFinalJson
	finalReviewPassed     bool
	wrappedTask           string
	wrappedCoderTask      string
	pmFilepath            string
	allChanges            *immutable.Map[string, string]
	codeSummaries         *immutable.List[string]
	mergedSummaries       string
	techLeadRevisionCycle bool
	techLeadReviewIter    int
	techLeadReviewResult  *dt.TechLeadFinalJson
}

func (b *DomainStateBuilder) Build() *DomainState {
	return &DomainState{
		architecture:          b.architecture,
		plan:                  b.plan,
		codeOutputs:           b.codeOutputs,
		codeSummary:           b.codeSummary,
		finalFeedback:         b.finalFeedback,
		finalReviewPassed:     b.finalReviewPassed,
		wrappedTask:           b.wrappedTask,
		wrappedCoderTask:      b.wrappedCoderTask,
		pmFilepath:            b.pmFilepath,
		allChanges:            b.allChanges,
		codeSummaries:         b.codeSummaries,
		mergedSummaries:       b.mergedSummaries,
		techLeadRevisionCycle: b.techLeadRevisionCycle,
		techLeadReviewIter:    b.techLeadReviewIter,
		techLeadReviewResult:  b.techLeadReviewResult,
	}
}

func (ds *DomainState) Architecture() *dt.ArchJson                  { return ds.architecture }
func (ds *DomainState) Plan() *dt.PlanJson                          { return ds.plan }
func (ds *DomainState) CodeOutputs() *immutable.List[dt.CoderJson]  { return ds.codeOutputs }
func (ds *DomainState) CodeSummary() string                         { return ds.codeSummary }
func (ds *DomainState) FinalFeedback() *dt.ArchFinalJson            { return ds.finalFeedback }
func (ds *DomainState) FinalReviewPassed() bool                     { return ds.finalReviewPassed }
func (ds *DomainState) WrappedTask() string                         { return ds.wrappedTask }
func (ds *DomainState) WrappedCoderTask() string                    { return ds.wrappedCoderTask }
func (ds *DomainState) PMFilepath() string                          { return ds.pmFilepath }
func (ds *DomainState) AllChanges() *immutable.Map[string, string]  { return ds.allChanges }
func (ds *DomainState) CodeSummaries() *immutable.List[string]      { return ds.codeSummaries }
func (ds *DomainState) MergedSummaries() string                     { return ds.mergedSummaries }
func (ds *DomainState) TechLeadRevisionCycle() bool                 { return ds.techLeadRevisionCycle }
func (ds *DomainState) TechLeadReviewIter() int                     { return ds.techLeadReviewIter }
func (ds *DomainState) TechLeadReviewResult() *dt.TechLeadFinalJson { return ds.techLeadReviewResult }

func (b *DomainStateBuilder) WithArchitecture(v *dt.ArchJson) *DomainStateBuilder {
	b.architecture = v
	return b
}
func (b *DomainStateBuilder) WithPlan(v *dt.PlanJson) *DomainStateBuilder { b.plan = v; return b }
func (b *DomainStateBuilder) WithCodeOutputs(v *immutable.List[dt.CoderJson]) *DomainStateBuilder {
	b.codeOutputs = v
	return b
}
func (b *DomainStateBuilder) WithCodeSummary(v string) *DomainStateBuilder {
	b.codeSummary = v
	return b
}
func (b *DomainStateBuilder) WithFinalFeedback(v *dt.ArchFinalJson) *DomainStateBuilder {
	b.finalFeedback = v
	return b
}
func (b *DomainStateBuilder) WithFinalReviewPassed(v bool) *DomainStateBuilder {
	b.finalReviewPassed = v
	return b
}
func (b *DomainStateBuilder) WithWrappedTask(v string) *DomainStateBuilder {
	b.wrappedTask = v
	return b
}
func (b *DomainStateBuilder) WithWrappedCoderTask(v string) *DomainStateBuilder {
	b.wrappedCoderTask = v
	return b
}
func (b *DomainStateBuilder) WithPMFilepath(v string) *DomainStateBuilder { b.pmFilepath = v; return b }
func (b *DomainStateBuilder) WithAllChanges(v *immutable.Map[string, string]) *DomainStateBuilder {
	b.allChanges = v
	return b
}
func (b *DomainStateBuilder) WithCodeSummaries(v *immutable.List[string]) *DomainStateBuilder {
	b.codeSummaries = v
	return b
}
func (b *DomainStateBuilder) WithMergedSummaries(v string) *DomainStateBuilder {
	b.mergedSummaries = v
	return b
}
func (b *DomainStateBuilder) WithTechLeadRevisionCycle(v bool) *DomainStateBuilder {
	b.techLeadRevisionCycle = v
	return b
}
func (b *DomainStateBuilder) WithTechLeadReviewIter(v int) *DomainStateBuilder {
	b.techLeadReviewIter = v
	return b
}
func (b *DomainStateBuilder) WithTechLeadReviewResult(v *dt.TechLeadFinalJson) *DomainStateBuilder {
	b.techLeadReviewResult = v
	return b
}

type WorkflowState struct {
	task                   string
	subdir                 string
	rephrasedTask          *dt.PmSynthesizerJson
	choices                string
	decompositionResult    *dt.SystemDecompositionJson
	domains                *immutable.Map[string, *DomainState]
	investigationPlan      *dt.InvestigatorPlanJson
	investigationResults   *dt.InvestigationReportJson
	completedWorkstreams   *immutable.Map[string, dt.InvestigatorFindingsJson]
	workstreams            *immutable.List[dt.InvestigatorPlanJsonworkstreamsElem]
	speculativeExpansions  *immutable.List[string]
	out                    string
	pmFilepath             string
	finalInvestigationTask string
	domainIterationIndex   int
	domainIterationTotal   int
	domainCurrentStage     string
	techLeadDocSuffix      string
}

func NewWorkflowStateBuilder(ws *WorkflowState) *WorkflowStateBuilder {
	if ws == nil {
		return &WorkflowStateBuilder{}
	}
	return &WorkflowStateBuilder{
		task:                   ws.task,
		subdir:                 ws.subdir,
		rephrasedTask:          ws.rephrasedTask,
		choices:                ws.choices,
		decompositionResult:    ws.decompositionResult,
		domains:                ws.domains,
		investigationPlan:      ws.investigationPlan,
		investigationResults:   ws.investigationResults,
		completedWorkstreams:   ws.completedWorkstreams,
		workstreams:            ws.workstreams,
		speculativeExpansions:  ws.speculativeExpansions,
		out:                    ws.out,
		pmFilepath:             ws.pmFilepath,
		finalInvestigationTask: ws.finalInvestigationTask,
		domainIterationIndex:   ws.domainIterationIndex,
		domainIterationTotal:   ws.domainIterationTotal,
		domainCurrentStage:     ws.domainCurrentStage,
		techLeadDocSuffix:      ws.techLeadDocSuffix,
	}
}

type WorkflowStateBuilder struct {
	task                   string
	subdir                 string
	rephrasedTask          *dt.PmSynthesizerJson
	choices                string
	decompositionResult    *dt.SystemDecompositionJson
	domains                *immutable.Map[string, *DomainState]
	investigationPlan      *dt.InvestigatorPlanJson
	investigationResults   *dt.InvestigationReportJson
	completedWorkstreams   *immutable.Map[string, dt.InvestigatorFindingsJson]
	workstreams            *immutable.List[dt.InvestigatorPlanJsonworkstreamsElem]
	speculativeExpansions  *immutable.List[string]
	out                    string
	pmFilepath             string
	finalInvestigationTask string
	domainIterationIndex   int
	domainIterationTotal   int
	domainCurrentStage     string
	techLeadDocSuffix      string
}

func (b *WorkflowStateBuilder) Build() *WorkflowState {
	return &WorkflowState{
		task:                   b.task,
		subdir:                 b.subdir,
		rephrasedTask:          b.rephrasedTask,
		choices:                b.choices,
		decompositionResult:    b.decompositionResult,
		domains:                b.domains,
		investigationPlan:      b.investigationPlan,
		investigationResults:   b.investigationResults,
		completedWorkstreams:   b.completedWorkstreams,
		workstreams:            b.workstreams,
		speculativeExpansions:  b.speculativeExpansions,
		out:                    b.out,
		pmFilepath:             b.pmFilepath,
		finalInvestigationTask: b.finalInvestigationTask,
		domainIterationIndex:   b.domainIterationIndex,
		domainIterationTotal:   b.domainIterationTotal,
		domainCurrentStage:     b.domainCurrentStage,
		techLeadDocSuffix:      b.techLeadDocSuffix,
	}
}

func (ws *WorkflowState) Task() string                         { return ws.task }
func (ws *WorkflowState) Subdir() string                       { return ws.subdir }
func (ws *WorkflowState) RephrasedTask() *dt.PmSynthesizerJson { return ws.rephrasedTask }
func (ws *WorkflowState) Choices() string                      { return ws.choices }
func (ws *WorkflowState) DecompositionResult() *dt.SystemDecompositionJson {
	return ws.decompositionResult
}
func (ws *WorkflowState) Domains() *immutable.Map[string, *DomainState] { return ws.domains }
func (ws *WorkflowState) InvestigationPlan() *dt.InvestigatorPlanJson   { return ws.investigationPlan }
func (ws *WorkflowState) InvestigationResults() *dt.InvestigationReportJson {
	return ws.investigationResults
}
func (ws *WorkflowState) CompletedWorkstreams() *immutable.Map[string, dt.InvestigatorFindingsJson] {
	return ws.completedWorkstreams
}
func (ws *WorkflowState) Workstreams() *immutable.List[dt.InvestigatorPlanJsonworkstreamsElem] {
	return ws.workstreams
}
func (ws *WorkflowState) SpeculativeExpansions() *immutable.List[string] {
	return ws.speculativeExpansions
}
func (ws *WorkflowState) Out() string                    { return ws.out }
func (ws *WorkflowState) PMFilepath() string             { return ws.pmFilepath }
func (ws *WorkflowState) FinalInvestigationTask() string { return ws.finalInvestigationTask }
func (ws *WorkflowState) DomainIterationIndex() int      { return ws.domainIterationIndex }
func (ws *WorkflowState) DomainIterationTotal() int      { return ws.domainIterationTotal }
func (ws *WorkflowState) DomainCurrentStage() string     { return ws.domainCurrentStage }
func (ws *WorkflowState) TechLeadDocSuffix() string      { return ws.techLeadDocSuffix }

func (b *WorkflowStateBuilder) WithTask(v string) *WorkflowStateBuilder   { b.task = v; return b }
func (b *WorkflowStateBuilder) WithSubdir(v string) *WorkflowStateBuilder { b.subdir = v; return b }
func (b *WorkflowStateBuilder) WithRephrasedTask(v *dt.PmSynthesizerJson) *WorkflowStateBuilder {
	b.rephrasedTask = v
	return b
}
func (b *WorkflowStateBuilder) WithChoices(v string) *WorkflowStateBuilder { b.choices = v; return b }
func (b *WorkflowStateBuilder) WithDecompositionResult(v *dt.SystemDecompositionJson) *WorkflowStateBuilder {
	b.decompositionResult = v
	return b
}
func (b *WorkflowStateBuilder) WithDomains(v *immutable.Map[string, *DomainState]) *WorkflowStateBuilder {
	b.domains = v
	return b
}
func (b *WorkflowStateBuilder) WithInvestigationPlan(v *dt.InvestigatorPlanJson) *WorkflowStateBuilder {
	b.investigationPlan = v
	return b
}
func (b *WorkflowStateBuilder) WithInvestigationResults(v *dt.InvestigationReportJson) *WorkflowStateBuilder {
	b.investigationResults = v
	return b
}
func (b *WorkflowStateBuilder) WithCompletedWorkstreams(v *immutable.Map[string, dt.InvestigatorFindingsJson]) *WorkflowStateBuilder {
	b.completedWorkstreams = v
	return b
}
func (b *WorkflowStateBuilder) WithWorkstreams(v *immutable.List[dt.InvestigatorPlanJsonworkstreamsElem]) *WorkflowStateBuilder {
	b.workstreams = v
	return b
}
func (b *WorkflowStateBuilder) WithSpeculativeExpansions(v *immutable.List[string]) *WorkflowStateBuilder {
	b.speculativeExpansions = v
	return b
}
func (b *WorkflowStateBuilder) WithOut(v string) *WorkflowStateBuilder { b.out = v; return b }
func (b *WorkflowStateBuilder) WithPMFilepath(v string) *WorkflowStateBuilder {
	b.pmFilepath = v
	return b
}
func (b *WorkflowStateBuilder) WithFinalInvestigationTask(v string) *WorkflowStateBuilder {
	b.finalInvestigationTask = v
	return b
}
func (b *WorkflowStateBuilder) WithDomainIterationIndex(v int) *WorkflowStateBuilder {
	b.domainIterationIndex = v
	return b
}
func (b *WorkflowStateBuilder) WithDomainIterationTotal(v int) *WorkflowStateBuilder {
	b.domainIterationTotal = v
	return b
}
func (b *WorkflowStateBuilder) WithDomainCurrentStage(v string) *WorkflowStateBuilder {
	b.domainCurrentStage = v
	return b
}
func (b *WorkflowStateBuilder) WithTechLeadDocSuffix(v string) *WorkflowStateBuilder {
	b.techLeadDocSuffix = v
	return b
}

func ListToImmutableList[T any](s []T) *immutable.List[T] {
	if s == nil {
		return nil
	}
	result := immutable.NewList[T]()
	for _, v := range s {
		result = result.Append(v)
	}
	return result
}

func ImmutableStringMapToRegular(m *immutable.Map[string, string]) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string)
	itr := m.Iterator()
	itr.First()
	for !itr.Done() {
		k, v, _ := itr.Next()
		result[k] = v
	}
	return result
}

type StepType int

const (
	StepPMCandidateGenerate StepType = iota
	StepPMSynthesize
	StepPMReview
	StepClassification
	StepDecomposition
	StepDecompositionReview
	StepDomainStart
	StepInvestigationPlanGenerate
	StepInvestigationPlanReview
	StepInvestigationWorkstream
	StepInvestigationWorkstreamReview
	StepInvestigationSynthesis
	StepInvestigationConsistencyReview
	StepArchitecture
	StepArchitectureReview
	StepPlan
	StepPlanReview
	StepCodeImplementation
	StepCodeReview
	StepTechLeadReview
	StepArchitectureFinalReview
	StepFinalInvestigation
	StepPMExpansionCleanup
	StepInvestigationPlanStructuralReview
	StepInvestigationFactReview
	StepDecompositionOrchestrator
	StepArchitectureOrchestrator
	StepPlanOrchestrator
	StepCodeReviewOrchestrator
	StepTechLeadReviewOrchestrator
	StepArchitectureFinalReviewOrchestrator
	StepInvestigationFactReviewOrchestrator
	StepInvestigationConsistencyReviewOrchestrator
	StepTechLeadEnd
	StepBoundary
)

type WorkflowStep struct {
	stepType            StepType
	state               *WorkflowState
	domainID            string
	workstreamID        string
	iteration           int
	reviewIteration     int
	codeReviewIteration int
	payload             *PayloadData
	domainIndex         int
	workstreamIndex     int
}

type WorkflowStepBuilder struct {
	stack               *WorkflowStack
	stepType            StepType
	state               *WorkflowState
	domainID            string
	workstreamID        string
	iteration           int
	reviewIteration     int
	codeReviewIteration int
	payload             *PayloadData
	domainIndex         int
	workstreamIndex     int
}

func (b *WorkflowStepBuilder) Build() *WorkflowStep {
	return &WorkflowStep{
		stepType:            b.stepType,
		state:               b.state,
		domainID:            b.domainID,
		workstreamID:        b.workstreamID,
		iteration:           b.iteration,
		reviewIteration:     b.reviewIteration,
		codeReviewIteration: b.codeReviewIteration,
		payload:             b.payload,
		domainIndex:         b.domainIndex,
		workstreamIndex:     b.workstreamIndex,
	}
}

func (ws *WorkflowStep) Type() StepType           { return ws.stepType }
func (ws *WorkflowStep) State() *WorkflowState    { return ws.state }
func (ws *WorkflowStep) DomainID() string         { return ws.domainID }
func (ws *WorkflowStep) WorkstreamID() string     { return ws.workstreamID }
func (ws *WorkflowStep) Iteration() int           { return ws.iteration }
func (ws *WorkflowStep) ReviewIteration() int     { return ws.reviewIteration }
func (ws *WorkflowStep) CodeReviewIteration() int { return ws.codeReviewIteration }
func (ws *WorkflowStep) Payload() *PayloadData    { return ws.payload }
func (ws *WorkflowStep) DomainIndex() int         { return ws.domainIndex }
func (ws *WorkflowStep) WorkstreamIndex() int     { return ws.workstreamIndex }

func (b *WorkflowStepBuilder) WithDomainID(v string) *WorkflowStepBuilder { b.domainID = v; return b }
func (b *WorkflowStepBuilder) WithWorkstreamID(v string) *WorkflowStepBuilder {
	b.workstreamID = v
	return b
}
func (b *WorkflowStepBuilder) WithIteration(v int) *WorkflowStepBuilder { b.iteration = v; return b }
func (b *WorkflowStepBuilder) WithReviewIteration(v int) *WorkflowStepBuilder {
	b.reviewIteration = v
	return b
}
func (b *WorkflowStepBuilder) WithCodeReviewIteration(v int) *WorkflowStepBuilder {
	b.codeReviewIteration = v
	return b
}
func (b *WorkflowStepBuilder) WithPayload(v *PayloadData) *WorkflowStepBuilder {
	b.payload = v
	return b
}
func (b *WorkflowStepBuilder) WithDomainIndex(v int) *WorkflowStepBuilder {
	b.domainIndex = v
	return b
}
func (b *WorkflowStepBuilder) WithWorkstreamIndex(v int) *WorkflowStepBuilder {
	b.workstreamIndex = v
	return b
}

func (b *WorkflowStepBuilder) Push() {
	b.stack.Push(b.Build())
}

type WorkflowStack struct {
	stack []*WorkflowStep
}

func (ws *WorkflowStack) Push(s *WorkflowStep) {
	ws.stack = append(ws.stack, s)
}

func (ws *WorkflowStack) NewStep(stepType StepType, state *WorkflowState) *WorkflowStepBuilder {
	return &WorkflowStepBuilder{
		stack:    ws,
		stepType: stepType,
		state:    state,
	}
}

func (ws *WorkflowStack) Pop() (*WorkflowStep, bool) {
	if len(ws.stack) == 0 {
		return nil, false
	}
	idx := len(ws.stack) - 1
	v := ws.stack[idx]
	ws.stack = ws.stack[:idx]
	return v, true
}
