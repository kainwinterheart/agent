package state

import "github.com/benbjohnson/immutable"

// DomainState holds per-domain execution state.
type DomainState struct {
	architecture          *immutable.Map[string, interface{}]
	plan                  *immutable.Map[string, interface{}]
	codeOutputs           *immutable.List[interface{}]
	codeSummary           string
	finalFeedback         *immutable.Map[string, interface{}]
	finalReviewPassed     bool
	coderTask             string
	spec                  string
	archExtra             string
	wrappedTask           string
	wrappedCoderTask      string
	pmFilepath            string
	allChanges            *immutable.Map[string, string]
	codeSummaries         *immutable.List[string]
	mergedSummaries       string
	codeReviewCounter     int
	techLeadRevisionCycle bool
	techLeadReview        *immutable.Map[string, interface{}]
	techLeadReviewCycle   int
	techLeadReviewIter    int
	techLeadReviewResult  *immutable.Map[string, interface{}]
}

// NewDomainStateBuilder creates a builder initialized from an existing DomainState.
// All fields default to the current values; set non-nil/non-empty overrides to change them.
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
		coderTask:             ds.coderTask,
		spec:                  ds.spec,
		archExtra:             ds.archExtra,
		wrappedTask:           ds.wrappedTask,
		wrappedCoderTask:      ds.wrappedCoderTask,
		pmFilepath:            ds.pmFilepath,
		allChanges:            ds.allChanges,
		codeSummaries:         ds.codeSummaries,
		mergedSummaries:       ds.mergedSummaries,
		codeReviewCounter:     ds.codeReviewCounter,
		techLeadRevisionCycle: ds.techLeadRevisionCycle,
		techLeadReview:        ds.techLeadReview,
		techLeadReviewCycle:   ds.techLeadReviewCycle,
		techLeadReviewIter:    ds.techLeadReviewIter,
		techLeadReviewResult:  ds.techLeadReviewResult,
	}
}

type DomainStateBuilder struct {
	architecture          *immutable.Map[string, interface{}]
	plan                  *immutable.Map[string, interface{}]
	codeOutputs           *immutable.List[interface{}]
	codeSummary           string
	finalFeedback         *immutable.Map[string, interface{}]
	finalReviewPassed     bool
	coderTask             string
	spec                  string
	archExtra             string
	wrappedTask           string
	wrappedCoderTask      string
	pmFilepath            string
	allChanges            *immutable.Map[string, string]
	codeSummaries         *immutable.List[string]
	mergedSummaries       string
	codeReviewCounter     int
	techLeadRevisionCycle bool
	techLeadReview        *immutable.Map[string, interface{}]
	techLeadReviewCycle   int
	techLeadReviewIter    int
	techLeadReviewResult  *immutable.Map[string, interface{}]
}

// Build returns a new DomainState with the builder's field values.
func (b *DomainStateBuilder) Build() *DomainState {
	return &DomainState{
		architecture:          b.architecture,
		plan:                  b.plan,
		codeOutputs:           b.codeOutputs,
		codeSummary:           b.codeSummary,
		finalFeedback:         b.finalFeedback,
		finalReviewPassed:     b.finalReviewPassed,
		coderTask:             b.coderTask,
		spec:                  b.spec,
		archExtra:             b.archExtra,
		wrappedTask:           b.wrappedTask,
		wrappedCoderTask:      b.wrappedCoderTask,
		pmFilepath:            b.pmFilepath,
		allChanges:            b.allChanges,
		codeSummaries:         b.codeSummaries,
		mergedSummaries:       b.mergedSummaries,
		codeReviewCounter:     b.codeReviewCounter,
		techLeadRevisionCycle: b.techLeadRevisionCycle,
		techLeadReview:        b.techLeadReview,
		techLeadReviewCycle:   b.techLeadReviewCycle,
		techLeadReviewIter:    b.techLeadReviewIter,
		techLeadReviewResult:  b.techLeadReviewResult,
	}
}

// --- Reader methods ---

