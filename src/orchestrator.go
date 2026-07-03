package main

import (
	dt "agent-go/gen"
	"agent-go/pkg/loader"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"agent-go/state"
	"agent-go/wman"
	"github.com/benbjohnson/immutable"
	"github.com/datadog/zstd"
	"gitlab.com/golang-utils/isnil"

	td "agent-go/test_data"
)

type CombinedReviewPayload struct {
	GapReview  dt.GapAnalysisReview  `json:"gap_review"`
	FactReview dt.FactCheckingReview `json:"fact_review"`
}

type StructuralReviewPayload struct {
	QualityReview dt.InvestigationPlanQualityReview `json:"quality_review"`
	StructReview  dt.StructuralReview               `json:"struct_review"`
}

type Context struct {
	Tracer *Tracer

	watchmanHook     func() map[string]string
	runCodexHook     func(agentName, session, prompt string, schema map[string]interface{}, timeout string) (string, string, error)
	markdownDocHook  func(content interface{}, stageName string, subdir []string) string
	resetHook        func(agentName, sessionSuffix string)
	runJSONAgentHook func(agentName, invocationID, prompt string) *td.AgentState
}

func NewContext() *Context {
	return &Context{Tracer: &Tracer{}}
}

type Orchestrator struct {
	Agents  *AgentRegistry
	Watcher *wman.Watchman
	Stack   state.WorkflowStack
	err     interface{}
}

type AgentRegistry struct {
	ProductManager             *Agent[dt.ProductManager]
	PMSynth                    *Agent[dt.PmSynthesizer]
	PMExpansionCleanup         *Agent[dt.PmExpansionCleanup]
	NextStepsCleanup           *Agent[dt.NonCoderNextStepsCleanup]
	PMReview                   *Agent[dt.PmReview]
	DesignCleanup              *Agent[dt.DesignToImplementPhrasing]
	Arch                       *Agent[*dt.Arch]
	TechLead                   *Agent[*dt.Plan]
	Coder                      *Agent[*dt.Coder]
	ArchReview                 *Agent[*dt.ArchReview]
	PlanReview                 *Agent[*dt.PlanReview]
	CodeReview                 *Agent[*dt.CodeReview]
	TechLeadFinal              *Agent[*dt.TechLeadFinal]
	ArchFinal                  *Agent[dt.ArchFinal]
	Decomposition              *Agent[*dt.SystemDecomposition]
	DecompositionReview        *Agent[dt.SystemDecompositionReview]
	InvestigationClassifier    *Agent[dt.InvestigationClassifier]
	InvestigatorPlanner        *Agent[dt.InvestigatorPlan]
	InvestigatorExecutor       *Agent[dt.InvestigatorFindings]
	SynthesisAgent             *Agent[dt.InvestigationReport]
	GapAnalysisReviewer        *Agent[dt.GapAnalysisReview]
	FactCheckingReviewer       *Agent[dt.FactCheckingReview]
	StructuralReviewer         *Agent[dt.StructuralReview]
	InvestigationPlanQuality   *Agent[dt.InvestigationPlanQualityReview]
	SynthesisConsistencyReview *Agent[dt.SynthesisConsistencyReview]
}

func NewOrchestrator(task string, subdir string) *Orchestrator {
	o := &Orchestrator{}
	o.Agents = NewAgentRegistry(subdir)
	o.Watcher = wman.NewWatchman(subdir)
	o.Watcher.Start()
	return o
}

func NewAgentRegistry(subdir string) *AgentRegistry {
	ar := &AgentRegistry{}

	ar.ProductManager = NewAgent[dt.ProductManager](
		"product_manager",
		loader.PRODUCT_MANAGER_PROMPT,
		loader.PRODUCT_MANAGER_SCHEMA,
		subdir,
		WithEphemeral[dt.ProductManager](true),
		WithTimeout[dt.ProductManager]("30m"),
	)
	ar.PMSynth = NewAgent[dt.PmSynthesizer](
		"pm_synth",
		loader.PM_SYNTHESIZER_PROMPT,
		loader.PM_SYNTHESIZER_SCHEMA,
		subdir,
		WithTimeout[dt.PmSynthesizer]("30m"),
	)
	ar.PMExpansionCleanup = NewAgent[dt.PmExpansionCleanup](
		"pm_expansion_cleanup",
		loader.PM_EXPANSION_CLEANUP_PROMPT,
		loader.PM_EXPANSION_CLEANUP_SCHEMA,
		subdir,
		WithEphemeral[dt.PmExpansionCleanup](true),
		WithTimeout[dt.PmExpansionCleanup]("10m"),
	)
	ar.NextStepsCleanup = NewAgent[dt.NonCoderNextStepsCleanup](
		"next_steps_cleanup",
		loader.NON_CODER_NEXT_STEPS_CLEANUP_PROMPT,
		loader.NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA,
		subdir,
		WithEphemeral[dt.NonCoderNextStepsCleanup](true),
		WithTimeout[dt.NonCoderNextStepsCleanup]("10m"),
	)
	ar.PMReview = NewAgent[dt.PmReview](
		"pm_review",
		loader.PM_REVIEW_PROMPT,
		loader.PM_REVIEW_SCHEMA,
		subdir,
		WithEphemeral[dt.PmReview](true),
		WithTimeout[dt.PmReview]("30m"),
		WithResume[dt.PmReview](loader.ReviewerResume),
	)

	ar.DesignCleanup = NewAgent[dt.DesignToImplementPhrasing](
		"design_cleanup",
		loader.DESIGN_TO_IMPLEMENT_PHRASING_PROMPT,
		loader.DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA,
		subdir,
		WithEphemeral[dt.DesignToImplementPhrasing](true),
		WithTimeout[dt.DesignToImplementPhrasing]("10m"),
	)

	ar.Arch = NewAgent[*dt.Arch](
		"arch", loader.ARCH_PROMPT, loader.ARCH_SCHEMA, subdir, WithTimeout[*dt.Arch]("40m"),
	)
	ar.TechLead = NewAgent[*dt.Plan](
		"tech_lead", loader.PLAN_PROMPT, loader.PLAN_SCHEMA, subdir, WithTimeout[*dt.Plan]("60m"),
	)
	ar.Coder = NewAgent[*dt.Coder](
		"coder", loader.CODER_PROMPT, loader.CODER_SCHEMA, subdir, WithTimeout[*dt.Coder]("180m"),
	)

	ar.ArchReview = NewAgent[*dt.ArchReview](
		"arch_review", loader.ARCH_REVIEW_PROMPT, loader.ARCH_REVIEW_SCHEMA, subdir,
		WithEphemeral[*dt.ArchReview](true), WithTimeout[*dt.ArchReview]("30m"),
		WithResume[*dt.ArchReview](loader.ReviewerResume),
	)

	ar.PlanReview = NewAgent[*dt.PlanReview](
		"plan_review", loader.PLAN_REVIEW_PROMPT, loader.PLAN_REVIEW_SCHEMA, subdir,
		WithEphemeral[*dt.PlanReview](true), WithTimeout[*dt.PlanReview]("30m"),
		WithResume[*dt.PlanReview](loader.ReviewerResume),
	)

	ar.CodeReview = NewAgent[*dt.CodeReview](
		"code_review", loader.CODE_REVIEW_PROMPT, loader.CODE_REVIEW_SCHEMA, subdir,
		WithEphemeral[*dt.CodeReview](true), WithTimeout[*dt.CodeReview]("60m"),
		WithResume[*dt.CodeReview](loader.ReviewerResume),
	)

	ar.TechLeadFinal = NewAgent[*dt.TechLeadFinal](
		"tech_lead_final", loader.TECH_LEAD_FINAL_PROMPT, loader.TECH_LEAD_FINAL_SCHEMA, subdir,
		WithEphemeral[*dt.TechLeadFinal](true), WithTimeout[*dt.TechLeadFinal]("60m"),
		WithResume[*dt.TechLeadFinal](loader.ReviewerResume),
	)

	ar.ArchFinal = NewAgent[dt.ArchFinal](
		"arch_final", loader.ARCH_FINAL_PROMPT, loader.ARCH_FINAL_SCHEMA, subdir,
		WithEphemeral[dt.ArchFinal](true), WithTimeout[dt.ArchFinal]("60m"),
		WithResume[dt.ArchFinal](loader.ReviewerResume),
	)

	ar.Decomposition = NewAgent[*dt.SystemDecomposition](
		"decomposition", loader.SYSTEM_DECOMPOSITION_PROMPT, loader.SYSTEM_DECOMPOSITION_SCHEMA,
		subdir, WithTimeout[*dt.SystemDecomposition]("40m"),
	)
	ar.DecompositionReview = NewAgent[dt.SystemDecompositionReview](
		"decomposition_review", loader.SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
		loader.SYSTEM_DECOMPOSITION_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.SystemDecompositionReview](true), WithTimeout[dt.SystemDecompositionReview]("30m"),
		WithResume[dt.SystemDecompositionReview](loader.ReviewerResume),
	)

	ar.InvestigationClassifier = NewAgent[dt.InvestigationClassifier](
		"investigation_classifier", loader.INVESTIGATION_CLASSIFIER_PROMPT,
		loader.INVESTIGATION_CLASSIFIER_SCHEMA, subdir,
		WithEphemeral[dt.InvestigationClassifier](true), WithTimeout[dt.InvestigationClassifier]("10m"),
	)
	ar.InvestigatorPlanner = NewAgent[dt.InvestigatorPlan](
		"investigator_planner", loader.INVESTIGATOR_PLANNER_PROMPT,
		loader.INVESTIGATOR_PLAN_SCHEMA, subdir, WithTimeout[dt.InvestigatorPlan]("90m"),
	)
	ar.InvestigatorExecutor = NewAgent[dt.InvestigatorFindings](
		"investigator_executor", loader.INVESTIGATOR_EXECUTOR_PROMPT,
		loader.INVESTIGATOR_FINDINGS_SCHEMA, subdir, WithTimeout[dt.InvestigatorFindings]("180m"),
	)
	ar.SynthesisAgent = NewAgent[dt.InvestigationReport](
		"synthesis_agent", loader.SYNTHESIS_PROMPT, loader.INVESTIGATION_REPORT_SCHEMA,
		subdir, WithEphemeral[dt.InvestigationReport](true), WithTimeout[dt.InvestigationReport]("90m"),
	)
	ar.GapAnalysisReviewer = NewAgent[dt.GapAnalysisReview](
		"gap_analysis_reviewer", loader.GAP_ANALYSIS_REVIEW_PROMPT,
		loader.GAP_ANALYSIS_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.GapAnalysisReview](true), WithTimeout[dt.GapAnalysisReview]("60m"),
		WithResume[dt.GapAnalysisReview](loader.ReviewerResume),
	)
	ar.FactCheckingReviewer = NewAgent[dt.FactCheckingReview](
		"fact_checking_reviewer", loader.FACT_CHECKING_REVIEW_PROMPT,
		loader.FACT_CHECKING_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.FactCheckingReview](true), WithTimeout[dt.FactCheckingReview]("60m"),
		WithResume[dt.FactCheckingReview](loader.ReviewerResume),
	)
	ar.StructuralReviewer = NewAgent[dt.StructuralReview](
		"structural_reviewer", loader.STRUCTURE_REVIEW_PROMPT,
		loader.STRUCTURAL_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.StructuralReview](true), WithTimeout[dt.StructuralReview]("30m"),
		WithResume[dt.StructuralReview](loader.ReviewerResume),
	)
	ar.InvestigationPlanQuality = NewAgent[dt.InvestigationPlanQualityReview](
		"investigation_plan_quality_reviewer", loader.INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT,
		loader.INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.InvestigationPlanQualityReview](true), WithTimeout[dt.InvestigationPlanQualityReview]("30m"),
		WithResume[dt.InvestigationPlanQualityReview](loader.ReviewerResume),
	)
	ar.SynthesisConsistencyReview = NewAgent[dt.SynthesisConsistencyReview](
		"synthesis_consistency_reviewer", loader.SYNTHESIS_CONSISTENCY_REVIEW_PROMPT,
		loader.SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.SynthesisConsistencyReview](true), WithTimeout[dt.SynthesisConsistencyReview]("60m"),
		WithResume[dt.SynthesisConsistencyReview](loader.ReviewerResume),
	)

	return ar
}

