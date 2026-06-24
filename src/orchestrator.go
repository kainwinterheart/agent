package main

import (
	dt "agent-go/gen"
	"agent-go/pkg/loader"
	jsonv2 "encoding/json/v2"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"agent-go/state"
	"agent-go/wman"
	"github.com/benbjohnson/immutable"
)

var watchmanHook func() map[string]string

type CombinedReviewPayload struct {
	GapReview  dt.GapAnalysisReviewJson  `json:"gap_review"`
	FactReview dt.FactCheckingReviewJson `json:"fact_review"`
}

type StructuralReviewPayload struct {
	QualityReview dt.InvestigationPlanQualityReviewJson `json:"quality_review"`
	StructReview  dt.StructuralReviewJson               `json:"struct_review"`
}
type Orchestrator struct {
	Agents  *AgentRegistry
	Watcher *wman.Watchman
	Stack   state.WorkflowStack
}

type AgentRegistry struct {
	ProductManager             *Agent[dt.ProductManagerJson]
	PMSynth                    *Agent[dt.PmSynthesizerJson]
	PMExpansionCleanup         *Agent[dt.PmExpansionCleanupJson]
	NextStepsCleanup           *Agent[dt.NonCoderNextStepsCleanupJson]
	PMReview                   *Agent[dt.PmReviewJson]
	DesignCleanup              *Agent[dt.DesignToImplementPhrasingJson]
	Arch                       *Agent[dt.ArchJson]
	TechLead                   *Agent[dt.PlanJson]
	Coder                      *Agent[dt.CoderJson]
	ArchReview                 *Agent[dt.ArchReviewJson]
	PlanReview                 *Agent[dt.PlanReviewJson]
	CodeReview                 *Agent[dt.CodeReviewJson]
	TechLeadFinal              *Agent[dt.TechLeadFinalJson]
	ArchFinal                  *Agent[dt.ArchFinalJson]
	Decomposition              *Agent[dt.SystemDecompositionJson]
	DecompositionReview        *Agent[dt.SystemDecompositionReviewJson]
	InvestigationClassifier    *Agent[dt.InvestigationClassifierJson]
	InvestigatorPlanner        *Agent[dt.InvestigatorPlanJson]
	InvestigatorExecutor       *Agent[dt.InvestigatorFindingsJson]
	SynthesisAgent             *Agent[dt.InvestigationReportJson]
	GapAnalysisReviewer        *Agent[dt.GapAnalysisReviewJson]
	FactCheckingReviewer       *Agent[dt.FactCheckingReviewJson]
	StructuralReviewer         *Agent[dt.StructuralReviewJson]
	InvestigationPlanQuality   *Agent[dt.InvestigationPlanQualityReviewJson]
	SynthesisConsistencyReview *Agent[dt.SynthesisConsistencyReviewJson]
}

func NewOrchestrator(task string, subdir string) *Orchestrator {
	trace("user_input", map[string]interface{}{"text": task})
	o := &Orchestrator{}
	o.Agents = NewAgentRegistry(subdir)
	o.Watcher = wman.NewWatchman(subdir)
	o.Watcher.Start()
	return o
}

func NewAgentRegistry(subdir string) *AgentRegistry {
	ar := &AgentRegistry{}

	ar.ProductManager = NewAgent[dt.ProductManagerJson](
		"product_manager",
		loader.PRODUCT_MANAGER_PROMPT,
		loader.PRODUCT_MANAGER_SCHEMA,
		subdir,
		WithEphemeral[dt.ProductManagerJson](true),
		WithTimeout[dt.ProductManagerJson]("30m"),
	)
	ar.PMSynth = NewAgent[dt.PmSynthesizerJson](
		"pm_synth",
		loader.PM_SYNTHESIZER_PROMPT,
		loader.PM_SYNTHESIZER_SCHEMA,
		subdir,
		WithTimeout[dt.PmSynthesizerJson]("30m"),
	)
	ar.PMExpansionCleanup = NewAgent[dt.PmExpansionCleanupJson](
		"pm_expansion_cleanup",
		loader.PM_EXPANSION_CLEANUP_PROMPT,
		loader.PM_EXPANSION_CLEANUP_SCHEMA,
		subdir,
		WithEphemeral[dt.PmExpansionCleanupJson](true),
		WithTimeout[dt.PmExpansionCleanupJson]("10m"),
	)
	ar.NextStepsCleanup = NewAgent[dt.NonCoderNextStepsCleanupJson](
		"next_steps_cleanup",
		loader.NON_CODER_NEXT_STEPS_CLEANUP_PROMPT,
		loader.NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA,
		subdir,
		WithEphemeral[dt.NonCoderNextStepsCleanupJson](true),
		WithTimeout[dt.NonCoderNextStepsCleanupJson]("10m"),
	)
	ar.PMReview = NewAgent[dt.PmReviewJson](
		"pm_review",
		loader.PM_REVIEW_PROMPT,
		loader.PM_REVIEW_SCHEMA,
		subdir,
		WithEphemeral[dt.PmReviewJson](true),
		WithTimeout[dt.PmReviewJson]("30m"),
		WithResume[dt.PmReviewJson](loader.ReviewerResume),
	)

	ar.DesignCleanup = NewAgent[dt.DesignToImplementPhrasingJson](
		"design_cleanup",
		loader.DESIGN_TO_IMPLEMENT_PHRASING_PROMPT,
		loader.DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA,
		subdir,
		WithEphemeral[dt.DesignToImplementPhrasingJson](true),
		WithTimeout[dt.DesignToImplementPhrasingJson]("10m"),
	)

	ar.Arch = NewAgent[dt.ArchJson](
		"arch", loader.ARCH_PROMPT, loader.ARCH_SCHEMA, subdir, WithTimeout[dt.ArchJson]("40m"),
	)
	ar.TechLead = NewAgent[dt.PlanJson](
		"tech_lead", loader.PLAN_PROMPT, loader.PLAN_SCHEMA, subdir, WithTimeout[dt.PlanJson]("60m"),
	)
	ar.Coder = NewAgent[dt.CoderJson](
		"coder", loader.CODER_PROMPT, loader.CODER_SCHEMA, subdir, WithTimeout[dt.CoderJson]("180m"),
	)

	ar.ArchReview = NewAgent[dt.ArchReviewJson](
		"arch_review", loader.ARCH_REVIEW_PROMPT, loader.ARCH_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.ArchReviewJson](true), WithTimeout[dt.ArchReviewJson]("30m"),
		WithResume[dt.ArchReviewJson](loader.ReviewerResume),
	)

	ar.PlanReview = NewAgent[dt.PlanReviewJson](
		"plan_review", loader.PLAN_REVIEW_PROMPT, loader.PLAN_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.PlanReviewJson](true), WithTimeout[dt.PlanReviewJson]("30m"),
		WithResume[dt.PlanReviewJson](loader.ReviewerResume),
	)

	ar.CodeReview = NewAgent[dt.CodeReviewJson](
		"code_review", loader.CODE_REVIEW_PROMPT, loader.CODE_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.CodeReviewJson](true), WithTimeout[dt.CodeReviewJson]("60m"),
		WithResume[dt.CodeReviewJson](loader.ReviewerResume),
	)

	ar.TechLeadFinal = NewAgent[dt.TechLeadFinalJson](
		"tech_lead_final", loader.TECH_LEAD_FINAL_PROMPT, loader.TECH_LEAD_FINAL_SCHEMA, subdir,
		WithEphemeral[dt.TechLeadFinalJson](true), WithTimeout[dt.TechLeadFinalJson]("60m"),
		WithResume[dt.TechLeadFinalJson](loader.ReviewerResume),
	)

	ar.ArchFinal = NewAgent[dt.ArchFinalJson](
		"arch_final", loader.ARCH_FINAL_PROMPT, loader.ARCH_FINAL_SCHEMA, subdir,
		WithEphemeral[dt.ArchFinalJson](true), WithTimeout[dt.ArchFinalJson]("60m"),
		WithResume[dt.ArchFinalJson](loader.ReviewerResume),
	)

	ar.Decomposition = NewAgent[dt.SystemDecompositionJson](
		"decomposition", loader.SYSTEM_DECOMPOSITION_PROMPT, loader.SYSTEM_DECOMPOSITION_SCHEMA,
		subdir, WithTimeout[dt.SystemDecompositionJson]("40m"),
	)
	ar.DecompositionReview = NewAgent[dt.SystemDecompositionReviewJson](
		"decomposition_review", loader.SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
		loader.SYSTEM_DECOMPOSITION_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.SystemDecompositionReviewJson](true), WithTimeout[dt.SystemDecompositionReviewJson]("30m"),
		WithResume[dt.SystemDecompositionReviewJson](loader.ReviewerResume),
	)

	ar.InvestigationClassifier = NewAgent[dt.InvestigationClassifierJson](
		"investigation_classifier", loader.INVESTIGATION_CLASSIFIER_PROMPT,
		loader.INVESTIGATION_CLASSIFIER_SCHEMA, subdir,
		WithEphemeral[dt.InvestigationClassifierJson](true), WithTimeout[dt.InvestigationClassifierJson]("10m"),
	)
	ar.InvestigatorPlanner = NewAgent[dt.InvestigatorPlanJson](
		"investigator_planner", loader.INVESTIGATOR_PLANNER_PROMPT,
		loader.INVESTIGATOR_PLAN_SCHEMA, subdir, WithTimeout[dt.InvestigatorPlanJson]("90m"),
	)
	ar.InvestigatorExecutor = NewAgent[dt.InvestigatorFindingsJson](
		"investigator_executor", loader.INVESTIGATOR_EXECUTOR_PROMPT,
		loader.INVESTIGATOR_FINDINGS_SCHEMA, subdir, WithTimeout[dt.InvestigatorFindingsJson]("180m"),
	)
	ar.SynthesisAgent = NewAgent[dt.InvestigationReportJson](
		"synthesis_agent", loader.SYNTHESIS_PROMPT, loader.INVESTIGATION_REPORT_SCHEMA,
		subdir, WithEphemeral[dt.InvestigationReportJson](true), WithTimeout[dt.InvestigationReportJson]("90m"),
	)
	ar.GapAnalysisReviewer = NewAgent[dt.GapAnalysisReviewJson](
		"gap_analysis_reviewer", loader.GAP_ANALYSIS_REVIEW_PROMPT,
		loader.GAP_ANALYSIS_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.GapAnalysisReviewJson](true), WithTimeout[dt.GapAnalysisReviewJson]("60m"),
		WithResume[dt.GapAnalysisReviewJson](loader.ReviewerResume),
	)
	ar.FactCheckingReviewer = NewAgent[dt.FactCheckingReviewJson](
		"fact_checking_reviewer", loader.FACT_CHECKING_REVIEW_PROMPT,
		loader.FACT_CHECKING_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.FactCheckingReviewJson](true), WithTimeout[dt.FactCheckingReviewJson]("60m"),
		WithResume[dt.FactCheckingReviewJson](loader.ReviewerResume),
	)
	ar.StructuralReviewer = NewAgent[dt.StructuralReviewJson](
		"structural_reviewer", loader.STRUCTURE_REVIEW_PROMPT,
		loader.STRUCTURAL_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.StructuralReviewJson](true), WithTimeout[dt.StructuralReviewJson]("30m"),
		WithResume[dt.StructuralReviewJson](loader.ReviewerResume),
	)
	ar.InvestigationPlanQuality = NewAgent[dt.InvestigationPlanQualityReviewJson](
		"investigation_plan_quality_reviewer", loader.INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT,
		loader.INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.InvestigationPlanQualityReviewJson](true), WithTimeout[dt.InvestigationPlanQualityReviewJson]("30m"),
		WithResume[dt.InvestigationPlanQualityReviewJson](loader.ReviewerResume),
	)
	ar.SynthesisConsistencyReview = NewAgent[dt.SynthesisConsistencyReviewJson](
		"synthesis_consistency_reviewer", loader.SYNTHESIS_CONSISTENCY_REVIEW_PROMPT,
		loader.SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA, subdir,
		WithEphemeral[dt.SynthesisConsistencyReviewJson](true), WithTimeout[dt.SynthesisConsistencyReviewJson]("60m"),
		WithResume[dt.SynthesisConsistencyReviewJson](loader.ReviewerResume),
	)

	return ar
}