func (ds *DomainState) Architecture() *immutable.Map[string, interface{}]   { return ds.architecture }
func (ds *DomainState) Plan() *immutable.Map[string, interface{}]           { return ds.plan }
func (ds *DomainState) CodeOutputs() *immutable.List[interface{}]           { return ds.codeOutputs }
func (ds *DomainState) CodeSummary() string                                 { return ds.codeSummary }
func (ds *DomainState) FinalFeedback() *immutable.Map[string, interface{}]  { return ds.finalFeedback }
func (ds *DomainState) FinalReviewPassed() bool                             { return ds.finalReviewPassed }
func (ds *DomainState) CoderTask() string                                   { return ds.coderTask }
func (ds *DomainState) Spec() string                                        { return ds.spec }
func (ds *DomainState) ArchExtra() string                                   { return ds.archExtra }
func (ds *DomainState) WrappedTask() string                                 { return ds.wrappedTask }
func (ds *DomainState) WrappedCoderTask() string                            { return ds.wrappedCoderTask }
func (ds *DomainState) PMFilepath() string                                  { return ds.pmFilepath }
func (ds *DomainState) AllChanges() *immutable.Map[string, string]          { return ds.allChanges }
func (ds *DomainState) CodeSummaries() *immutable.List[string]              { return ds.codeSummaries }
func (ds *DomainState) MergedSummaries() string                             { return ds.mergedSummaries }
func (ds *DomainState) CodeReviewCounter() int                              { return ds.codeReviewCounter }
func (ds *DomainState) TechLeadRevisionCycle() bool                         { return ds.techLeadRevisionCycle }
func (ds *DomainState) TechLeadReview() *immutable.Map[string, interface{}] { return ds.techLeadReview }
func (ds *DomainState) TechLeadReviewIter() int                             { return ds.techLeadReviewIter }
func (ds *DomainState) TechLeadReviewCycle() int                            { return ds.techLeadReviewCycle }
func (ds *DomainState) TechLeadReviewResult() *immutable.Map[string, interface{}] {
	return ds.techLeadReviewResult
}

// --- Builder field setters (return the builder for chaining) ---