func wrapText(text string) string {
	return fmt.Sprintf("<text>\n%s\n</text>", text)
}

func sortDomainsTyped(domains []dt.SystemDecompositiondecompositiondomainsElem) []int {
	g := buildDependencyGraphTyped(domains, "UpstreamDependencies")

	allResolved := true
	for i := 0; i < len(domains) && allResolved; i++ {
		if g.inDeg[i] != g.depKeys[i] {
			allResolved = false
		}
	}

	if allResolved {
		original := make([]int, len(domains))
		for i := range domains {
			original[i] = i
		}
		return original
	}

	type depEntry struct {
		index    int
		depCount int
	}
	entries := make([]depEntry, len(domains))
	for i := range domains {
		entries[i] = depEntry{index: i, depCount: g.depKeys[i]}
	}
	slices.SortStableFunc(entries, func(a, b depEntry) int {
		if a.depCount != b.depCount {
			return a.depCount - b.depCount
		}
		return a.index - b.index
	})
	sorted := make([]int, len(entries))
	for i, e := range entries {
		sorted[i] = e.index
	}
	return sorted
}

func buildDependencyGraphTyped(domains []dt.SystemDecompositiondecompositiondomainsElem, depsKey string) dependencyGraphTyped {
	g := dependencyGraphTyped{
		idToIdx: make(map[string]int),
		inDeg:   make(map[int]int),
		depKeys: make(map[int]int),
	}
	for i, domain := range domains {
		g.inDeg[i] = 0
		g.depKeys[i] = 0
		if depsKey == "UpstreamDependencies" {
			g.depKeys[i] = domain.UpstreamDependencies().Len()
			depsItr := domain.UpstreamDependencies().Iterator()
			depsItr.First()
			for !depsItr.Done() {
				_, depStr := depsItr.Next()
				depID := strings.ToLower(strings.TrimSpace(depStr))
				if _, exists := g.idToIdx[depID]; exists {
					g.inDeg[i]++
				}
			}
		}
		g.idToIdx[strings.ToLower(strings.TrimSpace(domain.Id()))] = i
	}
	return g
}

type dependencyGraphTyped struct {
	idToIdx map[string]int
	inDeg   map[int]int
	depKeys map[int]int
}

func (o *Orchestrator) Run(task string, subdir string) {
	currentState := state.NewWorkflowStateBuilder(nil).
		WithTask(wrapText(task)).
		WithSubdir(subdir).
		WithDomains(state.WorkflowStatedomains(*immutable.NewMap[string, state.DomainState](nil))).
		WithInvestigationResults(nil).
		WithCompletedWorkstreams(state.WorkflowStatecompletedworkstreams(*immutable.NewMap[string, dt.InvestigatorFindings](nil))).
		Build()

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePMCandidateGenerate, currentState).Build())

	var traceFile *os.File
	if dest := os.Getenv("AGENT_TRACE_FILE"); dest != "" {
		f, err := os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			panic(err)
		}
		traceFile = f
	}

	defer func() {
		if traceFile != nil {
			traceFile.Close()
		}
	}()

	var traceWriter *zstd.Writer
	if traceFile != nil {
		traceWriter = zstd.NewWriterLevel(traceFile, zstd.BestCompression)
	}

	o._Run(traceWriter)

	if traceWriter != nil {
		traceWriter.Close()
	}

	if o.err != nil {
		panic(o.err)
	}
}

func (o *Orchestrator) _Run(traceWriter io.Writer) {
	defer func() {
		o.err = recover()
	}()
	for {
		step, ok := o.Stack.Pop()
		if !ok {
			break
		}
		nextStepIdx := o.Stack.Len()
		c := NewContext()

		o.executeStep(step, c)

		followUpSteps := o.Stack.GetFrom(nextStepIdx)

		if !isnil.IsNil(traceWriter) {
			traceStack(traceWriter, step, followUpSteps, c.Tracer.Actions)
		}
	}
}

func (o *Orchestrator) executeStep(step *state.WorkflowStep, context *Context) {
	switch step.StepType() {
	case state.WorkflowStepsteptypePMCandidateGenerate:
		o.executePMCandidateGenerate(step, context)
	case state.WorkflowStepsteptypePMSynthesize:
		o.executePMSynthesize(step, context)
	case state.WorkflowStepsteptypePMReview:
		o.executePMReview(step, context)
	case state.WorkflowStepsteptypeClassification:
		o.executeClassification(step, context)
	case state.WorkflowStepsteptypeDecomposition:
		o.executeDecomposition(step, context)
	case state.WorkflowStepsteptypeDecompositionReview:
		o.executeDecompositionReview(step, context)
	case state.WorkflowStepsteptypeInvestigationPlanGenerate:
		o.executeInvestigationPlanGenerate(step, context)
	case state.WorkflowStepsteptypeInvestigationPlanReview:
		o.executeInvestigationPlanReview(step, context)
	case state.WorkflowStepsteptypeInvestigationWorkstream:
		o.executeInvestigationWorkstream(step, context)
	case state.WorkflowStepsteptypeInvestigationWorkstreamReview:
		o.executeInvestigationWorkstreamReview(step, context)
	case state.WorkflowStepsteptypeInvestigationSynthesis:
		o.executeInvestigationSynthesis(step, context)
	case state.WorkflowStepsteptypeInvestigationConsistencyReview:
		o.executeInvestigationConsistencyReview(step, context)
	case state.WorkflowStepsteptypeDomainStart:
		o.executeDomainStart(step, context)
	case state.WorkflowStepsteptypeArchitecture:
		o.executeArchitecture(step, context)
	case state.WorkflowStepsteptypeArchitectureReview:
		o.executeArchitectureReview(step, context)
	case state.WorkflowStepsteptypePlan:
		o.executePlan(step, context)

	case state.WorkflowStepsteptypePlanReview:
		o.executePlanReview(step, context)
	case state.WorkflowStepsteptypeCodeImplementation:
		o.executeCodeImplementation(step, context)
	case state.WorkflowStepsteptypeCodeReview:
		o.executeCodeReview(step, context)
	case state.WorkflowStepsteptypeTechLeadReview:
		o.executeTechLeadReview(step, context)
	case state.WorkflowStepsteptypeArchitectureFinalReview:
		o.executeArchitectureFinalReview(step, context)
	case state.WorkflowStepsteptypeFinalInvestigation:
		o.executeFinalInvestigation(step, context)
	case state.WorkflowStepsteptypePMExpansionCleanup:
		o.executePMExpansionCleanup(step, context)
	case state.WorkflowStepsteptypeInvestigationPlanStructuralReview:
		o.executeInvestigationPlanStructuralReview(step, context)
	case state.WorkflowStepsteptypeInvestigationFactReview:
		o.executeInvestigationFactReview(step, context)
	case state.WorkflowStepsteptypeDecompositionOrchestrator:
		o.executeDecompositionOrchestrator(step, context)
	case state.WorkflowStepsteptypeArchitectureOrchestrator:
		o.executeArchitectureOrchestrator(step, context)
	case state.WorkflowStepsteptypePlanOrchestrator:
		o.executePlanOrchestrator(step, context)
	case state.WorkflowStepsteptypeCodeReviewOrchestrator:
		o.executeCodeReviewOrchestrator(step, context)
	case state.WorkflowStepsteptypeTechLeadReviewOrchestrator:
		o.executeTechLeadReviewOrchestrator(step, context)
	case state.WorkflowStepsteptypeArchitectureFinalReviewOrchestrator:
		o.executeArchitectureFinalReviewOrchestrator(step, context)
	case state.WorkflowStepsteptypeInvestigationFactReviewOrchestrator:
		o.executeInvestigationFactReviewOrchestrator(step, context)
	case state.WorkflowStepsteptypeInvestigationConsistencyReviewOrchestrator:
		o.executeInvestigationConsistencyReviewOrchestrator(step, context)
	case state.WorkflowStepsteptypeTechLeadEnd:
		o.executeTechLeadEnd(step, context)
	case state.WorkflowStepsteptypeBoundary:
		o.executeBoundary(step, context)
	default:
		logStep(fmt.Sprintf("Unknown step type: %v", step.StepType()), "SYSTEM")
	}
}

func (o *Orchestrator) executePMCandidateGenerate(step *state.WorkflowStep, context *Context) {
	candidates := []dt.ProductManager{}
	biases := []string{
		"Bias toward minimal scope and preserving the literal user request.",
		"Bias toward UX completeness, validation rules, and expected user behavior.",
		"Bias toward implementation simplicity and minimal engineering risk.",
		"Bias toward edge cases, state transitions, and failure scenarios.",
		"Bias toward preserving existing system behavior and minimizing changes to current workflows.",
		"Bias toward permissions, roles, ownership boundaries, and access control behavior.",
		"Bias toward data model implications, persistence behavior, lifecycle management, and state consistency.",
		"Bias toward API behavior, input/output contracts, validation, and error handling.",
		"Bias toward reporting, auditability, notifications, logging, and observability requirements.",
		"Bias toward backward compatibility, migration concerns, rollout safety, and minimizing disruption to existing users.",
		"Bias toward operational concerns such as performance, scalability, concurrency, and long-term maintainability.",
		"Bias toward identifying the smallest possible implementation that still fully satisfies the request.",
		"Bias toward identifying where the request may be overcomplicated, unnecessary, or better solved through a smaller existing workflow change instead of a new feature.",
		"Bias toward preserving only what is explicitly stated by the user. Avoid assumptions unless absolutely necessary.",
		"Bias toward identifying the user's likely business goal and ensuring the specification solves that goal with the smallest possible feature set.",
	}

	for idx, bias := range biases {
		candidate := RunJSONAgent(
			o.Agents.ProductManager,
			fmt.Sprintf("USER REQUEST:\n%s\n\nTASK:\nProduce a focused engineering-ready specification.\n%s", step.State().Task(), bias),
			fmt.Sprintf("pm-spec-candidate-%d", idx),
			[]string{step.State().Subdir()}, context,
		)
		candidates = append(candidates, candidate)
	}
	choices := ""
	for i, v := range candidates {
		serialized := MarshalJSON(v)
		choices += fmt.Sprintf("CANDIDATE %d:\n%s\n", i+1, serialized)
		if i < len(candidates)-1 {
			choices += "\n"
		}
	}
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePMSynthesize, state.NewWorkflowStateBuilder(step.State()).WithChoices(choices).Build()).WithIteration(0).Build())
}

func (o *Orchestrator) executePMSynthesize(step *state.WorkflowStep, context *Context) {
	payload := ""
	if step.Payload() != nil {
		payload = step.Payload().RevisionPrompt()
	}
	var prompt string
	var invocationID string
	if step.Iteration() == 0 {
		prompt = fmt.Sprintf("ORIGINAL USER REQUEST:\n%s\n\n%sTASK:\nSelect the single best interpretation of the original user request.Preserve only the minimum assumptions necessary.Reject speculative scope expansion.", step.State().Task(), step.State().Choices())
		invocationID = "pm-spec-synthesis-0"
	} else {
		prompt = payload
		invocationID = fmt.Sprintf("pm-spec-synthesis-%d", step.Iteration())
	}
	rephrasedTask := RunJSONAgent(
		o.Agents.PMSynth, prompt, invocationID, []string{step.State().Subdir()}, context)
	modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(&rephrasedTask).Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePMReview, modState).WithIteration(step.Iteration()).Build())
}