func wrapText(text string) string {
	return fmt.Sprintf("<text>\n%s\n</text>", text)
}

func sortDomainsTyped(domains []dt.SystemDecompositionJsondecompositiondomainsElem) []int {
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

func buildDependencyGraphTyped(domains []dt.SystemDecompositionJsondecompositiondomainsElem, depsKey string) dependencyGraphTyped {
	g := dependencyGraphTyped{
		idToIdx: make(map[string]int),
		inDeg:   make(map[int]int),
		depKeys: make(map[int]int),
	}
	for i, domain := range domains {
		g.inDeg[i] = 0
		g.depKeys[i] = 0
		if depsKey == "UpstreamDependencies" {
			g.depKeys[i] = len(domain.UpstreamDependencies())
			for _, depStr := range domain.UpstreamDependencies() {
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
		WithDomains(immutable.NewMap[string, *state.DomainState](nil)).
		WithInvestigationResults(nil).
		WithCompletedWorkstreams(immutable.NewMap[string, dt.InvestigatorFindingsJson](nil)).
		Build()

	o.Stack.NewStep(state.StepPMCandidateGenerate, currentState).Push()

	for {
		step, ok := o.Stack.Pop()
		if !ok {
			break
		}
		currentState = step.State()
		o.executeStep(step)
	}

}

func (o *Orchestrator) executeStep(step *state.WorkflowStep) {
	switch step.Type() {
	case state.StepPMCandidateGenerate:
		o.executePMCandidateGenerate(step)
	case state.StepPMSynthesize:
		o.executePMSynthesize(step)
	case state.StepPMReview:
		o.executePMReview(step)
	case state.StepClassification:
		o.executeClassification(step)
	case state.StepDecomposition:
		o.executeDecomposition(step)
	case state.StepDecompositionReview:
		o.executeDecompositionReview(step)
	case state.StepInvestigationPlanGenerate:
		o.executeInvestigationPlanGenerate(step)
	case state.StepInvestigationPlanReview:
		o.executeInvestigationPlanReview(step)
	case state.StepInvestigationWorkstream:
		o.executeInvestigationWorkstream(step)
	case state.StepInvestigationWorkstreamReview:
		o.executeInvestigationWorkstreamReview(step)
	case state.StepInvestigationSynthesis:
		o.executeInvestigationSynthesis(step)
	case state.StepInvestigationConsistencyReview:
		o.executeInvestigationConsistencyReview(step)
	case state.StepDomainStart:
		o.executeDomainStart(step)
	case state.StepArchitecture:
		o.executeArchitecture(step)
	case state.StepArchitectureReview:
		o.executeArchitectureReview(step)
	case state.StepPlan:
		o.executePlan(step)

	case state.StepPlanReview:
		o.executePlanReview(step)
	case state.StepCodeImplementation:
		o.executeCodeImplementation(step)
	case state.StepCodeReview:
		o.executeCodeReview(step)
	case state.StepTechLeadReview:
		o.executeTechLeadReview(step)
	case state.StepArchitectureFinalReview:
		o.executeArchitectureFinalReview(step)
	case state.StepFinalInvestigation:
		o.executeFinalInvestigation(step)
	case state.StepPMExpansionCleanup:
		o.executePMExpansionCleanup(step)
	case state.StepInvestigationPlanStructuralReview:
		o.executeInvestigationPlanStructuralReview(step)
	case state.StepInvestigationFactReview:
		o.executeInvestigationFactReview(step)
	case state.StepDecompositionOrchestrator:
		o.executeDecompositionOrchestrator(step)
	case state.StepArchitectureOrchestrator:
		o.executeArchitectureOrchestrator(step)
	case state.StepPlanOrchestrator:
		o.executePlanOrchestrator(step)
	case state.StepCodeReviewOrchestrator:
		o.executeCodeReviewOrchestrator(step)
	case state.StepTechLeadReviewOrchestrator:
		o.executeTechLeadReviewOrchestrator(step)
	case state.StepArchitectureFinalReviewOrchestrator:
		o.executeArchitectureFinalReviewOrchestrator(step)
	case state.StepInvestigationFactReviewOrchestrator:
		o.executeInvestigationFactReviewOrchestrator(step)
	case state.StepInvestigationConsistencyReviewOrchestrator:
		o.executeInvestigationConsistencyReviewOrchestrator(step)
	case state.StepTechLeadEnd:
		o.executeTechLeadEnd(step)
	case state.StepBoundary:
		o.executeBoundary(step)
	default:
		logStep(fmt.Sprintf("Unknown step type: %v", step.Type()), "SYSTEM")
	}
}

func (o *Orchestrator) executePMCandidateGenerate(step *state.WorkflowStep) {
	candidates := []dt.ProductManagerJson{}
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
		candidate := RunJSONAgent[dt.ProductManagerJson](
			o.Agents.ProductManager,
			fmt.Sprintf("USER REQUEST:\n%s\n\nTASK:\nProduce a focused engineering-ready specification.\n%s", step.State().Task(), bias),
			fmt.Sprintf("pm-spec-candidate-%d", idx),
			[]string{step.State().Subdir()},
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
	o.Stack.NewStep(state.StepPMSynthesize, state.NewWorkflowStateBuilder(step.State()).WithChoices(choices).Build()).WithIteration(0).Push()
}

func (o *Orchestrator) executePMSynthesize(step *state.WorkflowStep) {
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
	rephrasedTask := RunJSONAgent[dt.PmSynthesizerJson](
		o.Agents.PMSynth, prompt, invocationID, []string{step.State().Subdir()})
	modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(&rephrasedTask).Build()
	o.Stack.NewStep(state.StepPMReview, modState).WithIteration(0).Push()
}

func (o *Orchestrator) executePMReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	rephrasedTask := step.State().RephrasedTask()
	review := RunJSONAgent[dt.PmReviewJson](
		o.Agents.PMReview,
		fmt.Sprintf("ORIGINAL USER REQUEST:\n%s\n\nSYNTHESIZED SPECIFICATION:\n%s\n\nATTEMPT: %d/%d\n\nTASK:\nReview whether the synthesized specification correctly preserves the original user intent.Reject only if the specification is ambiguous, speculative, internally inconsistent, or over-expanded.",
			step.State().Task(),
			MarshalJSON(rephrasedTask),
			iteration+1,
			MAXPlanIters,
		),
		fmt.Sprintf("pm-spec-review-%d", iteration),
		[]string{step.State().Subdir()},
	)

	if reviewOk[dt.PmReviewJsonissuesElemseverity](&review) {
		speculativeExpansions := rephrasedTask.SpeculativeExpansions()
		hasExpansions := false
		if len(speculativeExpansions) > 0 {
			hasExpansions = true
		}
		modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(rephrasedTask).WithSpeculativeExpansions(state.ListToImmutableList[string](speculativeExpansions)).Build()
		if hasExpansions {
			o.Stack.NewStep(state.StepPMExpansionCleanup, modState).Push()
			return
		}

		pmFilepath := MarkdownDocumentGenerator(rephrasedTask, "product_manager_final", []string{step.State().Subdir()})

		expandedModState := state.NewWorkflowStateBuilder(modState).WithOut(buildPMtask(*rephrasedTask)).WithPMFilepath(pmFilepath).Build()
		o.Stack.NewStep(state.StepBoundary, expandedModState).Push()
		return
	}

	var revisionPrompt string
	if shouldReset(&review) {
		o.Agents.PMSynth.Reset(fmt.Sprintf("%d", iteration))
		revisionPrompt = fmt.Sprintf(
			"ORIGINAL USER REQUEST:\n%s\n\n%sPREVIOUS SYNTHESIZED SPECIFICATION:\n%s\nREVIEW FEEDBACK:\n%s\nTASK:\nRevise the synthesized specification to address the review feedback while preserving the original user intent and keeping the scope minimal.",
			step.State().Task(), step.State().Choices(), MarshalJSON(rephrasedTask), MarshalJSON(review),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE SYNTHESIZED SPECIFICATION based on feedback:\n%s", MarshalJSON(review))
	}

	o.Stack.NewStep(state.StepPMReview, step.State()).WithIteration(iteration + 1).Push()
	o.Stack.NewStep(state.StepPMSynthesize, step.State()).WithPayload(state.SimpleStringPayload(revisionPrompt)).WithIteration(iteration + 1).Push()
}

func (o *Orchestrator) executePMExpansionCleanup(step *state.WorkflowStep) {
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
		cleanSpeculative := RunJSONAgent[dt.PmExpansionCleanupJson](
			o.Agents.PMExpansionCleanup,
			fmt.Sprintf("INPUT JSON:\n%s", MarshalJSON(map[string]interface{}{"lines": expSlice})),
			"pm-expansion-cleanup",
			[]string{step.State().Subdir()},
		)
		cleanLines := cleanSpeculative.Lines()
		if len(cleanLines) == len(expSlice) {
			expSlice = cleanLines
			break
		}
		expSlice = cleanLines
	}
	rephrasedTask = updateSpeculativeExpansions(rephrasedTask, expSlice)
	pmFilepath := MarkdownDocumentGenerator(rephrasedTask, "product_manager_final", []string{step.State().Subdir()})
	modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(rephrasedTask).WithOut(buildPMtask(*rephrasedTask)).WithPMFilepath(pmFilepath).Build()
	o.Stack.NewStep(state.StepBoundary, modState).Push()
}

func buildPMtask(rephrasedTask dt.PmSynthesizerJson) string {
	out := ""
	if rephrasedTask.TaskSpecification() != "" {
		out = rephrasedTask.TaskSpecification()
	}
	if len(rephrasedTask.Files()) > 0 {
		for i, f := range rephrasedTask.Files() {
			if i == 0 {
				out += fmt.Sprintf("\n\nMentioned files:\n* %s", f)
			} else {
				out += fmt.Sprintf("\n* %s", f)
			}
		}
	}
	if len(rephrasedTask.ProperNouns()) > 0 {
		for i, p := range rephrasedTask.ProperNouns() {
			if i == 0 {
				out += fmt.Sprintf("\n\nMentioned proper nouns:\n* %s", p)
			} else {
				out += fmt.Sprintf("\n* %s", p)
			}
		}
	}
	if len(rephrasedTask.Facts()) > 0 {
		for i, f := range rephrasedTask.Facts() {
			if i == 0 {
				out += fmt.Sprintf("\n\nStated facts:\n* %s", f)
			} else {
				out += fmt.Sprintf("\n* %s", f)
			}
		}
	}
	if len(rephrasedTask.MissingButNecessaryDetails()) > 0 {
		for i, f := range rephrasedTask.MissingButNecessaryDetails() {
			if i == 0 {
				out += fmt.Sprintf("\n\nAdditional considerations:\n* %s", f)
			} else {
				out += fmt.Sprintf("\n* %s", f)
			}
		}
	}
	if len(rephrasedTask.SpeculativeExpansions()) > 0 {
		for i, f := range rephrasedTask.SpeculativeExpansions() {
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

func (o *Orchestrator) executeClassification(step *state.WorkflowStep) {
	classification := RunJSONAgent[dt.InvestigationClassifierJson](
		o.Agents.InvestigationClassifier,
		fmt.Sprintf("REFINED TASK SPECIFICATION:\n%s", wrapText(step.State().Out())),
		"investigation-classifier",
		[]string{step.State().Subdir()},
	)

	modState := state.NewWorkflowStateBuilder(step.State()).Build()
	taskType := string(classification.AType())
	logStep(fmt.Sprintf("Task classified as: %s", taskType), "CLASSIFICATION")
	logStep(fmt.Sprintf("Reasoning: %s", classification.Reasoning()), "CLASSIFICATION")
	MarkdownDocumentGenerator(classification, "investigation_classification", []string{step.State().Subdir()})

	if taskType == "investigation" {
		o.Stack.NewStep(state.StepInvestigationPlanGenerate, modState).Push()
	} else {
		o.Stack.NewStep(state.StepDecomposition, modState).Push()
	}
}

func (o *Orchestrator) executeDecomposition(step *state.WorkflowStep) {
	iteration := step.Iteration()
	var prompt string
	var invocationID string
	if iteration == 0 {
		prompt = fmt.Sprintf("TASK:\n%s", wrapText(step.State().Out()))
		invocationID = "decomposition-0"
	} else {

		review := step.Payload().DecompositionReview()
		if shouldReset(review) {
			o.Agents.Decomposition.Reset(fmt.Sprintf("%d", iteration-1))
			prompt = fmt.Sprintf(
				"TASK:\n%s\nPREVIOUS DECOMPOSITION:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the decomposition from scratch using the original task and review feedback.",
				wrapText(step.State().Out()), MarshalJSON(step.State().DecompositionResult()), MarshalJSON(review),
			)
		} else {
			prompt = fmt.Sprintf("REVISE DECOMPOSITION based on feedback:\n%s", MarshalJSON(review))
		}
		invocationID = fmt.Sprintf("decomposition-%d", iteration)
	}
	decompositionResult := Nudge[dt.SystemDecompositionJson](
		100,
		o.Agents.Decomposition, prompt, invocationID, []string{step.State().Subdir()}, o.Agents.NextStepsCleanup)
	lastResult := decompositionResult[len(decompositionResult)-1].Out
	AssertNotEmpty(lastResult, "DECOMPOSITION")
	o.Stack.NewStep(state.StepDecompositionReview, state.NewWorkflowStateBuilder(step.State()).WithDecompositionResult(&lastResult).Build()).WithReviewIteration(iteration).Push()
}

func (o *Orchestrator) executeDecompositionReview(step *state.WorkflowStep) {
	iteration := step.ReviewIteration()
	resultMap := step.State().DecompositionResult()
	decompositionReview := RunJSONAgent[dt.SystemDecompositionReviewJson](
		o.Agents.DecompositionReview,
		fmt.Sprintf("TASK:\n%s\nATTEMPT: %d/%d\nDECOMPOSITION TO REVIEW:\n%s",
			wrapText(step.State().Out()), iteration+1, MAXPlanIters, MarshalJSON(resultMap)),
		fmt.Sprintf("decomposition-review-%d", iteration),
		[]string{step.State().Subdir()},
	)

	o.Stack.NewStep(state.StepDecompositionOrchestrator, step.State()).WithPayload(state.DecompositionReviewPayload(&decompositionReview)).WithReviewIteration(iteration).Push()
}

func (o *Orchestrator) executeInvestigationPlanGenerate(step *state.WorkflowStep) {
	iteration := step.Iteration()
	var plan dt.InvestigatorPlanJson
	wrappedTask := wrapText(step.State().Out())
	if step.State().FinalInvestigationTask() != "" {
		wrappedTask = step.State().FinalInvestigationTask()
	}
	if iteration == 0 {

		plan = RunJSONAgent[dt.InvestigatorPlanJson](
			o.Agents.InvestigatorPlanner,
			fmt.Sprintf("TASK:\n%s", wrappedTask),
			"investigation-plan",
			[]string{step.State().Subdir(), "investigation"},
		)
	} else {

		payload := step.Payload().RevisionPrompt()
		plan = RunJSONAgent[dt.InvestigatorPlanJson](
			o.Agents.InvestigatorPlanner,
			payload,
			fmt.Sprintf("investigation-plan-%d", iteration),
			[]string{step.State().Subdir(), "investigation"},
		)
	}
	o.Stack.NewStep(state.StepInvestigationPlanReview, state.NewWorkflowStateBuilder(step.State()).WithInvestigationPlan(&plan).Build()).WithIteration(iteration).Push()
}

func (o *Orchestrator) executeInvestigationPlanReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	plan := step.State().InvestigationPlan()

	var wrappedTask string
	if step.State().FinalInvestigationTask() != "" {
		wrappedTask = step.State().FinalInvestigationTask()
	} else {
		wrappedTask = wrapText(step.State().Out())
	}

	qualityReview := RunJSONAgent[dt.InvestigationPlanQualityReviewJson](
		o.Agents.InvestigationPlanQuality,
		fmt.Sprintf("TASK:\n%s\nPLAN TO REVIEW:\n%s", wrappedTask, MarshalJSON(plan)),
		fmt.Sprintf("investigation-plan_quality-review-%d", iteration),
		[]string{step.State().Subdir(), "investigation"},
	)

	o.Stack.NewStep(state.StepInvestigationPlanStructuralReview, step.State()).WithIteration(iteration).WithPayload(state.InvestigationPlanQualityReviewPayload(&qualityReview)).Push()
}

func (o *Orchestrator) executeInvestigationPlanStructuralReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	plan := step.State().InvestigationPlan()
	qualityReview := step.Payload().InvestigationPlanQualityReview()

	var wrappedTask string
	if step.State().FinalInvestigationTask() != "" {
		wrappedTask = step.State().FinalInvestigationTask()
	} else {
		wrappedTask = wrapText(step.State().Out())
	}

	structReview := RunJSONAgent[dt.StructuralReviewJson](
		o.Agents.StructuralReviewer,
		fmt.Sprintf("TASK:\n%s\nPLAN TO REVIEW:\n%s", wrappedTask, MarshalJSON(plan)),
		fmt.Sprintf("investigation-struct-review-%d", iteration),
		[]string{step.State().Subdir(), "investigation"},
	)

	if reviewOk[dt.InvestigationPlanQualityReviewJsonissuesElemseverity](qualityReview) && reviewOk[dt.StructuralReviewJsonissuesElemseverity](&structReview) {
		MarkdownDocumentGenerator(plan, "investigation_plan", []string{step.State().Subdir(), "investigation"})
		workstreams := plan.Workstreams()
		if len(workstreams) == 0 {
			return
		}

		modState := state.NewWorkflowStateBuilder(step.State()).WithWorkstreams(state.ListToImmutableList(workstreams)).WithCompletedWorkstreams(immutable.NewMap[string, dt.InvestigatorFindingsJson](nil)).Build()
		wsID := strings.ToLower(strings.TrimSpace(workstreams[0].Id()))
		o.Stack.NewStep(state.StepInvestigationWorkstream, modState).WithWorkstreamID(wsID).WithWorkstreamIndex(0).Push()
		return
	}

	combinedReview := StructuralReviewPayload{
		QualityReview: *qualityReview,
		StructReview:  structReview,
	}

	var revisionPrompt string
	if shouldReset(qualityReview) || shouldReset(&structReview) {
		o.Agents.InvestigatorPlanner.Reset(fmt.Sprintf("investigation-plan-%d", iteration))
		revisionPrompt = fmt.Sprintf(
			"TASK:\n%s\nPREVIOUS PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the investigation plan from scratch using the original task and review feedback.",
			wrappedTask, MarshalJSON(plan), MarshalJSON(combinedReview),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE PLAN based on feedback:\n%s", MarshalJSON(combinedReview))
	}

	o.Stack.NewStep(state.StepInvestigationPlanGenerate, step.State()).WithPayload(state.SimpleStringPayload(revisionPrompt)).WithIteration(iteration + 1).Push()
}

func buildInvestigationWorkstreamPrompt(wsElem dt.InvestigatorPlanJsonworkstreamsElem, completedWorkstreams *immutable.Map[string, dt.InvestigatorFindingsJson]) string {
	jsonBytes := []byte(MarshalJSON(wsElem))

	var m map[string]interface{}
	if err := jsonv2.Unmarshal(jsonBytes, &m); err != nil {
		return MarshalWorkstreamElement(wsElem)
	}

	if deps, ok := m["dependencies"].([]interface{}); ok && len(deps) > 0 {
		resolvedDeps := make([]interface{}, 0, len(deps))
		for _, depID := range deps {
			if depStr, ok := depID.(string); ok {
				depIDClean := strings.ToLower(strings.TrimSpace(depStr))
				if findings, exists := completedWorkstreams.Get(depIDClean); exists {
					findingsJSON := MarshalJSON(findings)
					var findingsMap map[string]interface{}
					if err := jsonv2.Unmarshal([]byte(findingsJSON), &findingsMap); err == nil {
						if !isEmptyInvestigationFindings(findingsMap) {
							resolvedDeps = append(resolvedDeps, findingsMap)
						}
					} else {
						resolvedDeps = append(resolvedDeps, depStr)
					}
				} else {
					resolvedDeps = append(resolvedDeps, depStr)
				}
			} else {
				resolvedDeps = append(resolvedDeps, depID)
			}
		}
		m["dependencies"] = resolvedDeps
	} else {
		delete(m, "dependencies")
	}

	return MarshalJSON(m)
}

func (o *Orchestrator) executeInvestigationWorkstream(step *state.WorkflowStep) {
	workstreamID := step.WorkstreamID()
	if _, exists := step.State().CompletedWorkstreams().Get(workstreamID); exists {

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

	deps := wsElem.Dependencies()
	if len(deps) > 0 {
		canRun := true
		for _, depStr := range deps {
			depID := strings.ToLower(strings.TrimSpace(depStr))
			if _, exists := step.State().CompletedWorkstreams().Get(depID); !exists {
				canRun = false
				break
			}
		}
		if !canRun {
			return
		}
	}

	if iteration == 0 {
		o.Agents.InvestigatorExecutor.Reset(sessionSuffix)
	}

	hasHypotheses := len(wsElem.Hypotheses()) > 0
	hasDataSources := len(wsElem.DataSources()) > 0
	if !hasHypotheses || !hasDataSources {
		cw := step.State().CompletedWorkstreams()
		cw = cw.Set(workstreamID, dt.InvestigatorFindingsJson{})
		modState := state.NewWorkflowStateBuilder(step.State()).WithCompletedWorkstreams(cw).Build()

		workstreams := modState.Workstreams()
		for i := workstreamIndex + 1; i < workstreams.Len(); i++ {
			nextWs := workstreams.Get(i)
			nh := len(nextWs.Hypotheses()) > 0
			nds := len(nextWs.DataSources()) > 0
			if nh && nds {
				nid := strings.ToLower(strings.TrimSpace(nextWs.Id()))
				o.Stack.NewStep(state.StepInvestigationWorkstream, modState).WithWorkstreamID(nid).WithWorkstreamIndex(i).Push()
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
	findings := RunJSONAgent[dt.InvestigatorFindingsJson](
		o.Agents.InvestigatorExecutor,
		prompt,
		workstreamInvID,
		subdir,
	)
	cw := step.State().CompletedWorkstreams()
	cw = cw.Set(workstreamID, findings)
	modState := state.NewWorkflowStateBuilder(step.State()).WithCompletedWorkstreams(cw).Build()
	o.Stack.NewStep(state.StepInvestigationWorkstreamReview, modState).WithWorkstreamID(workstreamID).WithIteration(iteration).WithWorkstreamIndex(workstreamIndex).Push()
}

func (o *Orchestrator) executeInvestigationWorkstreamReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	workstreamID := step.WorkstreamID()
	workstreamIndex := step.WorkstreamIndex()
	domainIntID := workstreamIndex + 1
	findingsVal, findingsOk := step.State().CompletedWorkstreams().Get(workstreamID)
	if !findingsOk {
		return
	}
	wsElem := step.State().Workstreams().Get(workstreamIndex)

	gapReview := RunJSONAgent[dt.GapAnalysisReviewJson](
		o.Agents.GapAnalysisReviewer,
		fmt.Sprintf("WORKSTREAM:\n%s\nFINDINGS TO REVIEW:\n%s", buildInvestigationWorkstreamPrompt(wsElem, step.State().CompletedWorkstreams()), MarshalJSON(findingsVal)),
		fmt.Sprintf("investigation-gap-review-ws-%d-%d", domainIntID, iteration),
		[]string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", domainIntID)},
	)

	o.Stack.NewStep(state.StepInvestigationFactReview, step.State()).WithIteration(iteration).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(state.GapAnalysisReviewPayload(&gapReview)).Push()
}

func (o *Orchestrator) executeInvestigationFactReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	workstreamID := step.WorkstreamID()
	workstreamIndex := step.WorkstreamIndex()
	gapReview := step.Payload().GapAnalysisReview()
	findingsVal, findingsOk := step.State().CompletedWorkstreams().Get(workstreamID)
	if !findingsOk {
		return
	}
	wsElem := step.State().Workstreams().Get(workstreamIndex)

	factReview := RunJSONAgent[dt.FactCheckingReviewJson](
		o.Agents.FactCheckingReviewer,
		fmt.Sprintf("WORKSTREAM:\n%s\nFINDINGS TO REVIEW:\n%s", buildInvestigationWorkstreamPrompt(wsElem, step.State().CompletedWorkstreams()), MarshalJSON(findingsVal)),
		fmt.Sprintf("investigation-fact-review-ws-%d-%d", workstreamIndex+1, iteration),
		[]string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", workstreamIndex+1)},
	)

	o.Stack.NewStep(state.StepInvestigationFactReviewOrchestrator, step.State()).WithIteration(iteration).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(state.InvestigationFactReviewOrchestratorPayload(
		gapReview,
		&factReview,
		&findingsVal,
		&wsElem,
	)).Push()
}

func (o *Orchestrator) executeInvestigationSynthesis(step *state.WorkflowStep) {
	iteration := step.Iteration()

	findingsList := []interface{}{}
	wsItr := step.State().Workstreams().Iterator()
	wsItr.First()
	for !wsItr.Done() {
		_, v := wsItr.Next()
		id := strings.ToLower(strings.TrimSpace(v.Id()))
		if wf, ok := step.State().CompletedWorkstreams().Get(id); ok {
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
	report := RunJSONAgent[dt.InvestigationReportJson](
		o.Agents.SynthesisAgent,
		prompt,
		synthesisInvID,
		[]string{step.State().Subdir(), "investigation"},
	)
	modState := state.NewWorkflowStateBuilder(step.State()).WithInvestigationResults(&report).Build()

	o.Stack.NewStep(state.StepInvestigationConsistencyReview, modState).WithIteration(iteration).Push()
}

func (o *Orchestrator) executeInvestigationConsistencyReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	reportVal := *step.State().InvestigationResults()
	findingsList := []interface{}{}
	wsItr := step.State().Workstreams().Iterator()
	wsItr.First()
	for !wsItr.Done() {
		_, v := wsItr.Next()
		id := strings.ToLower(strings.TrimSpace(v.Id()))
		if wf, ok := step.State().CompletedWorkstreams().Get(id); ok {
			if isEmptyFindings(wf) {
				findingsList = append(findingsList, nil)
			} else {
				findingsList = append(findingsList, wf)
			}
		}
	}

	consistencyReview := RunJSONAgent[dt.SynthesisConsistencyReviewJson](
		o.Agents.SynthesisConsistencyReview,
		fmt.Sprintf("REPORT TO REVIEW:\n%s\nSOURCE FINDINGS:\n%s", MarshalJSON(reportVal), MarshalJSON(findingsList)),
		fmt.Sprintf("investigation-consistency_review-final-%d", iteration),
		[]string{step.State().Subdir(), "investigation"},
	)

	o.Stack.NewStep(state.StepInvestigationConsistencyReviewOrchestrator, step.State()).WithIteration(iteration).WithPayload(state.InvestigationConsistencyReviewOrchestratorPayload(
		&reportVal,
		state.ConvertFindingsList(findingsList),
		&consistencyReview,
	)).Push()
}

func (o *Orchestrator) executeDomainStart(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	if _, exists := step.State().Domains().Get(domainID); exists {
		return
	}

	domainIndex := step.DomainIndex()
	sessionSuffix := fmt.Sprintf("d%d-start", domainIntID)

	decompVal := step.State().DecompositionResult()
	decomp := decompVal.Decomposition()
	domains := decomp.Domains()
	integrationOwnership := decomp.IntegrationOwnership()
	if domainIndex < len(domains) {
		domainMap := domains[domainIndex]
		spec, archExtra := BuildArchitectInput(domainMap, integrationOwnership)
		if spec == "" {
			ds := state.NewDomainStateBuilder(nil).
				WithFinalReviewPassed(true).
				Build()
			domains := step.State().Domains()
			domains = domains.Set(domainID, ds)
			return
		}

		o.Agents.Arch.Reset(sessionSuffix)
		o.Agents.TechLead.Reset(sessionSuffix)
		o.Agents.Coder.Reset(sessionSuffix)

		coderTask := RunJSONAgent[dt.DesignToImplementPhrasingJson](
			o.Agents.DesignCleanup,
			fmt.Sprintf("INPUT TEXT:\n%s", spec),
			fmt.Sprintf("d%d-design-cleanup", domainIntID),
			[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		)

		wrappedTask := wrapText(spec + archExtra)
		wrappedCoderTask := wrapText(coderTask.Text())

		ds := state.NewDomainStateBuilder(nil).
			WithWrappedTask(wrappedTask).
			WithWrappedCoderTask(wrappedCoderTask).
			WithPMFilepath(step.State().PMFilepath()).
			WithAllChanges(immutable.NewMap[string, string](nil)).
			WithCodeSummaries(immutable.NewList[string]()).
			Build()
		domains := step.State().Domains()
		domains = domains.Set(domainID, ds)
		modState := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
		o.Stack.NewStep(state.StepArchitecture, modState).WithDomainID(domainID).WithDomainIndex(domainIndex).WithIteration(0).Push()
		return
	}

	ds := state.NewDomainStateBuilder(nil).
		WithFinalReviewPassed(true).
		Build()
	domainStates := step.State().Domains()
	domainStates = domainStates.Set(domainID, ds)
}

func (o *Orchestrator) executeArchitecture(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	revIterForNudge := reviewIter
	ds, _ := step.State().Domains().Get(domainID)

	ds = state.NewDomainStateBuilder(ds).WithCodeSummaries(immutable.NewList[string]()).WithFinalReviewPassed(false).WithMergedSummaries("").Build()

	decompRes := step.State().DecompositionResult()
	decomp := decompRes.Decomposition()
	numDomains := len(decomp.Domains())
	logStep(fmt.Sprintf("Starting iteration %d/%d for domain %d/%d",
		iteration+1, MAXTopIterations, step.DomainIndex()+1, numDomains), "ITERATION")

	var initialPrompt string

	if step.Payload() == nil {
		if ds.FinalFeedback() != nil && ds.FinalFeedback() != nil {
			initialPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE ARCHITECTURE based on feedback post implementation:\n%s", ds.PMFilepath(), MarshalJSON(ds.FinalFeedback()))
		} else {
			initialPrompt = fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s", ds.WrappedTask(), ds.PMFilepath())
		}
	}

	extraPrompt := "\nKeep architecture focused on component boundaries, ownership, and interactions. Avoid naming concrete functions, methods, language constructs, or exact code statements unless they are architecturally significant."

	var reviewPrompt string
	if step.Payload() != nil {
		review := step.Payload().ArchReview()
		if shouldReset(review) {
			o.Agents.Arch.Reset(fmt.Sprintf("d%d-arch-%d-%d", domainIntID, iteration, reviewIter-1))
			reviewPrompt = fmt.Sprintf(
				"TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nPREVIOUS ARCHITECTURE:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the architecture from scratch using the task and review feedback.",
				ds.WrappedTask(), ds.PMFilepath(), MarshalJSON(ds.Architecture()), MarshalJSON(review),
			)
		} else {
			reviewPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE ARCHITECTURE based on feedback:\n%s", ds.PMFilepath(), MarshalJSON(review))
		}
	}

	archResults := Nudge[dt.ArchJson](
		100,
		o.Agents.Arch,
		initialPrompt+reviewPrompt+extraPrompt,
		fmt.Sprintf("d%d-arch-%d-%d", domainIntID, iteration, revIterForNudge),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup,
	)
	arch := archResults[len(archResults)-1].Out
	ds = state.NewDomainStateBuilder(ds).WithArchitecture(&arch).Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	o.Stack.NewStep(state.StepArchitectureReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(revIterForNudge).Push()
}

func (o *Orchestrator) executeArchitectureReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	ds, _ := step.State().Domains().Get(domainID)
	arch := ds.Architecture()

	archReviewResults := Nudge[dt.ArchReviewJson](
		100,
		o.Agents.ArchReview,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nATTEMPT: %d/%d\nARCHITECTURE TO REVIEW:\n%s",
			ds.WrappedTask(), ds.PMFilepath(), reviewIter+1, MAXPlanIters, MarshalJSON(arch)),
		fmt.Sprintf("d%d-arch-%d-review-%d", domainIntID, iteration, reviewIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup,
	)
	archReview := archReviewResults[len(archReviewResults)-1].Out

	o.Stack.NewStep(state.StepArchitectureOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter).WithPayload(state.ArchReviewPayload(&archReview)).Push()
}

func (o *Orchestrator) executeDecompositionOrchestrator(step *state.WorkflowStep) {
	iteration := step.ReviewIteration()
	decompositionReview := step.Payload().DecompositionReview()

	if reviewOk[dt.SystemDecompositionReviewJsonissuesElemseverity](decompositionReview) {
		resultMap := step.State().DecompositionResult()
		MarkdownDocumentGenerator(resultMap, "decomposition_final", []string{step.State().Subdir()})

		decomp := resultMap.Decomposition()
		domains := decomp.Domains()
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
			o.Stack.NewStep(state.StepBoundary, modState).Push()
		}
		return
	}

	o.Stack.NewStep(state.StepDecomposition, step.State()).WithPayload(state.DecompositionReviewPayload(decompositionReview)).WithIteration(iteration + 1).Push()
}

func (o *Orchestrator) executeArchitectureOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	archReview := step.Payload().ArchReview()

	if reviewOk[dt.ArchReviewJsonissuesElemseverity](archReview) {
		ds, _ := step.State().Domains().Get(domainID)
		arch := ds.Architecture()
		domainIntID := step.DomainIndex() + 1
		docSuffix := ""
		if iteration > 0 {
			docSuffix = fmt.Sprintf("%d", iteration+1)
		}
		MarkdownDocumentGenerator(arch, fmt.Sprintf("architecture_after_reviews%s", docSuffix), []string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)})

		modState := state.NewWorkflowStateBuilder(step.State()).WithDomainCurrentStage("architecture").Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	o.Stack.NewStep(state.StepArchitecture, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter + 1).WithPayload(state.ArchReviewPayload(archReview)).Push()
}

func (o *Orchestrator) executePlanOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	planReview := step.Payload().PlanReview()

	if reviewOk[dt.PlanReviewJsonissuesElemseverity](planReview) {
		ds, _ := step.State().Domains().Get(domainID)
		plan := ds.Plan()
		domainIntID := step.DomainIndex() + 1
		docSuffix := ""
		if iteration > 0 {
			docSuffix = fmt.Sprintf("%d", iteration+1)
		}
		MarkdownDocumentGenerator(plan, fmt.Sprintf("tech_plan_after_reviews%s", docSuffix), []string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)})

		ds, _ = step.State().Domains().Get(domainID)
		ds = state.NewDomainStateBuilder(ds).WithAllChanges(immutable.NewMap[string, string](nil)).WithCodeOutputs(immutable.NewList[dt.CoderJson]()).WithTechLeadReviewResult(nil).Build()
		domains := step.State().Domains().Set(domainID, ds)
		newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
		modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("plan").Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	o.Stack.NewStep(state.StepPlan, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(state.PlanReviewPayload(planReview)).WithReviewIteration(reviewIter).Push()
}

func (o *Orchestrator) executeCodeReviewOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	codeReviewIter := step.CodeReviewIteration()
	review := step.Payload().CodeReview()

	if reviewOk[dt.CodeReviewJsonissuesElemseverity](review) {
		ds, _ := step.State().Domains().Get(domainID)

		if ds.TechLeadRevisionCycle() {

			tlIter := ds.TechLeadReviewIter()

			ds = state.NewDomainStateBuilder(ds).WithTechLeadReviewIter(tlIter + 1).Build()
			domains := step.State().Domains().Set(domainID, ds)
			newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
			modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("code").Build()
			o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
			return
		}

		modState := state.NewWorkflowStateBuilder(step.State()).WithDomainCurrentStage("code").Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	ds, _ := step.State().Domains().Get(domainID)
	crIter := codeReviewIter
	if ds != nil && ds.TechLeadRevisionCycle() {
		crIter = codeReviewIter + 1
	}
	o.Stack.NewStep(state.StepCodeImplementation, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(crIter).WithPayload(state.CodeReviewWithPlanPayload(
		review,
		ds.TechLeadReviewIter(),
	)).Push()
}

func (o *Orchestrator) executeTechLeadReviewOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	techLeadFinalReview := step.Payload().TechLeadFinalReview()

	if reviewOk[dt.TechLeadFinalJsonissuesElemseverity](techLeadFinalReview) {
		modState := state.NewWorkflowStateBuilder(step.State()).
			WithDomainCurrentStage("tech_lead_review").
			WithTechLeadDocSuffix(step.Payload().DocSuffix()).
			Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	ds, _ := step.State().Domains().Get(domainID)

	tlIter := ds.TechLeadReviewIter()

	revisionTlIter := tlIter
	ds = state.NewDomainStateBuilder(ds).
		WithTechLeadRevisionCycle(true).
		WithTechLeadReviewIter(revisionTlIter).
		WithAllChanges(func() *immutable.Map[string, string] {
			m := immutable.NewMap[string, string](nil)
			return m
		}()).
		Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	o.Stack.NewStep(state.StepCodeImplementation, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(0).WithPayload(state.CodeImplementationPayload(
		techLeadFinalReview,
		step.Payload().DocSuffix(),
		revisionTlIter,
	)).Push()
}
func (o *Orchestrator) executeArchitectureFinalReviewOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	finalFeedback := step.Payload().ArchFinal()
	ds, _ := step.State().Domains().Get(domainID)

	if reviewOk[dt.ArchFinalJsonissuesElemseverity](finalFeedback) {
		ds = state.NewDomainStateBuilder(ds).WithFinalFeedback(finalFeedback).WithFinalReviewPassed(true).Build()
		domains := step.State().Domains().Set(domainID, ds)
		newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
		modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("final_review").Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	ds = state.NewDomainStateBuilder(ds).WithFinalFeedback(finalFeedback).Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	o.Stack.NewStep(state.StepArchitecture, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration + 1).Push()
}

func (o *Orchestrator) executeInvestigationFactReviewOrchestrator(step *state.WorkflowStep) {
	iteration := step.Iteration()
	workstreamID := step.WorkstreamID()
	workstreamIndex := step.WorkstreamIndex()
	gapReview := step.Payload().GapAnalysisReview()
	factReview := step.Payload().FactReview()
	findingsVal := step.Payload().InvestigatorFindings()
	wsElem := step.Payload().WorkstreamElem()

	if reviewOk[dt.GapAnalysisReviewJsonissuesElemseverity](gapReview) && reviewOk[dt.FactCheckingReviewJsonissuesElemseverity](factReview) {
		MarkdownDocumentGenerator(*findingsVal, fmt.Sprintf("investigation_workstream_%d", workstreamIndex+1), []string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", workstreamIndex+1)})
		cw := step.State().CompletedWorkstreams()
		cw = cw.Set(workstreamID, *findingsVal)
		modState := state.NewWorkflowStateBuilder(step.State()).WithCompletedWorkstreams(cw).Build()
		allComplete := true
		wsItr := modState.Workstreams().Iterator()
		wsItr.First()
		for !wsItr.Done() {
			_, ws := wsItr.Next()
			id := strings.ToLower(strings.TrimSpace(ws.Id()))
			if _, completed := modState.CompletedWorkstreams().Get(id); !completed {
				allComplete = false
				break
			}
		}
		if allComplete {
			o.Stack.NewStep(state.StepInvestigationSynthesis, modState).Push()
		} else {
			nextIdx := workstreamIndex + 1
			workstreams := modState.Workstreams()
			if nextIdx < workstreams.Len() {
				nextWs := workstreams.Get(nextIdx)
				nextID := strings.ToLower(strings.TrimSpace(nextWs.Id()))
				o.Stack.NewStep(state.StepInvestigationWorkstream, modState).WithWorkstreamID(nextID).WithWorkstreamIndex(nextIdx).Push()
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
		o.Agents.InvestigatorExecutor.Reset(fmt.Sprintf("investigation-workstream-%d-%d", workstreamIndex+1, iteration))
		revisionPrompt = fmt.Sprintf(
			"WORKSTREAM:\n%s\nPREVIOUS FINDINGS:\n%s\nREVIEW FEEDBACK:\n%s\n\nRevise the investigation findings for this workstream.",
			buildInvestigationWorkstreamPrompt(*wsElem, step.State().CompletedWorkstreams()), MarshalJSON(findingsVal), MarshalJSON(combinedReview),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE INVESTIGATION FINDINGS based on feedback:\n%s", MarshalJSON(combinedReview))
	}

	o.Stack.NewStep(state.StepInvestigationWorkstream, step.State()).WithIteration(iteration + 1).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(state.SimpleStringPayload(revisionPrompt)).Push()
}

func (o *Orchestrator) executeInvestigationConsistencyReviewOrchestrator(step *state.WorkflowStep) {
	iteration := step.Iteration()
	reportVal := step.Payload().InvestigationReport()
	findingsList := step.Payload().FindingsList()
	consistencyReview := step.Payload().ConsistencyReview()

	if reviewOk[dt.SynthesisConsistencyReviewJsonissuesElemseverity](consistencyReview) {
		MarkdownDocumentGenerator(reportVal, "investigation_report_final", []string{step.State().Subdir(), "investigation"})
		return
	}

	o.Stack.NewStep(state.StepInvestigationSynthesis, step.State()).WithIteration(iteration + 1).WithPayload(state.InvestigationSynthesisPayload(
		reportVal,
		findingsList,
		consistencyReview,
	)).Push()
}

func (o *Orchestrator) executePlanReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	ds, _ := step.State().Domains().Get(domainID)
	arch := ds.Architecture()
	plan := ds.Plan()

	planReviewResults := Nudge[dt.PlanReviewJson](
		100,
		o.Agents.PlanReview,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nATTEMPT: %d/%d\nAPPROVED ARCHITECTURE:\n%s\nPLAN TO REVIEW:\n%s",
			ds.WrappedCoderTask(), ds.PMFilepath(), reviewIter+1, MAXPlanIters, MarshalJSON(arch), MarshalJSON(plan)),
		fmt.Sprintf("d%d-plan-%d-review-%d", domainIntID, iteration, reviewIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup,
	)
	planReview := planReviewResults[len(planReviewResults)-1].Out

	o.Stack.NewStep(state.StepPlanOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter).WithPayload(state.PlanReviewPayload(&planReview)).Push()
}

func (o *Orchestrator) executePlan(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)
	arch := ds.Architecture()

	planExtraPrompt := "\nPrefer concrete file-level changes, but avoid embedding exact code snippets unless the task is trivial and the code itself is the clearest representation of the change."

	var initialPrompt string
	planReviewIteration := 0
	if step.Payload() != nil {

		review := step.Payload().PlanReview()
		if shouldReset(review) {
			o.Agents.TechLead.Reset(fmt.Sprintf("d%d-plan-%d-%d", domainIntID, iteration, step.ReviewIteration()))
			initialPrompt = fmt.Sprintf(
				"TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s\nPREVIOUS PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the plan from scratch using the task, architecture, and review feedback.",
				ds.WrappedCoderTask(), ds.PMFilepath(), MarshalJSON(arch), MarshalJSON(ds.Plan()), MarshalJSON(review),
			)
		} else {
			initialPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE PLAN based on feedback:\n%s", ds.PMFilepath(), MarshalJSON(review))
		}
		planReviewIteration = step.ReviewIteration() + 1
		initialPrompt += planExtraPrompt
	} else {

		initialPrompt = fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s%s",
			ds.WrappedCoderTask(), ds.PMFilepath(), MarshalJSON(arch), planExtraPrompt)
	}

	planResults := Nudge[dt.PlanJson](
		100,
		o.Agents.TechLead,
		initialPrompt,
		fmt.Sprintf("d%d-plan-%d-%d", domainIntID, iteration, planReviewIteration),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup,
	)
	plan := planResults[len(planResults)-1].Out
	ds = state.NewDomainStateBuilder(ds).WithPlan(&plan).Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	o.Stack.NewStep(state.StepPlanReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(planReviewIteration).Push()
}

func (o *Orchestrator) executeCodeImplementation(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)
	plan := ds.Plan()

	coderOutputs := make([]dt.CoderJson, 0)
	allChanges := make(map[string]string)

	readFromDomainState := true
	if ds.TechLeadRevisionCycle() && step.CodeReviewIteration() == 0 {
		readFromDomainState = false
	}
	if readFromDomainState {
		if cos := ds.CodeOutputs(); cos != nil {
			coderOutputs = make([]dt.CoderJson, 0, cos.Len())
			coItr := cos.Iterator()
			coItr.First()
			for !coItr.Done() {
				_, v := coItr.Next()
				coderOutputs = append(coderOutputs, v)
			}
		}
		if ac := ds.AllChanges(); ac != nil {
			allChanges = state.ImmutableStringMapToRegular(ac)
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

	var coderInvSuffix string
	if ds.TechLeadRevisionCycle() {

		coderInvSuffix = fmt.Sprintf("-coder-%d", step.CodeReviewIteration())
	} else if step.Payload() != nil {

		coderInvSuffix = fmt.Sprintf("-coder-%d", step.CodeReviewIteration()+1)
	} else {

		coderInvSuffix = fmt.Sprintf("-coder-%d", step.CodeReviewIteration())
	}

	var review *dt.CodeReviewJson
	var coderPrompt string
	if step.Payload() != nil {
		review = step.Payload().CodeReview()
		hasTLReview := false
		var tlReview *dt.TechLeadFinalJson
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

			if review.ShouldReset() {
				resetNeeded = true
			}
		}

		if resetNeeded {

			if ds.TechLeadRevisionCycle() {
				crIter := step.CodeReviewIteration()
				if crIter == 0 {
					o.Agents.Coder.Reset(fmt.Sprintf("%s-start", invocationIDPrefix))
				} else {
					o.Agents.Coder.Reset(fmt.Sprintf("%s-%d", invocationIDPrefix, crIter-1))
				}
			} else {
				o.Agents.Coder.Reset(fmt.Sprintf("%s-%d", invocationIDPrefix, step.CodeReviewIteration()))
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
	safeFlush(o.Watcher)
	coderResults := Nudge[dt.CoderJson](
		MAXCodeIters,
		o.Agents.Coder,
		coderPrompt,
		fmt.Sprintf("%s%s", invocationIDPrefix, coderInvSuffix),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		nil,
	)
	if len(coderResults) > 0 {
		lastCache := coderResults[len(coderResults)-1]
		lastCoder := lastCache.Out
		coderOutputs = append(coderOutputs, lastCoder)
		if !lastCache.FromCache && watchmanHook == nil {
			time.Sleep(10 * time.Second)
		}
	}
	changes := safeFlush(o.Watcher)
	for k, v := range changes {
		allChanges[k] = v
	}
	ds = state.NewDomainStateBuilder(ds).WithCodeOutputs(state.ListToImmutableList[dt.CoderJson](coderOutputs)).WithAllChanges(func() *immutable.Map[string, string] {
		m := immutable.NewMap[string, string](nil)
		for k, v := range allChanges {
			m = m.Set(k, v)
		}
		return m
	}()).Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	var codeReviewPushIter int
	if ds.TechLeadRevisionCycle() {
		codeReviewPushIter = step.CodeReviewIteration()
	} else if step.Payload() != nil {

		codeReviewPushIter = step.CodeReviewIteration() + 1
	} else {

		codeReviewPushIter = step.CodeReviewIteration()
	}
	if codeReviewPushIter >= MAXCodeIters {
		o.Stack.NewStep(state.StepBoundary, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
	} else if review != nil {
		o.Stack.NewStep(state.StepCodeReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(codeReviewPushIter).WithPayload(state.CodeReviewWithPlanPayload(
			review,
			ds.TechLeadReviewIter(),
		)).Push()
	} else {
		o.Stack.NewStep(state.StepCodeReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(codeReviewPushIter).Push()
	}
}
func (o *Orchestrator) executeCodeReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	codeReviewIter := step.CodeReviewIteration()
	ds, _ := step.State().Domains().Get(domainID)
	plan := ds.Plan()

	coderOutputs := ds.CodeOutputs()
	allChanges := ds.AllChanges()

	invocationIDPrefix := fmt.Sprintf("d%d-impl-%d", domainIntID, iteration)
	if ds.TechLeadRevisionCycle() {
		tlIter := ds.TechLeadReviewIter()
		invocationIDPrefix = fmt.Sprintf("d%d-impl-revision-%d-%d", domainIntID, iteration, tlIter)
	}

	var recentChanges []dt.CoderJsonchangesElem
	var recentChangesSummary string
	var allChangesList [][]dt.CoderJsonchangesElem
	coItr := coderOutputs.Iterator()
	coItr.First()
	for !coItr.Done() {
		_, v := coItr.Next()
		changesArr := v.Changes()
		allChangesList = append(allChangesList, changesArr)
		recentChanges = changesArr
		recentChangesSummary = v.Summary()
	}

	var pastChanges [][]dt.CoderJsonchangesElem
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
		recentChangesJSON = MarshalJSON(map[string]interface{}{"changes": recentChanges, "summary": recentChangesSummary})
	} else {
		recentChangesJSON = MarshalJSON(recentChanges)
	}
	reviewPrompt := fmt.Sprintf(
		"TASK:\n%s\nATTEMPT: %d/%d\n\nMOST RECENT CHANGES:\n%s\n%s\nchanges from past iterations for context:\n%s",
		taskContext,
		codeReviewIter+1,
		MAXCodeIters,
		recentChangesJSON,
		changesPrompt(state.ImmutableStringMapToRegular(allChanges)),
		MarshalJSON(pastChanges),
	)

	isTechLeadReview := step.Payload() != nil && step.Payload().HasCoderOutputs()

	var resultsRaw []CacheResult[dt.CodeReviewJson]
	if isTechLeadReview {

		coderOutputs := step.Payload().CoderOutputs()
		allChanges := step.Payload().AllChanges()
		revisionInvPrefix := step.Payload().RevisionInvPrefix()
		initialPromptContext := step.Payload().InitialPromptContext()

		iterCount := step.Payload().IterCount()

		for iterCount < MAXCodeIters {
			logStep(fmt.Sprintf("Iteration %d/%d", iterCount+1, MAXCodeIters), "CODER EXECUTION")

			var recentChanges []dt.CoderJsonchangesElem
			var recentChangesSummary string
			var allChangesList [][]dt.CoderJsonchangesElem
			for _, v := range coderOutputs {
				changesArr := v.Changes()
				allChangesList = append(allChangesList, changesArr)
				recentChanges = changesArr
				recentChangesSummary = v.Summary()
			}

			var pastChanges [][]dt.CoderJsonchangesElem
			if len(allChangesList) > 1 {
				pastChanges = allChangesList[:len(allChangesList)-1]
			}
			var recentChangesJSON string
			if len(recentChanges) == 0 {
				recentChangesJSON = MarshalJSON(map[string]interface{}{"changes": recentChanges, "summary": recentChangesSummary})
			} else {
				recentChangesJSON = MarshalJSON(recentChanges)
			}
			reviewPrompt := fmt.Sprintf(
				"TASK:\n%s\nATTEMPT: %d/%d\n\nMOST RECENT CHANGES:\n%s\n%s\nchanges from past iterations for context:\n%s",
				initialPromptContext,
				iterCount+1,
				MAXCodeIters,
				recentChangesJSON,
				changesPrompt(allChanges),
				MarshalJSON(pastChanges),
			)

			resultsRaw = Nudge[dt.CodeReviewJson](
				MAXCodeIters,
				o.Agents.CodeReview,
				reviewPrompt,
				fmt.Sprintf("%s-code-review-%d", revisionInvPrefix, iterCount),
				[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
				o.Agents.NextStepsCleanup,
			)
			review := resultsRaw[len(resultsRaw)-1].Out

			if reviewOk[dt.CodeReviewJsonissuesElemseverity](&review) {
				break
			}

			o.Stack.NewStep(state.StepCodeReview, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(state.CodeReviewOrchestratorPayload(
				coderOutputs,
				allChanges,
				revisionInvPrefix,
				initialPromptContext,
				iterCount,
				step.Payload().TechLeadFinalReview(),
				step.Payload().DocSuffix(),
			)).Push()
			return
		}
	} else {
		resultsRaw = Nudge[dt.CodeReviewJson](
			MAXCodeIters,
			o.Agents.CodeReview,
			reviewPrompt,
			fmt.Sprintf("%s-code-review-%d", invocationIDPrefix, codeReviewIter),
			[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
			o.Agents.NextStepsCleanup,
		)
	}
	review := resultsRaw[len(resultsRaw)-1].Out

	o.Stack.NewStep(state.StepCodeReviewOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(codeReviewIter).WithPayload(state.CodeReviewWithPlanPayload(
		&review,
		ds.TechLeadReviewIter(),
	)).Push()
}

func (o *Orchestrator) executeTechLeadReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)
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
	codeSummary := fmt.Sprintf("%s\n%s", strings.Join(revisions, "\n"), changesPrompt(state.ImmutableStringMapToRegular(ds.AllChanges())))
	ds = state.NewDomainStateBuilder(ds).WithCodeSummary(codeSummary).Build()

	if step.Payload() != nil {
		cs := ds.CodeSummaries().Append(codeSummary)
		ds = state.NewDomainStateBuilder(ds).WithCodeSummaries(cs).Build()
	}
	domains := step.State().Domains().Set(domainID, ds)
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

	techLeadFinalResults := Nudge[dt.TechLeadFinalJson](
		100,
		o.Agents.TechLeadFinal,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s\nAPPROVED IMPLEMENTATION PLAN:\n%s\n%s<aggregate_implementation_summary>\n%s\n</aggregate_implementation_summary>\n",
			wrappedCoderTask, ds.PMFilepath(), MarshalJSON(arch), MarshalJSON(plan), extraPromptTL, codeSummary),
		fmt.Sprintf("d%d-tl-review-%d-%d", domainIntID, iteration, tlIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		o.Agents.NextStepsCleanup,
	)
	techLeadFinalReview := techLeadFinalResults[len(techLeadFinalResults)-1].Out

	ds = state.NewDomainStateBuilder(ds).WithTechLeadReviewResult(&techLeadFinalReview).Build()
	domains = step.State().Domains().Set(domainID, ds)
	newWs = state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	o.Stack.NewStep(state.StepTechLeadReviewOrchestrator, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(state.TechLeadReviewOrchestratorPayload(
		&techLeadFinalReview,
		docSuffix,
		tlIter+1,
	)).Push()
}

func (o *Orchestrator) executeTechLeadEnd(step *state.WorkflowStep) {
	docSuffix := step.Payload().DocSuffix()
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)

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
	ds = state.NewDomainStateBuilder(ds).WithMergedSummaries(mergedSummaries).WithTechLeadRevisionCycle(false).WithTechLeadReviewIter(0).Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	MarkdownDocumentGenerator(mergedSummaries, fmt.Sprintf("code_summary%s", docSuffix), []string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)})

	modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("tech_lead_end").Build()
	o.Stack.NewStep(state.StepBoundary, modState).
		WithDomainID(domainID).
		WithDomainIndex(step.DomainIndex()).
		WithIteration(iteration).
		Push()
}

func (o *Orchestrator) executeBoundary(step *state.WorkflowStep) {
	ws := step.State()

	if ws.DomainIterationTotal() == 0 && ws.DecompositionResult() == nil && ws.Out() != "" {
		o.Stack.NewStep(state.StepClassification, ws).Push()
		return
	}

	decompRes := ws.DecompositionResult()
	decomp := decompRes.Decomposition()
	numDomains := len(decomp.Domains())
	if ws.DecompositionResult() != nil && numDomains > 0 && ws.DomainIterationTotal() > 0 && ws.DomainCurrentStage() == "" {
		total := ws.DomainIterationTotal()

		decompVal := ws.DecompositionResult()
		decomp := decompVal.Decomposition()
		domains := decomp.Domains()
		if len(domains) == 0 {
			o.Stack.NewStep(state.StepFinalInvestigation, ws).Push()
			return
		}
		sortedIndices := sortDomainsTyped(domains)
		firstSortedIdx := sortedIndices[0]
		if firstSortedIdx >= len(domains) {
			o.Stack.NewStep(state.StepFinalInvestigation, ws).Push()
			return
		}
		firstDomainID := strings.ToLower(strings.TrimSpace(domains[firstSortedIdx].Id()))
		modState := state.NewWorkflowStateBuilder(ws).
			WithDomainIterationIndex(0).
			WithDomainIterationTotal(total).
			WithDomainCurrentStage("domain_start").
			Build()
		o.Stack.NewStep(state.StepDomainStart, modState).
			WithDomainID(firstDomainID).
			WithDomainIndex(firstSortedIdx).
			Push()
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
			o.Stack.NewStep(state.StepPlan, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(archIter).
				Push()

		case "plan":

			planIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("code").Build()
			o.Stack.NewStep(state.StepCodeImplementation, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(planIter).
				WithCodeReviewIteration(0).
				Push()

		case "code":

			ds, _ := ws.Domains().Get(step.DomainID())
			tlIter := 0
			var techLeadFinalReview dt.TechLeadFinalJson
			if ds != nil {
				tlIter = ds.TechLeadReviewIter()
				if tr := ds.TechLeadReviewResult(); tr != nil {
					techLeadFinalReview = *tr
				}
			}
			codeIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("tech_lead_review").Build()
			o.Stack.NewStep(state.StepTechLeadReview, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(codeIter).
				WithPayload(state.TechLeadReviewPayload(
					tlIter,
					&techLeadFinalReview,
				)).
				Push()

		case "tech_lead_review":

			docSuffix := ws.TechLeadDocSuffix()
			tlReviewIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("tech_lead_end").Build()
			o.Stack.NewStep(state.StepTechLeadEnd, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(tlReviewIter).
				WithPayload(state.PMExpansionCleanupPayload(docSuffix)).
				Push()

		case "tech_lead_end":

			tlEndIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("final_review").Build()
			o.Stack.NewStep(state.StepArchitectureFinalReview, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(tlEndIter).
				Push()

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
				o.Stack.NewStep(state.StepFinalInvestigation, ws).Push()
				return
			}

			if currentIdx < total-1 {
				decompVal := ws.DecompositionResult()
				decomp := decompVal.Decomposition()
				domains := decomp.Domains()
				sortedIndices := sortDomainsTyped(domains)
				nextPosition := currentIdx + 1
				nextSortedIdx := sortedIndices[nextPosition]
				if nextSortedIdx < len(domains) {
					nextDomainID := strings.ToLower(strings.TrimSpace(domains[nextSortedIdx].Id()))
					modState := state.NewWorkflowStateBuilder(ws).
						WithDomainIterationIndex(nextPosition).
						WithDomainCurrentStage("domain_start").
						Build()
					o.Stack.NewStep(state.StepDomainStart, modState).
						WithDomainID(nextDomainID).
						WithDomainIndex(nextSortedIdx).
						Push()
				}
			}
			return
		default:
			panic(fmt.Sprintf("Unexpected stage: %s", currentStage))
		}
		return
	}

	o.Stack.NewStep(state.StepFinalInvestigation, ws).Push()
}

func (o *Orchestrator) executeArchitectureFinalReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)
	arch := ds.Architecture()
	plan := ds.Plan()
	wrappedTask := ds.WrappedTask()

	finalFeedback := RunJSONAgent[dt.ArchFinalJson](
		o.Agents.ArchFinal,
		fmt.Sprintf("TASK:\n%s\nARCHITECTURE:\n%s\nAPPROVED IMPLEMENTATION PLAN:\n%s\n<aggregate_implementation_summary>\n%s\n</aggregate_implementation_summary>\n",
			wrappedTask,
			MarshalJSON(arch),
			MarshalJSON(plan),
			ds.MergedSummaries(),
		),
		fmt.Sprintf("d%d-arch-final-review-%d", domainIntID, iteration),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
	)

	o.Stack.NewStep(state.StepArchitectureFinalReviewOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(state.ArchFinalPayload(&finalFeedback)).Push()
}

func (o *Orchestrator) executeFinalInvestigation(step *state.WorkflowStep) {
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
	o.Stack.NewStep(state.StepInvestigationPlanGenerate, modState).Push()
}

func safeFlush(watcher *wman.Watchman) map[string]string {
	defer func() {
		recover()
	}()
	var out map[string]string
	if watchmanHook != nil {
		out = watchmanHook()
	} else {
		out = watcher.Flush()
	}
	trace("watchman", map[string]interface{}{"changes": out})
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

func updateSpeculativeExpansions(task *dt.PmSynthesizerJson, expansions []string) *dt.PmSynthesizerJson {
	jsonBytes := []byte(MarshalJSON(task))
	var m map[string]interface{}
	if err := jsonv2.Unmarshal(jsonBytes, &m); err == nil {
		m["speculative_expansions"] = expansions
	}
	jsonBytes = []byte(MarshalJSON(m))
	var result dt.PmSynthesizerJson
	jsonv2.Unmarshal(jsonBytes, &result)
	return &result
}

func isEmptyInvestigationFindings(m map[string]interface{}) bool {
	for _, v := range m {
		switch val := v.(type) {
		case string:
			if val != "" {
				return false
			}
		case []interface{}:
			if len(val) > 0 {
				return false
			}
		case nil:
			continue
		default:
			return false
		}
	}
	return true
}

func isEmptyFindings(f dt.InvestigatorFindingsJson) bool {
	return len(f.Conclusions()) == 0 &&
		f.ConfidenceLevel() == "" &&
		len(f.SupportingEvidence()) == 0 &&
		len(f.UnansweredQuestions()) == 0 &&
		f.WorkstreamObjective() == ""
}