func (b *DomainStateBuilder) WithArchitecture(v *immutable.Map[string, interface{}]) *DomainStateBuilder {
	b.architecture = v
	return b
}
func (b *DomainStateBuilder) WithPlan(v *immutable.Map[string, interface{}]) *DomainStateBuilder {
	b.plan = v
	return b
}
func (b *DomainStateBuilder) WithCodeOutputs(v *immutable.List[interface{}]) *DomainStateBuilder {
	b.codeOutputs = v
	return b
}
func (b *DomainStateBuilder) WithCodeSummary(v string) *DomainStateBuilder {
	b.codeSummary = v
	return b
}
func (b *DomainStateBuilder) WithFinalFeedback(v *immutable.Map[string, interface{}]) *DomainStateBuilder {
	b.finalFeedback = v
	return b
}
func (b *DomainStateBuilder) WithFinalReviewPassed(v bool) *DomainStateBuilder {
	b.finalReviewPassed = v
	return b
}
func (b *DomainStateBuilder) WithCoderTask(v string) *DomainStateBuilder {
	b.coderTask = v
	return b
}
func (b *DomainStateBuilder) WithSpec(v string) *DomainStateBuilder {
	b.spec = v
	return b
}
func (b *DomainStateBuilder) WithArchExtra(v string) *DomainStateBuilder {
	b.archExtra = v
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
func (b *DomainStateBuilder) WithPMFilepath(v string) *DomainStateBuilder {
	b.pmFilepath = v
	return b
}
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
func (b *DomainStateBuilder) WithCodeReviewCounter(v int) *DomainStateBuilder {
	b.codeReviewCounter = v
	return b
}
func (b *DomainStateBuilder) WithTechLeadRevisionCycle(v bool) *DomainStateBuilder {
	b.techLeadRevisionCycle = v
	return b
}
func (b *DomainStateBuilder) WithTechLeadReview(v *immutable.Map[string, interface{}]) *DomainStateBuilder {
	b.techLeadReview = v
	return b
}
func (b *DomainStateBuilder) WithTechLeadReviewIter(v int) *DomainStateBuilder {
	b.techLeadReviewIter = v
	return b
}
func (b *DomainStateBuilder) WithTechLeadReviewCycle(v int) *DomainStateBuilder {
	b.techLeadReviewCycle = v
	return b
}
func (b *DomainStateBuilder) WithTechLeadReviewResult(v *immutable.Map[string, interface{}]) *DomainStateBuilder {
	b.techLeadReviewResult = v
	return b
}

// --- Conversion helpers ---

func MapToImmutable(m map[string]interface{}) *immutable.Map[string, interface{}] {
	if m == nil {
		return nil
	}
	result := immutable.NewMap[string, interface{}](nil)
	for k, v := range m {
		result = result.Set(k, v)
	}
	return result
}

func SliceToImmutableList(s []interface{}) *immutable.List[interface{}] {
	if s == nil {
		return nil
	}
	result := immutable.NewList[interface{}]()
	for _, v := range s {
		result = result.Append(v)
	}
	return result
}

func stringMapToImmutable(m map[string]string) *immutable.Map[string, string] {
	if m == nil {
		return nil
	}
	result := immutable.NewMap[string, string](nil)
	for k, v := range m {
		result = result.Set(k, v)
	}
	return result
}

// ImmutableMapSliceToRegular converts *immutable.List[map[string]interface{}] to []map[string]interface{}.
func ImmutableMapSliceToRegular(l *immutable.List[map[string]interface{}]) []map[string]interface{} {
	if l == nil {
		return nil
	}
	result := make([]map[string]interface{}, l.Len())
	itr := l.Iterator()
	itr.First()
	for i := 0; i < l.Len(); i++ {
		_, v := itr.Next()
		result[i] = v
	}
	return result
}

// ImmutableMapToRegular converts *immutable.Map[string, interface{}] to map[string]interface{}.
func ImmutableMapToRegular(m *immutable.Map[string, interface{}]) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{})
	itr := m.Iterator()
	itr.First()
	for !itr.Done() {
		k, v, _ := itr.Next()
		result[k] = v
	}
	return result
}

func stringSliceToImmutable(s []string) *immutable.List[string] {
	if s == nil {
		return nil
	}
	result := immutable.NewList[string]()
	for _, v := range s {
		result = result.Append(v)
	}
	return result
}

// ImmutableListToRegularSlice converts *immutable.List[interface{}] to []interface{}.
func ImmutableListToRegularSlice(l *immutable.List[interface{}]) []interface{} {
	if l == nil {
		return nil
	}
	result := make([]interface{}, l.Len())
	for i := 0; i < l.Len(); i++ {
		result[i] = l.Get(i)
	}
	return result
}

// =========================
// WORKFLOW STATE
// =========================

// WorkflowState holds the state of a multi-domain workflow.
type WorkflowState struct {
	task                   string
	subdir                 string
	pmCandidates           *immutable.List[map[string]interface{}]
	rephrasedTask          *immutable.Map[string, interface{}]
	choices                string
	classification         *immutable.Map[string, interface{}]
	decompositionResult    *immutable.Map[string, interface{}]
	domains                *immutable.Map[string, *DomainState]
	investigationPlan      *immutable.Map[string, interface{}]
	investigationResults   *immutable.Map[string, interface{}]
	completedWorkstreams   *immutable.Map[string, interface{}]
	workstreams            *immutable.List[interface{}]
	investigationReport    string
	speculativeExpansions  *immutable.List[interface{}]
	out                    string
	pmFilepath             string
	finalInvestigationTask string
	domainIterationIndex   int
	domainIterationTotal   int
	domainCurrentStage     string
	techLeadDocSuffix      string
}