func (o *Orchestrator) executePMReview(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	rephrasedTask := step.State().RephrasedTask()
	review := RunJSONAgent(
		o.Agents.PMReview,
		fmt.Sprintf("ORIGINAL USER REQUEST:\n%s\n\nSYNTHESIZED SPECIFICATION:\n%s\n\nATTEMPT: %d/%d\n\nTASK:\nReview whether the synthesized specification correctly preserves the original user intent.Reject only if the specification is ambiguous, speculative, internally inconsistent, or over-expanded.",
			step.State().Task(),
			MarshalJSON(rephrasedTask),
			iteration+1,
			MAXPlanIters,
		),
		fmt.Sprintf("pm-spec-review-%d", iteration),
		[]string{step.State().Subdir()}, context,
	)

	if reviewOk[dt.PmReviewissuesElemseverity](&review) {
		speculativeExpansions := rephrasedTask.SpeculativeExpansions()
		hasExpansions := false
		if speculativeExpansions.Len() > 0 {
			hasExpansions = true
		}
		modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(rephrasedTask).WithSpeculativeExpansions(speculativeExpansions).Build()
		if hasExpansions {
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePMExpansionCleanup, modState).Build())
			return
		}

		pmFilepath := MarkdownDocumentGenerator(rephrasedTask, "product_manager_final", []string{step.State().Subdir()}, context)

		expandedModState := state.NewWorkflowStateBuilder(modState).WithOut(buildPMtask(*rephrasedTask)).WithPmFilepath(pmFilepath).Build()
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, expandedModState).Build())
		return
	}

	var revisionPrompt string
	if shouldReset(&review) {
		o.Agents.PMSynth.Reset(context, fmt.Sprintf("%d", iteration))
		revisionPrompt = fmt.Sprintf(
			"ORIGINAL USER REQUEST:\n%s\n\n%sPREVIOUS SYNTHESIZED SPECIFICATION:\n%s\nREVIEW FEEDBACK:\n%s\nTASK:\nRevise the synthesized specification to address the review feedback while preserving the original user intent and keeping the scope minimal.",
			step.State().Task(), step.State().Choices(), MarshalJSON(rephrasedTask), MarshalJSON(review),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE SYNTHESIZED SPECIFICATION based on feedback:\n%s", MarshalJSON(review))
	}

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePMSynthesize, step.State()).WithPayload(state.NewPayloadDataBuilder(nil).WithRevisionPrompt(revisionPrompt).Build()).WithIteration(iteration + 1).Build())
}

func (o *Orchestrator) executePMExpansionCleanup(step *state.WorkflowStep, context *Context) {
	rephrasedTask := step.State().RephrasedTask()
	speculativeExpansions := step.State().SpeculativeExpansions()
	expSlice := make([]string, speculativeExpansions.Len())
	itr := speculativeExpansions.Iterator()
	itr.First()
	for i := 0; i < speculativeExpansions.Len(); i++ {
		_, v := itr.Next()
		expSlice[i] = v
	}
	for {
		cleanSpeculative := RunJSONAgent(
			o.Agents.PMExpansionCleanup,
			fmt.Sprintf("INPUT JSON:\n%s", MarshalJSON(map[string]interface{}{"lines": expSlice})),
			"pm-expansion-cleanup",
			[]string{step.State().Subdir()}, context,
		)
		cleanLines := cleanSpeculative.Lines()
		cleanSlice := immutableListToSlice(cleanLines)
		if cleanLines.Len() == len(expSlice) {
			expSlice = cleanSlice
			break
		}
		expSlice = cleanSlice
	}
	rephrasedTask = updateSpeculativeExpansions(rephrasedTask, expSlice)
	pmFilepath := MarkdownDocumentGenerator(rephrasedTask, "product_manager_final", []string{step.State().Subdir()}, context)
	modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(rephrasedTask).WithOut(buildPMtask(*rephrasedTask)).WithPmFilepath(pmFilepath).Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).Build())
}

func buildPMtask(rephrasedTask dt.PmSynthesizer) string {
	out := ""
	if rephrasedTask.TaskSpecification() != "" {
		out = rephrasedTask.TaskSpecification()
	}
	files := immutableListToSlice(rephrasedTask.Files())
	if len(files) > 0 {
		for i, f := range files {
			if i == 0 {
				out += fmt.Sprintf("\n\nMentioned files:\n* %s", f)
			} else {
				out += fmt.Sprintf("\n* %s", f)
			}
		}
	}
	properNouns := immutableListToSlice(rephrasedTask.ProperNouns())
	if len(properNouns) > 0 {
		for i, p := range properNouns {
			if i == 0 {
				out += fmt.Sprintf("\n\nMentioned proper nouns:\n* %s", p)
			} else {
				out += fmt.Sprintf("\n* %s", p)
			}
		}
	}
	facts := immutableListToSlice(rephrasedTask.Facts())
	if len(facts) > 0 {
		for i, f := range facts {
			if i == 0 {
				out += fmt.Sprintf("\n\nStated facts:\n* %s", f)
			} else {
				out += fmt.Sprintf("\n* %s", f)
			}
		}
	}
	missingDetails := immutableListToSlice(rephrasedTask.MissingButNecessaryDetails())
	if len(missingDetails) > 0 {
		for i, f := range missingDetails {
			if i == 0 {
				out += fmt.Sprintf("\n\nAdditional considerations:\n* %s", f)
			} else {
				out += fmt.Sprintf("\n* %s", f)
			}
		}
	}
	specExpansions := immutableListToSlice(rephrasedTask.SpeculativeExpansions())
	if len(specExpansions) > 0 {
		for i, f := range specExpansions {
			if i == 0 {
				out += fmt.Sprintf("\n\n\nOut of scope:\n* %s", f)
			} else {
				out += fmt.Sprintf("\n* %s", f)
			}
		}
	}

	out += "\n"
	return out
}

func (o *Orchestrator) executeClassification(step *state.WorkflowStep, context *Context) {
	classification := RunJSONAgent(
		o.Agents.InvestigationClassifier,
		fmt.Sprintf("REFINED TASK SPECIFICATION:\n%s", wrapText(step.State().Out())),
		"investigation-classifier",
		[]string{step.State().Subdir()}, context,
	)

	modState := state.NewWorkflowStateBuilder(step.State()).Build()
	taskType := string(classification.AType())
	logStep(fmt.Sprintf("Task classified as: %s", taskType), "CLASSIFICATION")
	logStep(fmt.Sprintf("Reasoning: %s", classification.Reasoning()), "CLASSIFICATION")
	MarkdownDocumentGenerator(classification, "investigation_classification", []string{step.State().Subdir()}, context)

	if taskType == "investigation" {
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationPlanGenerate, modState).Build())
	} else {
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeDecomposition, modState).Build())
	}
}

func (o *Orchestrator) executeDecomposition(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	var prompt string
	var invocationID string
	if iteration == 0 {
		prompt = fmt.Sprintf("TASK:\n%s", wrapText(step.State().Out()))
		invocationID = "decomposition-0"
	} else {

		review := step.Payload().DecompositionReview()
		if shouldReset(review) {
			o.Agents.Decomposition.Reset(context, fmt.Sprintf("%d", iteration-1))
			prompt = fmt.Sprintf(
				"TASK:\n%s\nPREVIOUS DECOMPOSITION:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the decomposition from scratch using the original task and review feedback.",
				wrapText(step.State().Out()), MarshalJSON(step.State().DecompositionResult()), MarshalJSON(review),
			)
		} else {
			prompt = fmt.Sprintf("REVISE DECOMPOSITION based on feedback:\n%s", MarshalJSON(review))
		}
		invocationID = fmt.Sprintf("decomposition-%d", iteration)
	}
	decompositionResult := Nudge(
		100,
		o.Agents.Decomposition, prompt, invocationID, []string{step.State().Subdir()}, o.Agents.NextStepsCleanup, context)
	lastResult := decompositionResult[len(decompositionResult)-1].Out
	AssertNotEmpty(lastResult, "DECOMPOSITION")
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeDecompositionReview, state.NewWorkflowStateBuilder(step.State()).WithDecompositionResult(lastResult).Build()).WithReviewIteration(iteration).Build())
}

func (o *Orchestrator) executeDecompositionReview(step *state.WorkflowStep, context *Context) {
	iteration := step.ReviewIteration()
	resultMap := step.State().DecompositionResult()
	decompositionReview := RunJSONAgent(
		o.Agents.DecompositionReview,
		fmt.Sprintf("TASK:\n%s\nATTEMPT: %d/%d\nDECOMPOSITION TO REVIEW:\n%s",
			wrapText(step.State().Out()), iteration+1, MAXPlanIters, MarshalJSON(resultMap)),
		fmt.Sprintf("decomposition-review-%d", iteration),
		[]string{step.State().Subdir()}, context,
	)

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeDecompositionOrchestrator, step.State()).WithPayload(state.NewPayloadDataBuilder(nil).WithDecompositionReview(&decompositionReview).Build()).WithReviewIteration(iteration).Build())
}

func (o *Orchestrator) executeInvestigationPlanGenerate(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	var plan dt.InvestigatorPlan
	wrappedTask := wrapText(step.State().Out())
	if step.State().FinalInvestigationTask() != "" {
		wrappedTask = step.State().FinalInvestigationTask()
	}
	if iteration == 0 {

		plan = RunJSONAgent(
			o.Agents.InvestigatorPlanner,
			fmt.Sprintf("TASK:\n%s", wrappedTask),
			"investigation-plan",
			[]string{step.State().Subdir(), "investigation"}, context,
		)
	} else {

		payload := step.Payload().RevisionPrompt()
		plan = RunJSONAgent(
			o.Agents.InvestigatorPlanner,
			payload,
			fmt.Sprintf("investigation-plan-%d", iteration),
			[]string{step.State().Subdir(), "investigation"}, context,
		)
	}
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationPlanReview, state.NewWorkflowStateBuilder(step.State()).WithInvestigationPlan(&plan).Build()).WithIteration(iteration).Build())
}

func (o *Orchestrator) executeInvestigationPlanReview(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	plan := step.State().InvestigationPlan()

	var wrappedTask string
	if step.State().FinalInvestigationTask() != "" {
		wrappedTask = step.State().FinalInvestigationTask()
	} else {
		wrappedTask = wrapText(step.State().Out())
	}

	qualityReview := RunJSONAgent(
		o.Agents.InvestigationPlanQuality,
		fmt.Sprintf("TASK:\n%s\nPLAN TO REVIEW:\n%s", wrappedTask, MarshalJSON(plan)),
		fmt.Sprintf("investigation-plan_quality-review-%d", iteration),
		[]string{step.State().Subdir(), "investigation"}, context,
	)

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationPlanStructuralReview, step.State()).WithIteration(iteration).WithPayload(state.NewPayloadDataBuilder(nil).WithInvestigationPlanQualityReview(&qualityReview).Build()).Build())
}

func (o *Orchestrator) executeInvestigationPlanStructuralReview(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	plan := step.State().InvestigationPlan()
	qualityReview := step.Payload().InvestigationPlanQualityReview()

	var wrappedTask string
	if step.State().FinalInvestigationTask() != "" {
		wrappedTask = step.State().FinalInvestigationTask()
	} else {
		wrappedTask = wrapText(step.State().Out())
	}

	structReview := RunJSONAgent(
		o.Agents.StructuralReviewer,
		fmt.Sprintf("TASK:\n%s\nPLAN TO REVIEW:\n%s", wrappedTask, MarshalJSON(plan)),
		fmt.Sprintf("investigation-struct-review-%d", iteration),
		[]string{step.State().Subdir(), "investigation"}, context,
	)

	if reviewOk[dt.InvestigationPlanQualityReviewissuesElemseverity](qualityReview) && reviewOk[dt.StructuralReviewissuesElemseverity](&structReview) {
		MarkdownDocumentGenerator(plan, "investigation_plan", []string{step.State().Subdir(), "investigation"}, context)
		workstreamsList := plan.Workstreams()
		workstreams := immutableListToSlice(workstreamsList)
		if len(workstreams) == 0 {
			return
		}

		modState := state.NewWorkflowStateBuilder(step.State()).WithWorkstreams(listToImmutableList(workstreams)).WithCompletedWorkstreams(state.WorkflowStatecompletedworkstreams(*immutable.NewMap[string, dt.InvestigatorFindings](nil))).Build()
		wsID := strings.ToLower(strings.TrimSpace(workstreams[0].Id()))
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationWorkstream, modState).WithWorkstreamID(wsID).WithWorkstreamIndex(0).Build())
		return
	}

	combinedReview := StructuralReviewPayload{
		QualityReview: *qualityReview,
		StructReview:  structReview,
	}

	var revisionPrompt string
	if shouldReset(qualityReview) || shouldReset(&structReview) {
		o.Agents.InvestigatorPlanner.Reset(context, fmt.Sprintf("investigation-plan-%d", iteration))
		revisionPrompt = fmt.Sprintf(
			"TASK:\n%s\nPREVIOUS PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the investigation plan from scratch using the original task and review feedback.",
			wrappedTask, MarshalJSON(plan), MarshalJSON(combinedReview),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE PLAN based on feedback:\n%s", MarshalJSON(combinedReview))
	}

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationPlanGenerate, step.State()).WithPayload(state.NewPayloadDataBuilder(nil).WithRevisionPrompt(revisionPrompt).Build()).WithIteration(iteration + 1).Build())
}

func lookupCompletedWorkstream(cw state.WorkflowStatecompletedworkstreams, key string) (dt.InvestigatorFindings, bool) {
	m := (*immutable.Map[string, dt.InvestigatorFindings])(&cw)
	return m.Get(key)
}

func getDomain(domains state.WorkflowStatedomains, key string) (state.DomainState, bool) {
	m := (*immutable.Map[string, state.DomainState])(&domains)
	return m.Get(key)
}

func setDomain(domains state.WorkflowStatedomains, key string, val *state.DomainState) state.WorkflowStatedomains {
	m := (*immutable.Map[string, state.DomainState])(&domains)
	var v state.DomainState
	if val != nil {
		v = *val
	}
	newM := m.Set(key, v)
	return state.WorkflowStatedomains(*newM)
}

func setCompletedWorkstream(cw state.WorkflowStatecompletedworkstreams, key string, val dt.InvestigatorFindings) state.WorkflowStatecompletedworkstreams {
	m := (*immutable.Map[string, dt.InvestigatorFindings])(&cw)
	newM := m.Set(key, val)
	return state.WorkflowStatecompletedworkstreams(*newM)
}

func immutableMapToRegular(m *immutable.Map[string, string]) map[string]string {
	result := make(map[string]string)
	iter := m.Iterator()
	iter.First()
	for !iter.Done() {
		k, v, _ := iter.Next()
		result[k] = v
	}
	return result
}
func immutableStringMapToRegular(m state.DomainStateallchanges) map[string]string {
	result := make(map[string]string)
	iter := (*immutable.Map[string, string])(&m).Iterator()
	iter.First()
	for !iter.Done() {
		k, v, _ := iter.Next()
		result[k] = v
	}
	return result
}
func listToImmutableList[T any](slice []T) *immutable.List[T] {
	if slice == nil {
		return nil
	}
	l := immutable.NewList[T]()
	for _, v := range slice {
		l = l.Append(v)
	}
	return l
}
func buildInvestigationWorkstreamPrompt(wsElem dt.InvestigatorPlanworkstreamsElem, completedWorkstreams state.WorkflowStatecompletedworkstreams) string {
	resolvedDeps := []dt.InvestigatorFindings{}
	if deps := wsElem.Dependencies(); deps != nil {
		depSlice := immutableListToSlice(*deps)
		for _, depID := range depSlice {
			depIDClean := strings.ToLower(strings.TrimSpace(depID))
			if findings, exists := lookupCompletedWorkstream(completedWorkstreams, depIDClean); exists {
				if !isEmptyFindings(findings) {
					resolvedDeps = append(resolvedDeps, findings)
				}
			}
		}
	}

	out := map[string]interface{}{ // XXX
		"data_sources":          immutableListToSlice(wsElem.DataSources()),
		"expected_deliverables": immutableListToSlice(wsElem.ExpectedDeliverables()),
		"hypotheses":            immutableListToSlice(wsElem.Hypotheses()),
		"id":                    wsElem.Id(),
		"investigation_methods": immutableListToSlice(wsElem.InvestigationMethods()),
		"objective":             wsElem.Objective(),
	}
	if len(resolvedDeps) > 0 {
		out["dependencies"] = resolvedDeps
	}
	return MarshalJSON(out)
}

func (o *Orchestrator) executeInvestigationWorkstream(step *state.WorkflowStep, context *Context) {
	workstreamID := step.WorkstreamID()
	if _, exists := lookupCompletedWorkstream(step.State().CompletedWorkstreams(), workstreamID); exists {

		if step.Payload() == nil {
			return
		}
	}

	workstreamIndex := step.WorkstreamIndex()
	domainIntID := workstreamIndex + 1
	sessionSuffix := fmt.Sprintf("d%d-start", domainIntID)
	iteration := step.Iteration()

	workstreams := step.State().Workstreams()
	wsElem := workstreams.Get(workstreamIndex)

	depsList := wsElem.Dependencies()
	if depsList != nil {
		depSlice := immutableListToSlice(*depsList)
		if len(depSlice) > 0 {
			canRun := true
			for _, depStr := range depSlice {
				depID := strings.ToLower(strings.TrimSpace(depStr))
				if _, exists := lookupCompletedWorkstream(step.State().CompletedWorkstreams(), depID); !exists {
					canRun = false
					break
				}
			}
			if !canRun {
				return
			}
		}
	}

	if iteration == 0 {
		o.Agents.InvestigatorExecutor.Reset(context, sessionSuffix)
	}

	hasHypotheses := wsElem.Hypotheses().Len() > 0
	hasDataSources := wsElem.DataSources().Len() > 0
	if !hasHypotheses || !hasDataSources {
		cw := step.State().CompletedWorkstreams()
		cwMap := (*immutable.Map[string, dt.InvestigatorFindings])(&cw)
		newCwMap := cwMap.Set(workstreamID, *dt.NewInvestigatorFindingsBuilder(nil).WithConfidenceLevel(dt.InvestigatorFindingsconfidencelevelLow).Build())
		cw = state.WorkflowStatecompletedworkstreams(*newCwMap)
		modState := state.NewWorkflowStateBuilder(step.State()).WithCompletedWorkstreams(cw).Build()

		workstreams := modState.Workstreams()
		for i := workstreamIndex + 1; i < workstreams.Len(); i++ {
			nextWs := workstreams.Get(i)
			nh := nextWs.Hypotheses().Len() > 0
			nds := nextWs.DataSources().Len() > 0
			if nh && nds {
				nid := strings.ToLower(strings.TrimSpace(nextWs.Id()))
				o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationWorkstream, modState).WithWorkstreamID(nid).WithWorkstreamIndex(i).Build())
				return
			}
		}
		return
	}

	var revisionPrompt string
	if step.Payload() != nil {
		revisionPrompt = step.Payload().RevisionPrompt()
	}

	subdir := []string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", domainIntID)}
	var prompt string
	if revisionPrompt != "" {
		prompt = revisionPrompt
	} else {
		prompt = fmt.Sprintf("WORKSTREAM:\n%s", buildInvestigationWorkstreamPrompt(wsElem, step.State().CompletedWorkstreams()))
	}
	workstreamInvID := fmt.Sprintf("investigation-workstream-%d", domainIntID)
	if iteration > 0 || revisionPrompt != "" {
		workstreamInvID = fmt.Sprintf("investigation-workstream-%d-%d", domainIntID, iteration)
	}
	findings := RunJSONAgent(
		o.Agents.InvestigatorExecutor,
		prompt,
		workstreamInvID,
		subdir, context,
	)
	cw := step.State().CompletedWorkstreams()
	cw = setCompletedWorkstream(cw, workstreamID, findings)
	modState := state.NewWorkflowStateBuilder(step.State()).WithCompletedWorkstreams(cw).Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationWorkstreamReview, modState).WithWorkstreamID(workstreamID).WithIteration(iteration).WithWorkstreamIndex(workstreamIndex).Build())
}

func (o *Orchestrator) executeInvestigationWorkstreamReview(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	workstreamID := step.WorkstreamID()
	workstreamIndex := step.WorkstreamIndex()
	domainIntID := workstreamIndex + 1
	findingsVal, findingsOk := lookupCompletedWorkstream(step.State().CompletedWorkstreams(), workstreamID)
	if !findingsOk {
		return
	}
	wsElem := step.State().Workstreams().Get(workstreamIndex)

	gapReview := RunJSONAgent(
		o.Agents.GapAnalysisReviewer,
		fmt.Sprintf("WORKSTREAM:\n%s\nFINDINGS TO REVIEW:\n%s", buildInvestigationWorkstreamPrompt(wsElem, step.State().CompletedWorkstreams()), MarshalJSON(findingsVal)),
		fmt.Sprintf("investigation-gap-review-ws-%d-%d", domainIntID, iteration),
		[]string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", domainIntID)}, context,
	)

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationFactReview, step.State()).WithIteration(iteration).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(state.NewPayloadDataBuilder(nil).WithGapAnalysisReview(&gapReview).Build()).Build())
}

func (o *Orchestrator) executeInvestigationFactReview(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	workstreamID := step.WorkstreamID()
	workstreamIndex := step.WorkstreamIndex()
	gapReview := step.Payload().GapAnalysisReview()
	findingsVal, findingsOk := lookupCompletedWorkstream(step.State().CompletedWorkstreams(), workstreamID)
	if !findingsOk {
		return
	}
	wsElem := step.State().Workstreams().Get(workstreamIndex)

	factReview := RunJSONAgent(
		o.Agents.FactCheckingReviewer,
		fmt.Sprintf("WORKSTREAM:\n%s\nFINDINGS TO REVIEW:\n%s", buildInvestigationWorkstreamPrompt(wsElem, step.State().CompletedWorkstreams()), MarshalJSON(findingsVal)),
		fmt.Sprintf("investigation-fact-review-ws-%d-%d", workstreamIndex+1, iteration),
		[]string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", workstreamIndex+1)}, context,
	)

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationFactReviewOrchestrator, step.State()).WithIteration(iteration).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(state.NewPayloadDataBuilder(nil).WithFactReview(&factReview).WithGapAnalysisReview(gapReview).WithInvestigatorFindings(&findingsVal).WithWorkstreamElem(&wsElem).Build()).Build())
}