// NewWorkflowStateBuilder creates a builder initialized from an existing WorkflowState.
// All fields default to the current values; set non-nil/non-empty overrides to change them.
func NewWorkflowStateBuilder(ws *WorkflowState) *WorkflowStateBuilder {
	if ws == nil {
		return &WorkflowStateBuilder{}
	}
	return &WorkflowStateBuilder{
		task:                   ws.task,
		subdir:                 ws.subdir,
		pmCandidates:           ws.pmCandidates,
		rephrasedTask:          ws.rephrasedTask,
		choices:                ws.choices,
		classification:         ws.classification,
		decompositionResult:    ws.decompositionResult,
		domains:                ws.domains,
		investigationPlan:      ws.investigationPlan,
		investigationResults:   ws.investigationResults,
		completedWorkstreams:   ws.completedWorkstreams,
		workstreams:            ws.workstreams,
		investigationReport:    ws.investigationReport,
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
	pmCandidates           *immutable.List[map[string]interface{}]
	rephrasedTask          *immutable.Map[string, interface{}]
	choices                string
	classification         *immutable.Map[string, interface{}]
	decompositionResult    *immutable.Map[string, interface{}]
	domains                *immutable.Map[string, *DomainState]
	investigationPlan      *immutable.Map[string, interface{}]
	investigationResults   *immutable.Map[string, interface{}]
	completedWorkstreams   *immutable.Map[string, interface{}]
	workstreams            *immutable.List[interface{}]
	investigationReport    string
	speculativeExpansions  *immutable.List[interface{}]
	out                    string
	pmFilepath             string
	finalInvestigationTask string
	domainIterationIndex   int
	domainIterationTotal   int
	domainCurrentStage     string
	techLeadDocSuffix      string
}

// Build returns a new WorkflowState with the builder's field values.
func (b *WorkflowStateBuilder) Build() *WorkflowState {
	return &WorkflowState{
		task:                   b.task,
		subdir:                 b.subdir,
		pmCandidates:           b.pmCandidates,
		rephrasedTask:          b.rephrasedTask,
		choices:                b.choices,
		classification:         b.classification,
		decompositionResult:    b.decompositionResult,
		domains:                b.domains,
		investigationPlan:      b.investigationPlan,
		investigationResults:   b.investigationResults,
		completedWorkstreams:   b.completedWorkstreams,
		workstreams:            b.workstreams,
		investigationReport:    b.investigationReport,
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

// --- Reader methods ---

func (ws *WorkflowState) Task() string   { return ws.task }
func (ws *WorkflowState) Subdir() string { return ws.subdir }
func (ws *WorkflowState) PMCandidates() *immutable.List[map[string]interface{}] {
	return ws.pmCandidates
}
func (ws *WorkflowState) RephrasedTask() *immutable.Map[string, interface{}] { return ws.rephrasedTask }
func (ws *WorkflowState) Choices() string                                    { return ws.choices }
func (ws *WorkflowState) Classification() *immutable.Map[string, interface{}] {
	return ws.classification
}
func (ws *WorkflowState) DecompositionResult() *immutable.Map[string, interface{}] {
	return ws.decompositionResult
}
func (ws *WorkflowState) Domains() *immutable.Map[string, *DomainState] { return ws.domains }
func (ws *WorkflowState) InvestigationPlan() *immutable.Map[string, interface{}] {
	return ws.investigationPlan
}
func (ws *WorkflowState) InvestigationResults() *immutable.Map[string, interface{}] {
	return ws.investigationResults
}
func (ws *WorkflowState) CompletedWorkstreams() *immutable.Map[string, interface{}] {
	return ws.completedWorkstreams
}
func (ws *WorkflowState) Workstreams() *immutable.List[interface{}] { return ws.workstreams }
func (ws *WorkflowState) InvestigationReport() string               { return ws.investigationReport }
func (ws *WorkflowState) SpeculativeExpansions() *immutable.List[interface{}] {
	return ws.speculativeExpansions
}
func (ws *WorkflowState) Out() string                    { return ws.out }
func (ws *WorkflowState) PMFilepath() string             { return ws.pmFilepath }
func (ws *WorkflowState) FinalInvestigationTask() string { return ws.finalInvestigationTask }
func (ws *WorkflowState) DomainIterationIndex() int      { return ws.domainIterationIndex }
func (ws *WorkflowState) DomainIterationTotal() int      { return ws.domainIterationTotal }
func (ws *WorkflowState) DomainCurrentStage() string     { return ws.domainCurrentStage }
func (ws *WorkflowState) TechLeadDocSuffix() string      { return ws.techLeadDocSuffix }

// --- Builder field setters (return the builder for chaining) ---

func (b *WorkflowStateBuilder) WithTask(v string) *WorkflowStateBuilder {
	b.task = v
	return b
}
func (b *WorkflowStateBuilder) WithSubdir(v string) *WorkflowStateBuilder {
	b.subdir = v
	return b
}
func (b *WorkflowStateBuilder) WithPMCandidates(v *immutable.List[map[string]interface{}]) *WorkflowStateBuilder {
	b.pmCandidates = v
	return b
}
func (b *WorkflowStateBuilder) WithRephrasedTask(v *immutable.Map[string, interface{}]) *WorkflowStateBuilder {
	b.rephrasedTask = v
	return b
}
func (b *WorkflowStateBuilder) WithChoices(v string) *WorkflowStateBuilder {
	b.choices = v
	return b
}
func (b *WorkflowStateBuilder) WithClassification(v *immutable.Map[string, interface{}]) *WorkflowStateBuilder {
	b.classification = v
	return b
}
func (b *WorkflowStateBuilder) WithDecompositionResult(v *immutable.Map[string, interface{}]) *WorkflowStateBuilder {
	b.decompositionResult = v
	return b
}
func (b *WorkflowStateBuilder) WithDomains(v *immutable.Map[string, *DomainState]) *WorkflowStateBuilder {
	b.domains = v
	return b
}
func (b *WorkflowStateBuilder) WithInvestigationPlan(v *immutable.Map[string, interface{}]) *WorkflowStateBuilder {
	b.investigationPlan = v
	return b
}
func (b *WorkflowStateBuilder) WithInvestigationResults(v *immutable.Map[string, interface{}]) *WorkflowStateBuilder {
	b.investigationResults = v
	return b
}
func (b *WorkflowStateBuilder) WithCompletedWorkstreams(v *immutable.Map[string, interface{}]) *WorkflowStateBuilder {
	b.completedWorkstreams = v
	return b
}
func (b *WorkflowStateBuilder) WithWorkstreams(v *immutable.List[interface{}]) *WorkflowStateBuilder {
	b.workstreams = v
	return b
}
func (b *WorkflowStateBuilder) WithInvestigationReport(v string) *WorkflowStateBuilder {
	b.investigationReport = v
	return b
}
func (b *WorkflowStateBuilder) WithSpeculativeExpansions(v *immutable.List[interface{}]) *WorkflowStateBuilder {
	b.speculativeExpansions = v
	return b
}
func (b *WorkflowStateBuilder) WithOut(v string) *WorkflowStateBuilder {
	b.out = v
	return b
}
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

// Conversion helpers for WorkflowState fields

func ListMapToImmutableList(m []map[string]interface{}) *immutable.List[map[string]interface{}] {
	if m == nil {
		return nil
	}
	result := immutable.NewList[map[string]interface{}]()
	for _, v := range m {
		result = result.Append(v)
	}
	return result
}

// StringMapToImmutable converts map[string]string to *immutable.Map[string, string].
func StringMapToImmutable(m map[string]string) *immutable.Map[string, string] {
	if m == nil {
		return nil
	}
	result := immutable.NewMap[string, string](nil)
	for k, v := range m {
		result = result.Set(k, v)
	}
	return result
}

// ImmutableStringMapToRegular converts *immutable.Map[string, string] to map[string]string.
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

// ToRegularInterfaceMap converts *immutable.Map[string, interface{}] to map[string]interface{} for JSON marshaling.
func ToRegularInterfaceMap(m *immutable.Map[string, interface{}]) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{})
	itr := m.Iterator()
	itr.First()
	for !itr.Done() {
		k, v, _ := itr.Next()
		result[k] = v
	}
	return result
}

// =========================
// WORKFLOW STEP
// =========================

// StepType identifies the kind of workflow step.
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

// WorkflowStep represents a single step on the workflow stack.
type WorkflowStep struct {
	stepType            StepType
	state               *WorkflowState
	domainID            string
	workstreamID        string
	iteration           int
	reviewIteration     int
	codeReviewIteration int
	payload             interface{}
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
	payload             interface{}
	domainIndex         int
	workstreamIndex     int
}

// Build returns a new WorkflowStep with the builder's field values.
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

// --- Reader methods ---

func (ws *WorkflowStep) Type() StepType           { return ws.stepType }
func (ws *WorkflowStep) State() *WorkflowState    { return ws.state }
func (ws *WorkflowStep) DomainID() string         { return ws.domainID }
func (ws *WorkflowStep) WorkstreamID() string     { return ws.workstreamID }
func (ws *WorkflowStep) Iteration() int           { return ws.iteration }
func (ws *WorkflowStep) ReviewIteration() int     { return ws.reviewIteration }
func (ws *WorkflowStep) CodeReviewIteration() int { return ws.codeReviewIteration }
func (ws *WorkflowStep) Payload() interface{}     { return ws.payload }
func (ws *WorkflowStep) DomainIndex() int         { return ws.domainIndex }
func (ws *WorkflowStep) WorkstreamIndex() int     { return ws.workstreamIndex }

// --- Builder field setters (return the builder for chaining) ---

func (b *WorkflowStepBuilder) WithStepType(v StepType) *WorkflowStepBuilder {
	b.stepType = v
	return b
}
func (b *WorkflowStepBuilder) WithState(v *WorkflowState) *WorkflowStepBuilder {
	b.state = v
	return b
}
func (b *WorkflowStepBuilder) WithDomainID(v string) *WorkflowStepBuilder {
	b.domainID = v
	return b
}
func (b *WorkflowStepBuilder) WithWorkstreamID(v string) *WorkflowStepBuilder {
	b.workstreamID = v
	return b
}
func (b *WorkflowStepBuilder) WithIteration(v int) *WorkflowStepBuilder {
	b.iteration = v
	return b
}
func (b *WorkflowStepBuilder) WithReviewIteration(v int) *WorkflowStepBuilder {
	b.reviewIteration = v
	return b
}
func (b *WorkflowStepBuilder) WithCodeReviewIteration(v int) *WorkflowStepBuilder {
	b.codeReviewIteration = v
	return b
}
func (b *WorkflowStepBuilder) WithPayload(v interface{}) *WorkflowStepBuilder {
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

// Push constructs a WorkflowStep from the builder and pushes it onto the stack.
func (b *WorkflowStepBuilder) Push() {
	b.stack.Push(b.Build())
}

// =========================
// WORKFLOW STACK
// =========================

// WorkflowStack is a LIFO stack of WorkflowStep values.
// All modifications go through Push and Pop so that the underlying
// slice is never mutated directly from outside this package.
type WorkflowStack struct {
	stack []*WorkflowStep
}

// Push adds a step to the top of the stack.
func (ws *WorkflowStack) Push(s *WorkflowStep) {
	ws.stack = append(ws.stack, s)
}

// NewStep creates a new WorkflowStepBuilder pre-seeded with step type and state,
// then returns it so additional With* methods can be chained before Push.
func (ws *WorkflowStack) NewStep(stepType StepType, state *WorkflowState) *WorkflowStepBuilder {
	return &WorkflowStepBuilder{
		stack:    ws,
		stepType: stepType,
		state:    state,
	}
}

// Pop removes and returns the top step. Returns ok=false when empty.
func (ws *WorkflowStack) Pop() (*WorkflowStep, bool) {
	if len(ws.stack) == 0 {
		return nil, false
	}
	idx := len(ws.stack) - 1
	v := ws.stack[idx]
	ws.stack = ws.stack[:idx]
	return v, true
}