func (o *Orchestrator) executeInvestigationSynthesis(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()

	findingsList := []interface{}{}
	wsItr := step.State().Workstreams().Iterator()
	wsItr.First()
	for !wsItr.Done() {
		_, v := wsItr.Next()
		id := strings.ToLower(strings.TrimSpace(v.Id()))
		if wf, ok := lookupCompletedWorkstream(step.State().CompletedWorkstreams(), id); ok {
			if isEmptyFindings(wf) {
				findingsList = append(findingsList, nil)
			} else {
				findingsList = append(findingsList, wf)
			}
		}
	}

	var revisionPrompt string
	if step.Payload() != nil {
		reportVal := step.Payload().InvestigationReport()
		consistencyReview := step.Payload().ConsistencyReview()
		revisionPrompt = fmt.Sprintf(
			"REVIEW FEEDBACK:\n%s\nFINDINGS:\n%s\nPREVIOUS REPORT:\n%s\n\nRebuild the investigation report from scratch using the findings and review feedback.",
			MarshalJSON(consistencyReview), MarshalJSON(findingsList), MarshalJSON(reportVal),
		)
	}

	var prompt string
	if revisionPrompt != "" {
		prompt = revisionPrompt
	} else {
		prompt = fmt.Sprintf("FINDINGS:\n%s", MarshalJSON(findingsList))
	}

	synthesisInvID := "investigation-synthesis"
	if iteration > 0 || revisionPrompt != "" {
		synthesisInvID = fmt.Sprintf("investigation-synthesis-%d", iteration)
	}
	report := RunJSONAgent(
		o.Agents.SynthesisAgent,
		prompt,
		synthesisInvID,
		[]string{step.State().Subdir(), "investigation"}, context,
	)
	modState := state.NewWorkflowStateBuilder(step.State()).WithInvestigationResults(&report).Build()

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationConsistencyReview, modState).WithIteration(iteration).Build())
}

func (o *Orchestrator) executeInvestigationConsistencyReview(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	reportVal := *step.State().InvestigationResults()
	findingsList := []interface{}{}
	wsItr := step.State().Workstreams().Iterator()
	wsItr.First()
	for !wsItr.Done() {
		_, v := wsItr.Next()
		id := strings.ToLower(strings.TrimSpace(v.Id()))
		if wf, ok := lookupCompletedWorkstream(step.State().CompletedWorkstreams(), id); ok {
			if isEmptyFindings(wf) {
				findingsList = append(findingsList, nil)
			} else {
				findingsList = append(findingsList, wf)
			}
		}
	}

	consistencyReview := RunJSONAgent(
		o.Agents.SynthesisConsistencyReview,
		fmt.Sprintf("REPORT TO REVIEW:\n%s\nSOURCE FINDINGS:\n%s", MarshalJSON(reportVal), MarshalJSON(findingsList)),
		fmt.Sprintf("investigation-consistency_review-final-%d", iteration),
		[]string{step.State().Subdir(), "investigation"}, context,
	)

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationConsistencyReviewOrchestrator, step.State()).WithIteration(iteration).WithPayload(state.NewPayloadDataBuilder(nil).WithInvestigationReport(&reportVal).WithConsistencyReview(&consistencyReview).Build()).Build())
}

func (o *Orchestrator) executeDomainStart(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	if _, exists := getDomain(step.State().Domains(), domainID); exists {
		return
	}

	domainIndex := step.DomainIndex()
	sessionSuffix := fmt.Sprintf("d%d-start", domainIntID)

	decompVal := step.State().DecompositionResult()
	decomp := decompVal.Decomposition()
	domainList := immutableListToSlice(decomp.Domains())
	integrationOwnershipList := immutableListToSlice(decomp.IntegrationOwnership())
	if domainIndex < len(domainList) {
		domainMap := domainList[domainIndex]
		spec, archExtra := BuildArchitectInput(domainMap, integrationOwnershipList)
		if spec == "" {
			ds := state.NewDomainStateBuilder(nil).
				WithFinalReviewPassed(true).
				Build()
			domains := setDomain(step.State().Domains(), domainID, ds)
			modState := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).WithDomainCurrentStage("final_review").Build()
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(0).Build())
			return
		}

		o.Agents.Arch.Reset(context, sessionSuffix)
		o.Agents.TechLead.Reset(context, sessionSuffix)
		o.Agents.Coder.Reset(context, sessionSuffix)

		coderTask := RunJSONAgent(
			o.Agents.DesignCleanup,
			fmt.Sprintf("INPUT TEXT:\n%s", spec),
			fmt.Sprintf("d%d-design-cleanup", domainIntID),
			[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)}, context,
		)

		wrappedTask := wrapText(spec + archExtra)
		wrappedCoderTask := wrapText(coderTask.Text())

		emptyMap := immutable.NewMap[string, string](nil)
		ds := state.NewDomainStateBuilder(nil).
			WithWrappedTask(wrappedTask).
			WithWrappedCoderTask(wrappedCoderTask).
			WithPmFilepath(step.State().PmFilepath()).
			WithAllChanges(state.DomainStateallchanges(*emptyMap)).
			WithCodeSummaries(immutable.NewList[string]()).
			Build()
		domains := step.State().Domains()
		domains = setDomain(domains, domainID, ds)
		modState := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeArchitecture, modState).WithDomainID(domainID).WithDomainIndex(domainIndex).WithIteration(0).Build())
		return
	}

	ds := state.NewDomainStateBuilder(nil).
		WithFinalReviewPassed(true).
		Build()
	domainStates := step.State().Domains()
	domainStates = setDomain(domainStates, domainID, ds)
	modState := state.NewWorkflowStateBuilder(step.State()).WithDomains(domainStates).WithDomainCurrentStage("final_review").Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(0).Build())
}

func (o *Orchestrator) executeArchitecture(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	revIterForNudge := reviewIter
	ds, _ := getDomain(step.State().Domains(), domainID)

	ds = *state.NewDomainStateBuilder(&ds).WithCodeSummaries(immutable.NewList[string]()).WithFinalReviewPassed(false).WithMergedSummaries("").Build()

	decompRes := step.State().DecompositionResult()
	decomp := decompRes.Decomposition()
	numDomains := len(immutableListToSlice(decomp.Domains()))
	logStep(fmt.Sprintf("Starting iteration %d/%d for domain %d/%d",
		iteration+1, MAXTopIterations, step.DomainIndex()+1, numDomains), "ITERATION")

	var initialPrompt string

	if step.Payload() == nil {
		if ds.FinalFeedback() != nil && ds.FinalFeedback() != nil {
			initialPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE ARCHITECTURE based on feedback post implementation:\n%s", ds.PmFilepath(), MarshalJSON(ds.FinalFeedback()))
		} else {
			initialPrompt = fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s", ds.WrappedTask(), ds.PmFilepath())
		}
	}

	extraPrompt := "\nKeep architecture focused on component boundaries, ownership, and interactions. Avoid naming concrete functions, methods, language constructs, or exact code statements unless they are architecturally significant."

	var reviewPrompt string
	if step.Payload() != nil {
		review := step.Payload().ArchReview()
		if shouldReset(review) {
			o.Agents.Arch.Reset(context, fmt.Sprintf("d%d-arch-%d-%d", domainIntID, iteration, reviewIter-1))
			reviewPrompt = fmt.Sprintf(
				"TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nPREVIOUS ARCHITECTURE:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the architecture from scratch using the task and review feedback.",
				ds.WrappedTask(), ds.PmFilepath(), MarshalJSON(ds.Architecture()), MarshalJSON(review),
			)
		} else {
			reviewPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE ARCHITECTURE based on feedback:\n%s", ds.PmFilepath(), MarshalJSON(review))
		}
	}

	archResults := Nudge(
		100,
		o.Agents.Arch,
		initialPrompt+reviewPrompt+extraPrompt,
		fmt.Sprintf("d%d-arch-%d-%d", domainIntID, iteration, revIterForNudge),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup, context,
	)
	arch := archResults[len(archResults)-1].Out
	ds = *state.NewDomainStateBuilder(&ds).WithArchitecture(arch).Build()
	domains := setDomain(step.State().Domains(), domainID, &ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeArchitectureReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(revIterForNudge).Build())
}

func (o *Orchestrator) executeArchitectureReview(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	ds, _ := getDomain(step.State().Domains(), domainID)
	arch := ds.Architecture()

	archReviewResults := Nudge(
		100,
		o.Agents.ArchReview,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nATTEMPT: %d/%d\nARCHITECTURE TO REVIEW:\n%s",
			ds.WrappedTask(), ds.PmFilepath(), reviewIter+1, MAXPlanIters, MarshalJSON(arch)),
		fmt.Sprintf("d%d-arch-%d-review-%d", domainIntID, iteration, reviewIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup, context,
	)
	archReview := archReviewResults[len(archReviewResults)-1].Out

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeArchitectureOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter).WithPayload(state.NewPayloadDataBuilder(nil).WithArchReview(archReview).Build()).Build())
}

func (o *Orchestrator) executeDecompositionOrchestrator(step *state.WorkflowStep, context *Context) {
	iteration := step.ReviewIteration()
	decompositionReview := step.Payload().DecompositionReview()

	if reviewOk[dt.SystemDecompositionReviewissuesElemseverity](decompositionReview) {
		resultMap := step.State().DecompositionResult()
		MarkdownDocumentGenerator(resultMap, "decomposition_final", []string{step.State().Subdir()}, context)

		decomp := resultMap.Decomposition()
		domains := immutableListToSlice(decomp.Domains())
		if len(domains) == 0 {
			fmt.Println("No domains to execute.")
			return
		}

		sortedIndices := sortDomainsTyped(domains)
		firstSortedIdx := sortedIndices[0]
		if firstSortedIdx < len(domains) {
			modState := state.NewWorkflowStateBuilder(step.State()).
				WithDomainIterationIndex(0).
				WithDomainIterationTotal(len(sortedIndices)).
				Build()
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).Build())
		}
		return
	}

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeDecomposition, step.State()).WithPayload(state.NewPayloadDataBuilder(nil).WithDecompositionReview(decompositionReview).Build()).WithIteration(iteration + 1).Build())
}

func (o *Orchestrator) executeArchitectureOrchestrator(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	archReview := step.Payload().ArchReview()

	if reviewOk[dt.ArchReviewissuesElemseverity](archReview) {
		ds, _ := getDomain(step.State().Domains(), domainID)
		arch := ds.Architecture().Clone().WithArchitecture(ds.Architecture().Architecture().Clone().WithReviewerNotes(nil).Build()).Build()
		o.Agents.Arch.LastCorrectResponse = &arch // XXX
		domains := setDomain(step.State().Domains(), domainID, state.NewDomainStateBuilder(&ds).WithArchitecture(arch).Build())
		domainIntID := step.DomainIndex() + 1
		docSuffix := ""
		if iteration > 0 {
			docSuffix = fmt.Sprintf("%d", iteration+1)
		}
		MarkdownDocumentGenerator(arch, fmt.Sprintf("architecture_after_reviews%s", docSuffix), []string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)}, context)

		modState := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).WithDomainCurrentStage("architecture").Build()
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Build())
		return
	}

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeArchitecture, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter + 1).WithPayload(state.NewPayloadDataBuilder(nil).WithArchReview(archReview).Build()).Build())
}

func (o *Orchestrator) executePlanOrchestrator(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	planReview := step.Payload().PlanReview()

	if reviewOk[dt.PlanReviewissuesElemseverity](planReview) {
		ds, _ := getDomain(step.State().Domains(), domainID)
		plan := ds.Plan().Clone().WithPlan(ds.Plan().Plan().Clone().WithReviewerNotes(nil).Build()).Build()
		o.Agents.TechLead.LastCorrectResponse = &plan // XXX
		domainIntID := step.DomainIndex() + 1
		docSuffix := ""
		if iteration > 0 {
			docSuffix = fmt.Sprintf("%d", iteration+1)
		}
		MarkdownDocumentGenerator(plan, fmt.Sprintf("tech_plan_after_reviews%s", docSuffix), []string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)}, context)

		ds, _ = getDomain(step.State().Domains(), domainID)
		emptyMap := immutable.NewMap[string, string](nil)
		ds = *state.NewDomainStateBuilder(&ds).WithAllChanges(state.DomainStateallchanges(*emptyMap)).WithCodeOutputs(immutable.NewList[dt.Coder]()).WithTechLeadReviewResult(nil).WithTechLeadRevisionCycle(false).WithPlan(plan).Build()
		domains := setDomain(step.State().Domains(), domainID, &ds)
		newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
		modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("plan").Build()
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Build())
		return
	}

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePlan, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(state.NewPayloadDataBuilder(nil).WithPlanReview(planReview).Build()).WithReviewIteration(reviewIter).Build())
}

func (o *Orchestrator) executeCodeReviewOrchestrator(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	codeReviewIter := step.CodeReviewIteration()
	review := step.Payload().CodeReview()

	if reviewOk[dt.CodeReviewissuesElemseverity](review) {
		ds, _ := getDomain(step.State().Domains(), domainID)

		if ds.TechLeadRevisionCycle() {

			tlIter := ds.TechLeadReviewIter()

			ds = *state.NewDomainStateBuilder(&ds).WithTechLeadReviewIter(tlIter + 1).Build()
			domains := setDomain(step.State().Domains(), domainID, &ds)
			newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
			modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("code").Build()
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Build())
			return
		}

		modState := state.NewWorkflowStateBuilder(step.State()).WithDomainCurrentStage("code").Build()
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Build())
		return
	}

	ds, _ := getDomain(step.State().Domains(), domainID)
	crIter := codeReviewIter + 1
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeCodeImplementation, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(crIter).WithPayload(state.NewPayloadDataBuilder(nil).WithCodeReview(review).WithTlIter(ds.TechLeadReviewIter()).Build()).Build())
}

func (o *Orchestrator) executeTechLeadReviewOrchestrator(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	techLeadFinalReview := step.Payload().TechLeadFinalReview()

	if reviewOk[dt.TechLeadFinalissuesElemseverity](techLeadFinalReview) {
		modState := state.NewWorkflowStateBuilder(step.State()).
			WithDomainCurrentStage("tech_lead_review").
			WithTechLeadDocSuffix(step.Payload().DocSuffix()).
			Build()
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Build())
		return
	}

	ds, _ := getDomain(step.State().Domains(), domainID)

	tlIter := ds.TechLeadReviewIter()

	revisionTlIter := tlIter
	ds = *state.NewDomainStateBuilder(&ds).
		WithTechLeadRevisionCycle(true).
		WithTechLeadReviewIter(revisionTlIter).
		WithAllChanges(state.DomainStateallchanges(*func() *immutable.Map[string, string] {
			m := immutable.NewMap[string, string](nil)
			return m
		}())).
		Build()
	domains := setDomain(step.State().Domains(), domainID, &ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeCodeImplementation, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(0).WithPayload(state.NewPayloadDataBuilder(nil).WithTechLeadFinalReview(techLeadFinalReview).WithHasTechLeadFinalReview(true).WithDocSuffix(step.Payload().DocSuffix()).WithTlIter(revisionTlIter).Build()).Build())
}
func (o *Orchestrator) executeArchitectureFinalReviewOrchestrator(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	finalFeedback := step.Payload().ArchFinal()
	ds, _ := getDomain(step.State().Domains(), domainID)

	if reviewOk[dt.ArchFinalissuesElemseverity](finalFeedback) {
		ds = *state.NewDomainStateBuilder(&ds).WithFinalFeedback(finalFeedback).WithFinalReviewPassed(true).Build()
		domains := setDomain(step.State().Domains(), domainID, &ds)
		newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
		modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("final_review").Build()
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Build())
		return
	}

	ds = *state.NewDomainStateBuilder(&ds).WithFinalFeedback(finalFeedback).Build()
	domains := setDomain(step.State().Domains(), domainID, &ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeArchitecture, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration + 1).Build())
}

func (o *Orchestrator) executeInvestigationFactReviewOrchestrator(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	workstreamID := step.WorkstreamID()
	workstreamIndex := step.WorkstreamIndex()
	gapReview := step.Payload().GapAnalysisReview()
	factReview := step.Payload().FactReview()
	findingsVal := step.Payload().InvestigatorFindings()
	wsElem := step.Payload().WorkstreamElem()

	if reviewOk[dt.GapAnalysisReviewissuesElemseverity](gapReview) && reviewOk[dt.FactCheckingReviewissuesElemseverity](factReview) {
		MarkdownDocumentGenerator(*findingsVal, fmt.Sprintf("investigation_workstream_%d", workstreamIndex+1), []string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", workstreamIndex+1)}, context)
		cw := step.State().CompletedWorkstreams()
		cw = setCompletedWorkstream(cw, workstreamID, *findingsVal)
		modState := state.NewWorkflowStateBuilder(step.State()).WithCompletedWorkstreams(cw).Build()
		allComplete := true
		wsItr := modState.Workstreams().Iterator()
		wsItr.First()
		for !wsItr.Done() {
			_, ws := wsItr.Next()
			id := strings.ToLower(strings.TrimSpace(ws.Id()))
			if _, completed := lookupCompletedWorkstream(modState.CompletedWorkstreams(), id); !completed {
				allComplete = false
				break
			}
		}
		if allComplete {
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationSynthesis, modState).Build())
		} else {
			nextIdx := workstreamIndex + 1
			workstreams := modState.Workstreams()
			if nextIdx < workstreams.Len() {
				nextWs := workstreams.Get(nextIdx)
				nextID := strings.ToLower(strings.TrimSpace(nextWs.Id()))
				o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationWorkstream, modState).WithWorkstreamID(nextID).WithWorkstreamIndex(nextIdx).Build())
			}
		}
		return
	}

	combinedReview := CombinedReviewPayload{
		GapReview:  *gapReview,
		FactReview: *factReview,
	}
	var revisionPrompt string
	if shouldReset(gapReview) || shouldReset(factReview) {
		o.Agents.InvestigatorExecutor.Reset(context, fmt.Sprintf("investigation-workstream-%d-%d", workstreamIndex+1, iteration))
		revisionPrompt = fmt.Sprintf(
			"WORKSTREAM:\n%s\nPREVIOUS FINDINGS:\n%s\nREVIEW FEEDBACK:\n%s\n\nRevise the investigation findings for this workstream.",
			buildInvestigationWorkstreamPrompt(*wsElem, step.State().CompletedWorkstreams()), MarshalJSON(findingsVal), MarshalJSON(combinedReview),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE INVESTIGATION FINDINGS based on feedback:\n%s", MarshalJSON(combinedReview))
	}

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationWorkstream, step.State()).WithIteration(iteration + 1).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(state.NewPayloadDataBuilder(nil).WithRevisionPrompt(revisionPrompt).Build()).Build())
}

func (o *Orchestrator) executeInvestigationConsistencyReviewOrchestrator(step *state.WorkflowStep, context *Context) {
	iteration := step.Iteration()
	reportVal := step.Payload().InvestigationReport()
	consistencyReview := step.Payload().ConsistencyReview()

	if reviewOk[dt.SynthesisConsistencyReviewissuesElemseverity](consistencyReview) {
		MarkdownDocumentGenerator(reportVal, "investigation_report_final", []string{step.State().Subdir(), "investigation"}, context)
		return
	}

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationSynthesis, step.State()).WithIteration(iteration + 1).WithPayload(state.NewPayloadDataBuilder(nil).WithInvestigationReport(reportVal).WithConsistencyReview(consistencyReview).Build()).Build())
}

func (o *Orchestrator) executePlanReview(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	ds, _ := getDomain(step.State().Domains(), domainID)
	arch := ds.Architecture()
	plan := ds.Plan()

	planReviewResults := Nudge(
		100,
		o.Agents.PlanReview,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nATTEMPT: %d/%d\nAPPROVED ARCHITECTURE:\n%s\nPLAN TO REVIEW:\n%s",
			ds.WrappedCoderTask(), ds.PmFilepath(), reviewIter+1, MAXPlanIters, MarshalJSON(arch), MarshalJSON(plan)),
		fmt.Sprintf("d%d-plan-%d-review-%d", domainIntID, iteration, reviewIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup, context,
	)
	planReview := planReviewResults[len(planReviewResults)-1].Out

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePlanOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter).WithPayload(state.NewPayloadDataBuilder(nil).WithPlanReview(planReview).Build()).Build())
}

func (o *Orchestrator) executePlan(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := getDomain(step.State().Domains(), domainID)
	arch := ds.Architecture()

	planExtraPrompt := "\nPrefer concrete file-level changes, but avoid embedding exact code snippets unless the task is trivial and the code itself is the clearest representation of the change."

	var initialPrompt string
	planReviewIteration := 0
	if step.Payload() != nil {

		review := step.Payload().PlanReview()
		if shouldReset(review) {
			o.Agents.TechLead.Reset(context, fmt.Sprintf("d%d-plan-%d-%d", domainIntID, iteration, step.ReviewIteration()))
			initialPrompt = fmt.Sprintf(
				"TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s\nPREVIOUS PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the plan from scratch using the task, architecture, and review feedback.",
				ds.WrappedCoderTask(), ds.PmFilepath(), MarshalJSON(arch), MarshalJSON(ds.Plan()), MarshalJSON(review),
			)
		} else {
			initialPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE PLAN based on feedback:\n%s", ds.PmFilepath(), MarshalJSON(review))
		}
		planReviewIteration = step.ReviewIteration() + 1
		initialPrompt += planExtraPrompt
	} else {

		initialPrompt = fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s%s",
			ds.WrappedCoderTask(), ds.PmFilepath(), MarshalJSON(arch), planExtraPrompt)
	}

	planResults := Nudge(
		100,
		o.Agents.TechLead,
		initialPrompt,
		fmt.Sprintf("d%d-plan-%d-%d", domainIntID, iteration, planReviewIteration),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup, context,
	)
	plan := planResults[len(planResults)-1].Out
	ds = *state.NewDomainStateBuilder(&ds).WithPlan(plan).Build()
	domains := setDomain(step.State().Domains(), domainID, &ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePlanReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(planReviewIteration).Build())
}

func (o *Orchestrator) executeCodeImplementation(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := getDomain(step.State().Domains(), domainID)
	if iteration == 0 && step.CodeReviewIteration() == 0 && !ds.TechLeadRevisionCycle() {
		ds = *state.NewDomainStateBuilder(&ds).WithTechLeadRevisionCycle(false).Build()
	}
	plan := ds.Plan()

	coderOutputs := make([]dt.Coder, 0)
	allChanges := make(map[string]string)

	readFromDomainState := true
	if ds.TechLeadRevisionCycle() && step.CodeReviewIteration() == 0 {
		readFromDomainState = false
	}
	if readFromDomainState {
		if cos := ds.CodeOutputs(); cos != nil {
			coderOutputs = make([]dt.Coder, 0, cos.Len())
			coItr := cos.Iterator()
			coItr.First()
			for !coItr.Done() {
				_, v := coItr.Next()
				coderOutputs = append(coderOutputs, v)
			}
		}
		if ac := ds.AllChanges(); (*immutable.Map[string, string])(&ac).Len() > 0 {
			allChanges = immutableStringMapToRegular(ac)
		}
	}

	var invocationIDPrefix string
	var tlIter int
	if ds.TechLeadRevisionCycle() {

		if step.Payload() != nil {
			tlIter = step.Payload().TlIter()
		}
		invocationIDPrefix = fmt.Sprintf("d%d-impl-revision-%d-%d", domainIntID, iteration, tlIter)
	} else {
		invocationIDPrefix = fmt.Sprintf("d%d-impl-%d", domainIntID, iteration)
	}

	coderInvSuffix := fmt.Sprintf("-coder-%d", step.CodeReviewIteration())
	var review *dt.CodeReview
	var coderPrompt string
	if step.Payload() != nil {
		review = step.Payload().CodeReview()
		hasTLReview := false
		var tlReview *dt.TechLeadFinal
		if step.Payload().HasTechLeadFinalReview() {
			hasTLReview = true
			tlReview = step.Payload().TechLeadFinalReview()
		}

		var resetNeeded bool
		if hasTLReview {

			if tlReview.ShouldReset() {
				resetNeeded = true
			}
		} else {

			if review != nil && review.ShouldReset() {
				resetNeeded = true
			}
		}

		if resetNeeded {

			if ds.TechLeadRevisionCycle() {
				crIter := step.CodeReviewIteration()
				if crIter == 0 {
					o.Agents.Coder.Reset(context, fmt.Sprintf("%s-start", invocationIDPrefix))
				} else {
					o.Agents.Coder.Reset(context, fmt.Sprintf("%s-%d", invocationIDPrefix, crIter-1))
				}
			} else {
				o.Agents.Coder.Reset(context, fmt.Sprintf("%s-%d", invocationIDPrefix, step.CodeReviewIteration()-1))
			}
			if hasTLReview && step.CodeReviewIteration() == 0 {

				coderPrompt = fmt.Sprintf(
					"APPROVED IMPLEMENTATION PLAN:\n%s\nTECH LEAD FEEDBACK:\n%s\n\nRe-implement from a clean context using the plan and tech lead feedback.",
					MarshalJSON(plan), MarshalJSON(tlReview),
				)
			} else {

				coderPrompt = fmt.Sprintf(
					"PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRe-implement from a clean context using the plan and review feedback. Do not assume prior implementation decisions are correct unless still justified.",
					MarshalJSON(plan), MarshalJSON(review),
				)
			}
		} else {
			if hasTLReview {

				coderPrompt = fmt.Sprintf("TECH LEAD FEEDBACK TO ADDRESS:\n%s", MarshalJSON(tlReview))
			} else {

				coderPrompt = fmt.Sprintf("FEEDBACK TO ADDRESS:\n%s", MarshalJSON(review))
			}
		}
	} else {

		coderPrompt = fmt.Sprintf("APPROVED IMPLEMENTATION PLAN:\n%s\n\nImplement the approved plan exactly as written.\nThe plan has already been reviewed and approved.\nDo not question whether planned file creation or modification should occur.", MarshalJSON(plan))
	}
	safeFlush(o.Watcher, context)
	coderResults := Nudge(
		MAXCodeIters,
		o.Agents.Coder,
		coderPrompt,
		fmt.Sprintf("%s%s", invocationIDPrefix, coderInvSuffix),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		nil, context,
	)
	if len(coderResults) > 0 {
		lastCache := coderResults[len(coderResults)-1]
		lastCoder := lastCache.Out
		coderOutputs = append(coderOutputs, *lastCoder)
		if !lastCache.FromCache && context.watchmanHook == nil {
			time.Sleep(10 * time.Second)
		}
	}
	changes := safeFlush(o.Watcher, context)
	for k, v := range changes {
		allChanges[k] = v
	}
	ds = *state.NewDomainStateBuilder(&ds).WithCodeOutputs(listToImmutableList[dt.Coder](coderOutputs)).WithAllChanges(state.DomainStateallchanges(*func() *immutable.Map[string, string] {
		m := immutable.NewMap[string, string](nil)
		for k, v := range allChanges {
			m = m.Set(k, v)
		}
		return m
	}())).Build()
	domains := setDomain(step.State().Domains(), domainID, &ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	codeReviewPushIter := step.CodeReviewIteration()
	if codeReviewPushIter >= MAXCodeIters {
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Build())
	} else if review != nil {
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeCodeReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(codeReviewPushIter).WithPayload(state.NewPayloadDataBuilder(nil).WithCodeReview(review).WithTlIter(ds.TechLeadReviewIter()).Build()).Build())
	} else {
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeCodeReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(codeReviewPushIter).Build())
	}
}
func (o *Orchestrator) executeCodeReview(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	codeReviewIter := step.CodeReviewIteration()
	ds, _ := getDomain(step.State().Domains(), domainID)
	plan := ds.Plan()

	coderOutputs := ds.CodeOutputs()
	allChanges := ds.AllChanges()

	invocationIDPrefix := fmt.Sprintf("d%d-impl-%d", domainIntID, iteration)
	if ds.TechLeadRevisionCycle() {
		tlIter := ds.TechLeadReviewIter()
		invocationIDPrefix = fmt.Sprintf("d%d-impl-revision-%d-%d", domainIntID, iteration, tlIter)
	}

	var recentChanges []dt.CoderchangesElem
	var recentCoderOutput dt.Coder
	var allChangesList [][]dt.CoderchangesElem
	coItr := coderOutputs.Iterator()
	coItr.First()
	for !coItr.Done() {
		_, v := coItr.Next()
		changesArr := v.Changes()
		allChangesList = append(allChangesList, immutableListToSlice(changesArr))
		recentChanges = immutableListToSlice(changesArr)
		recentCoderOutput = v
	}

	var pastChanges [][]dt.CoderchangesElem
	if len(allChangesList) > 1 {
		pastChanges = allChangesList[:len(allChangesList)-1]
	}

	var taskContext string
	if ds.TechLeadRevisionCycle() && ds.TechLeadReviewResult() != nil {

		tlr := ds.TechLeadReviewResult()
		if tlr.ShouldReset() {

			planJSON := MarshalJSON(plan)
			taskContext = fmt.Sprintf("APPROVED IMPLEMENTATION PLAN:\n%s\nTECH LEAD FEEDBACK:\n%s\n\nRe-implement from a clean context using the plan and tech lead feedback.", planJSON, MarshalJSON(tlr))
		} else {

			taskContext = fmt.Sprintf("TECH LEAD FEEDBACK TO ADDRESS:\n%s", MarshalJSON(tlr))
		}
	} else {

		planJSON := MarshalJSON(plan)
		taskContext = fmt.Sprintf("APPROVED IMPLEMENTATION PLAN:\n%s\n\nImplement the approved plan exactly as written.\nThe plan has already been reviewed and approved.\nDo not question whether planned file creation or modification should occur.", planJSON)
	}

	var recentChangesJSON string
	if len(recentChanges) == 0 {
		recentChangesJSON = MarshalJSON(recentCoderOutput)
	} else {
		recentChangesJSON = MarshalJSON(recentChanges)
	}
	reviewPrompt := fmt.Sprintf(
		"TASK:\n%s\nATTEMPT: %d/%d\n\nMOST RECENT CHANGES:\n%s\n%s\nchanges from past iterations for context:\n%s",
		taskContext,
		codeReviewIter+1,
		MAXCodeIters,
		recentChangesJSON,
		changesPrompt(immutableMapToRegular((*immutable.Map[string, string])(&allChanges))),
		MarshalJSON(pastChanges),
	)

	isTechLeadReview := step.Payload() != nil && step.Payload().CoderOutputs() != nil && step.Payload().CoderOutputs().Len() > 0

	var resultsRaw []CacheResult[*dt.CodeReview]
	if isTechLeadReview {

		coderOutputs := step.Payload().CoderOutputs()
		allChanges := step.Payload().AllChanges()
		revisionInvPrefix := step.Payload().RevisionInvPrefix()
		initialPromptContext := step.Payload().InitialPromptContext()

		iterCount := step.Payload().IterCount()

		for iterCount < MAXCodeIters {
			logStep(fmt.Sprintf("Iteration %d/%d", iterCount+1, MAXCodeIters), "CODER EXECUTION")

			var recentChanges []dt.CoderchangesElem
			var allChangesList [][]dt.CoderchangesElem
			coderOutputsSlice := immutableListToSlice(coderOutputs)
			for _, v := range coderOutputsSlice {
				changesArr := v.Changes()
				allChangesList = append(allChangesList, immutableListToSlice(changesArr))
				recentChanges = immutableListToSlice(changesArr)
			}

			var pastChanges [][]dt.CoderchangesElem
			if len(allChangesList) > 1 {
				pastChanges = allChangesList[:len(allChangesList)-1]
			}
			var recentChangesJSON string
			if len(recentChanges) == 0 {
				recentChangesJSON = MarshalJSON(coderOutputsSlice[len(coderOutputsSlice)-1])
			} else {
				recentChangesJSON = MarshalJSON(recentChanges)
			}
			reviewPrompt := fmt.Sprintf(
				"TASK:\n%s\nATTEMPT: %d/%d\n\nMOST RECENT CHANGES:\n%s\n%s\nchanges from past iterations for context:\n%s",
				initialPromptContext,
				iterCount+1,
				MAXCodeIters,
				recentChangesJSON,
				changesPrompt(immutableMapToRegular((*immutable.Map[string, string])(&allChanges))),
				MarshalJSON(pastChanges),
			)

			resultsRaw = Nudge(
				MAXCodeIters,
				o.Agents.CodeReview,
				reviewPrompt,
				fmt.Sprintf("%s-code-review-%d", revisionInvPrefix, iterCount),
				[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
				o.Agents.NextStepsCleanup, context,
			)
			review := resultsRaw[len(resultsRaw)-1].Out

			if reviewOk[dt.CodeReviewissuesElemseverity](review) {
				break
			}

			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeCodeReview, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(state.NewPayloadDataBuilder(nil).WithCoderOutputs(coderOutputs).WithAllChanges(allChanges).WithRevisionInvPrefix(revisionInvPrefix).WithInitialPromptContext(initialPromptContext).WithIterCount(iterCount).WithTechLeadFinalReview(step.Payload().TechLeadFinalReview()).WithDocSuffix(step.Payload().DocSuffix()).Build()).Build())
			return
		}
	} else {
		resultsRaw = Nudge(
			MAXCodeIters,
			o.Agents.CodeReview,
			reviewPrompt,
			fmt.Sprintf("%s-code-review-%d", invocationIDPrefix, codeReviewIter),
			[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
			o.Agents.NextStepsCleanup, context,
		)
	}
	review := resultsRaw[len(resultsRaw)-1].Out

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeCodeReviewOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(codeReviewIter).WithPayload(state.NewPayloadDataBuilder(nil).WithCodeReview(review).WithTlIter(ds.TechLeadReviewIter()).Build()).Build())
}

func (o *Orchestrator) executeTechLeadReview(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := getDomain(step.State().Domains(), domainID)
	plan := ds.Plan()
	arch := ds.Architecture()
	wrappedCoderTask := ds.WrappedCoderTask()
	docSuffix := ""
	if iteration > 0 {
		docSuffix = fmt.Sprintf("%d", iteration+1)
	}

	var revisions []string
	totalOutputs := ds.CodeOutputs().Len()
	codeItr := ds.CodeOutputs().Iterator()
	codeItr.First()
	for i := 0; i < totalOutputs; i++ {
		_, v := codeItr.Next()
		summary := v.Summary()

		revisions = append(revisions, fmt.Sprintf("<revision%d>\n%s\n</revision%d>", i+1, summary, i+1))
	}
	codeSummary := fmt.Sprintf("%s\n%s", strings.Join(revisions, "\n"), changesPrompt(immutableStringMapToRegular(ds.AllChanges())))
	ds = *state.NewDomainStateBuilder(&ds).WithCodeSummary(codeSummary).Build()

	if step.Payload() != nil {
		cs := ds.CodeSummaries().Append(codeSummary)
		ds = *state.NewDomainStateBuilder(&ds).WithCodeSummaries(cs).Build()
	}
	domains := setDomain(step.State().Domains(), domainID, &ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	tlIter := 0
	if step.Payload() != nil {
		tlIter = step.Payload().TlIter()
	}

	var extraPromptTL string
	if tlIter > 0 && step.Payload() != nil {
		tlfr := step.Payload().TechLeadFinalReview()
		if tlfr != nil {
			extraPromptTL = fmt.Sprintf("PREVIOUS FEEDBACK:\n%s\n", MarshalJSON(tlfr))
		}
	}

	techLeadFinalResults := Nudge(
		100,
		o.Agents.TechLeadFinal,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s\nAPPROVED IMPLEMENTATION PLAN:\n%s\n%s<aggregate_implementation_summary>\n%s\n</aggregate_implementation_summary>\n",
			wrappedCoderTask, ds.PmFilepath(), MarshalJSON(arch), MarshalJSON(plan), extraPromptTL, codeSummary),
		fmt.Sprintf("d%d-tl-review-%d-%d", domainIntID, iteration, tlIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup, context,
	)
	techLeadFinalReview := techLeadFinalResults[len(techLeadFinalResults)-1].Out

	ds = *state.NewDomainStateBuilder(&ds).WithTechLeadReviewIter(tlIter).WithTechLeadReviewResult(techLeadFinalReview).Build()
	domains = setDomain(step.State().Domains(), domainID, &ds)
	newWs = state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeTechLeadReviewOrchestrator, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(state.NewPayloadDataBuilder(nil).WithTechLeadFinalReview(techLeadFinalReview).WithHasTechLeadFinalReview(true).WithDocSuffix(docSuffix).WithTlIter(tlIter + 1).Build()).Build())
}

func (o *Orchestrator) executeTechLeadEnd(step *state.WorkflowStep, context *Context) {
	docSuffix := step.Payload().DocSuffix()
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := getDomain(step.State().Domains(), domainID)

	mergedSummaries := ""
	itr := ds.CodeSummaries().Iterator()
	itr.First()
	for i := 0; i < ds.CodeSummaries().Len(); i++ {
		_, cs := itr.Next()
		mergedSummaries += fmt.Sprintf("<summary%d>\n%s\n</summary%d>\n", i+1, cs, i+1)
	}
	if mergedSummaries != "" {
		mergedSummaries = strings.TrimSuffix(mergedSummaries, "\n")
	} else {
		mergedSummaries = fmt.Sprintf("<summary1>\n%s\n</summary1>", ds.CodeSummary())
	}
	ds = *state.NewDomainStateBuilder(&ds).WithMergedSummaries(mergedSummaries).WithTechLeadRevisionCycle(false).WithTechLeadReviewIter(0).Build()
	domains := setDomain(step.State().Domains(), domainID, &ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	MarkdownDocumentGenerator(mergedSummaries, fmt.Sprintf("code_summary%s", docSuffix), []string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)}, context)

	modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("tech_lead_end").Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeBoundary, modState).
		WithDomainID(domainID).
		WithDomainIndex(step.DomainIndex()).
		WithIteration(iteration).
		Build())
}

func (o *Orchestrator) executeBoundary(step *state.WorkflowStep, context *Context) {
	ws := step.State()

	if ws.DomainIterationTotal() == 0 && ws.DecompositionResult() == nil && ws.Out() != "" {
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeClassification, ws).Build())
		return
	}

	decompRes := ws.DecompositionResult()
	decomp := decompRes.Decomposition()
	numDomains := len(immutableListToSlice(decomp.Domains()))
	if ws.DecompositionResult() != nil && numDomains > 0 && ws.DomainIterationTotal() > 0 && ws.DomainCurrentStage() == "" {
		total := ws.DomainIterationTotal()

		decompVal := ws.DecompositionResult()
		decomp := decompVal.Decomposition()
		domains := immutableListToSlice(decomp.Domains())
		if len(domains) == 0 {
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeFinalInvestigation, ws).Build())
			return
		}
		sortedIndices := sortDomainsTyped(domains)
		firstSortedIdx := sortedIndices[0]
		if firstSortedIdx >= len(domains) {
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeFinalInvestigation, ws).Build())
			return
		}
		firstDomainID := strings.ToLower(strings.TrimSpace(domains[firstSortedIdx].Id()))
		modState := state.NewWorkflowStateBuilder(ws).
			WithDomainIterationIndex(0).
			WithDomainIterationTotal(total).
			WithDomainCurrentStage("domain_start").
			Build()
		o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeDomainStart, modState).
			WithDomainID(firstDomainID).
			WithDomainIndex(firstSortedIdx).
			Build())
		return
	}

	if ws.DomainIterationTotal() > 0 {
		currentIdx := ws.DomainIterationIndex()
		total := ws.DomainIterationTotal()
		currentStage := ws.DomainCurrentStage()

		switch currentStage {
		case "architecture":

			archIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("plan").Build()
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypePlan, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(archIter).
				Build())

		case "plan":

			planIter := step.Iteration()
			domainID := step.DomainID()
			ds, _ := getDomain(ws.Domains(), domainID)
			ds = *state.NewDomainStateBuilder(&ds).WithTechLeadRevisionCycle(false).Build()
			domains := setDomain(ws.Domains(), domainID, &ds)
			newWs := state.NewWorkflowStateBuilder(ws).WithDomains(domains).WithDomainCurrentStage("code").Build()
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeCodeImplementation, newWs).
				WithDomainID(domainID).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(planIter).
				WithCodeReviewIteration(0).
				Build())
		case "code":

			ds, dsExists := getDomain(ws.Domains(), step.DomainID())
			tlIter := 0
			var techLeadFinalReview *dt.TechLeadFinal
			if dsExists {
				tlIter = ds.TechLeadReviewIter()
				if tr := ds.TechLeadReviewResult(); tr != nil {
					techLeadFinalReview = tr
				}
			}
			codeIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("tech_lead_review").Build()
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeTechLeadReview, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(codeIter).
				WithPayload(state.NewPayloadDataBuilder(nil).WithTlIter(tlIter).WithTechLeadFinalReview(techLeadFinalReview).Build()).
				Build())
		case "tech_lead_review":

			docSuffix := ws.TechLeadDocSuffix()
			tlReviewIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("tech_lead_end").Build()
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeTechLeadEnd, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(tlReviewIter).
				WithPayload(state.NewPayloadDataBuilder(nil).WithDocSuffix(docSuffix).Build()).
				Build())

		case "tech_lead_end":

			tlEndIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("final_review").Build()
			o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeArchitectureFinalReview, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(tlEndIter).
				Build())

		case "final_review":

			passedCount := 0
			domItr := ws.Domains().Iterator()
			domItr.First()
			for !domItr.Done() {
				_, d, _ := domItr.Next()
				if d.FinalReviewPassed() {
					passedCount++
				}
			}
			if passedCount == total {
				o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeFinalInvestigation, ws).Build())
				return
			}

			if currentIdx < total-1 {
				decompVal := ws.DecompositionResult()
				decomp := decompVal.Decomposition()
				domains := immutableListToSlice(decomp.Domains())
				sortedIndices := sortDomainsTyped(domains)
				nextPosition := currentIdx + 1
				nextSortedIdx := sortedIndices[nextPosition]
				if nextSortedIdx < len(domains) {
					nextDomainID := strings.ToLower(strings.TrimSpace(domains[nextSortedIdx].Id()))
					modState := state.NewWorkflowStateBuilder(ws).
						WithDomainIterationIndex(nextPosition).
						WithDomainCurrentStage("domain_start").
						Build()
					o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeDomainStart, modState).
						WithDomainID(nextDomainID).
						WithDomainIndex(nextSortedIdx).
						Build())
				}
			}
			return
		default:
			panic(fmt.Sprintf("Unexpected stage: %s", currentStage))
		}
		return
	}

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeFinalInvestigation, ws).Build())
}

func (o *Orchestrator) executeArchitectureFinalReview(step *state.WorkflowStep, context *Context) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := getDomain(step.State().Domains(), domainID)
	arch := ds.Architecture()
	plan := ds.Plan()
	wrappedTask := ds.WrappedTask()

	finalFeedback := RunJSONAgent(
		o.Agents.ArchFinal,
		fmt.Sprintf("TASK:\n%s\nARCHITECTURE:\n%s\nAPPROVED IMPLEMENTATION PLAN:\n%s\n<aggregate_implementation_summary>\n%s\n</aggregate_implementation_summary>\n",
			wrappedTask,
			MarshalJSON(arch),
			MarshalJSON(plan),
			ds.MergedSummaries(),
		),
		fmt.Sprintf("d%d-arch-final-review-%d", domainIntID, iteration),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)}, context,
	)

	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeArchitectureFinalReviewOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(state.NewPayloadDataBuilder(nil).WithArchFinal(&finalFeedback).Build()).Build())
}

func (o *Orchestrator) executeFinalInvestigation(step *state.WorkflowStep, context *Context) {
	subdir := []string{step.State().Subdir(), "investigation"}
	_ = subdir
	modState := state.NewWorkflowStateBuilder(step.State()).WithFinalInvestigationTask(wrapText(`INVESTIGATION OBJECTIVE:
Determine whether the resulting system state faithfully realizes the intent of the original request.

The task is not to verify the presence of artifacts.
The task is to identify semantic mismatches between requested intent and resulting system behavior/structure.

ORIGINAL REQUEST:
` + step.State().Task() + `

FOCUS AREAS:
* hidden assumptions
* ambiguity resolution choices
* workflow coherence
* semantic completeness
* edge cases implied by the request
* overengineering
* underengineering
* brittle architecture
* superficial requirement satisfaction
* optimization toward incorrect interpretations

IMPORTANT:
* Do not infer correctness from implementation sophistication.
* Do not assume implemented behavior reflects intended behavior.
* Treat all implementation decisions as hypotheses requiring justification.`)).Build()
	o.Stack.Push(o.Stack.NewStep(state.WorkflowStepsteptypeInvestigationPlanGenerate, modState).Build())
}

func safeFlush(watcher *wman.Watchman, context *Context) map[string]string {
	defer func() {
		recover()
	}()
	var out map[string]string
	if context.watchmanHook != nil {
		out = context.watchmanHook()
	} else {
		out = watcher.Flush()
	}
	context.Tracer.trace("watchman", td.NewActionDetailsBuilder(nil).WithChanges(td.ActionDetailschanges(*immutable.NewMapOf[string](nil, out))).Build())
	return out
}

func changesPrompt(changes map[string]string) string {
	if len(changes) == 0 {
		return "**AUTOMATED VERIFICATION FAILED: NO ACTUAL CHANGES DETECTED**\n"
	}
	out := "AUTOMATED VERIFICATION DETECTED POTENTIAL CHANGES TO FOLLOWING FILES, COMPLETENESS MUST BE ASSESSED:\n"
	names := make([]string, 0, len(changes))
	for name := range changes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out += fmt.Sprintf("* %s: %s\n", changes[name], name)
	}
	return out
}

func updateSpeculativeExpansions(task *dt.PmSynthesizer, expansions []string) *dt.PmSynthesizer {
	return task.Clone().WithSpeculativeExpansions(listToImmutableList(expansions)).Build()
}

func isEmptyFindings(f dt.InvestigatorFindings) bool {
	if f.Conclusions() == nil || f.SupportingEvidence() == nil || f.UnansweredQuestions() == nil {
		return true
	}
	return f.Conclusions().Len() == 0 &&
		f.ConfidenceLevel() == "" &&
		f.SupportingEvidence().Len() == 0 &&
		f.UnansweredQuestions().Len() == 0 &&
		f.WorkstreamObjective() == ""
}
