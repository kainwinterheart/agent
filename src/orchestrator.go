// =========================
// ORCHESTRATOR
// =========================
package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"agent-go/state"
	"agent-go/wman"
	"github.com/benbjohnson/immutable"
)

// watchmanHook is set by tests to mock watcher.Flush() responses.
var watchmanHook func() map[string]string

// Orchestrator manages the multi-agent workflow.
type Orchestrator struct {
	Agents  *AgentRegistry
	Watcher *wman.Watchman
	Stack   state.WorkflowStack
}

// AgentRegistry holds all AI agents used by the orchestrator.
type AgentRegistry struct {
	ProductManager             *Agent
	PMSynth                    *Agent
	PMExpansionCleanup         *Agent
	NextStepsCleanup           *Agent
	PMReview                   *Agent
	DesignCleanup              *Agent
	Arch                       *Agent
	TechLead                   *Agent
	Coder                      *Agent
	ArchReview                 *Agent
	PlanReview                 *Agent
	CodeReview                 *Agent
	TechLeadFinal              *Agent
	ArchFinal                  *Agent
	Decomposition              *Agent
	DecompositionReview        *Agent
	InvestigationClassifier    *Agent
	InvestigatorPlanner        *Agent
	InvestigatorExecutor       *Agent
	SynthesisAgent             *Agent
	GapAnalysisReviewer        *Agent
	FactCheckingReviewer       *Agent
	StructuralReviewer         *Agent
	InvestigationPlanQuality   *Agent
	SynthesisConsistencyReview *Agent
}

func NewOrchestrator(task string, subdir string) *Orchestrator {
	trace("user_input", map[string]interface{}{"text": task})
	o := &Orchestrator{}
	o.Agents = NewAgentRegistry(subdir)
	return o
}

func NewAgentRegistry(subdir string) *AgentRegistry {
	ar := &AgentRegistry{}

	ar.ProductManager = NewAgent(
		"product_manager",
		PRODUCT_MANAGER_PROMPT,
		PRODUCT_MANAGER_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("30m"),
	)
	ar.PMSynth = NewAgent(
		"pm_synth",
		PM_SYNTHESIZER_PROMPT,
		PM_SYNTHESIZER_SCHEMA,
		subdir,
		WithTimeout("30m"),
	)
	ar.PMExpansionCleanup = NewAgent(
		"pm_expansion_cleanup",
		PM_EXPANSION_CLEANUP_PROMPT,
		PM_EXPANSION_CLEANUP_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("10m"),
	)
	ar.NextStepsCleanup = NewAgent(
		"next_steps_cleanup",
		NON_CODER_NEXT_STEPS_CLEANUP_PROMPT,
		NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("10m"),
	)
	ar.PMReview = NewAgent(
		"pm_review",
		PM_REVIEW_PROMPT,
		PM_REVIEW_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	ar.DesignCleanup = NewAgent(
		"design_cleanup",
		DESIGN_TO_IMPLEMENT_PHRASING_PROMPT,
		DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("10m"),
	)

	ar.Arch = NewAgent(
		"arch", ARCH_PROMPT, ARCH_SCHEMA, subdir, WithTimeout("40m"),
	)
	ar.TechLead = NewAgent(
		"tech_lead", PLAN_PROMPT, PLAN_SCHEMA, subdir, WithTimeout("60m"),
	)
	ar.Coder = NewAgent(
		"coder", CODER_PROMPT, CODER_SCHEMA, subdir, WithTimeout("180m"),
	)

	ar.ArchReview = NewAgent(
		"arch_review", ARCH_REVIEW_PROMPT, ARCH_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	ar.PlanReview = NewAgent(
		"plan_review", PLAN_REVIEW_PROMPT, PLAN_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	ar.CodeReview = NewAgent(
		"code_review", CODE_REVIEW_PROMPT, CODE_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	ar.TechLeadFinal = NewAgent(
		"tech_lead_final", TECH_LEAD_FINAL_PROMPT, TECH_LEAD_FINAL_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	ar.ArchFinal = NewAgent(
		"arch_final", ARCH_FINAL_PROMPT, ARCH_FINAL_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	ar.Decomposition = NewAgent(
		"decomposition", SYSTEM_DECOMPOSITION_PROMPT, SYSTEM_DECOMPOSITION_SCHEMA,
		subdir, WithTimeout("40m"),
	)
	ar.DecompositionReview = NewAgent(
		"decomposition_review", SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
		SYSTEM_DECOMPOSITION_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	ar.InvestigationClassifier = NewAgent(
		"investigation_classifier", INVESTIGATION_CLASSIFIER_PROMPT,
		INVESTIGATION_CLASSIFIER_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("10m"),
	)
	ar.InvestigatorPlanner = NewAgent(
		"investigator_planner", INVESTIGATOR_PLANNER_PROMPT,
		INVESTIGATOR_PLAN_SCHEMA, subdir, WithTimeout("90m"),
	)
	ar.InvestigatorExecutor = NewAgent(
		"investigator_executor", INVESTIGATOR_EXECUTOR_PROMPT,
		INVESTIGATOR_FINDINGS_SCHEMA, subdir, WithTimeout("180m"),
	)
	ar.SynthesisAgent = NewAgent(
		"synthesis_agent", SYNTHESIS_PROMPT, INVESTIGATION_REPORT_SCHEMA,
		subdir, WithEphemeral(true), WithTimeout("90m"),
	)
	ar.GapAnalysisReviewer = NewAgent(
		"gap_analysis_reviewer", GAP_ANALYSIS_REVIEW_PROMPT,
		GAP_ANALYSIS_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)
	ar.FactCheckingReviewer = NewAgent(
		"fact_checking_reviewer", FACT_CHECKING_REVIEW_PROMPT,
		FACT_CHECKING_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)
	ar.StructuralReviewer = NewAgent(
		"structural_reviewer", STRUCTURE_REVIEW_PROMPT,
		STRUCTURAL_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)
	ar.InvestigationPlanQuality = NewAgent(
		"investigation_plan_quality_reviewer", INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT,
		INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)
	ar.SynthesisConsistencyReview = NewAgent(
		"synthesis_consistency_reviewer", SYNTHESIS_CONSISTENCY_REVIEW_PROMPT,
		SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	return ar
}

func wrapText(text string) string {
	return fmt.Sprintf("<text>\n%s\n</text>", text)
}

// =========================
// STACK-BASED WORKFLOW TYPES
// =========================

// dependencyGraph holds parsed dependency data shared between sort functions.
type dependencyGraph struct {
	idToIdx  map[string]int
	inDeg    map[int]int
	children map[int][]int
	depKeys  map[int]int // node index -> total dependency count (including unresolved)
}

// buildDependencyGraph extracts IDs and dependencies from items.
// depsKey is the JSON key name for the dependencies field
// (e.g. "dependencies" or "upstream_dependencies").
func buildDependencyGraph(items []interface{}, depsKey string) dependencyGraph {
	g := dependencyGraph{
		idToIdx:  make(map[string]int),
		inDeg:    make(map[int]int),
		children: make(map[int][]int),
		depKeys:  make(map[int]int),
	}
	for i, item := range items {
		g.inDeg[i] = 0
		if m, ok := item.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok {
				g.idToIdx[strings.ToLower(strings.TrimSpace(id))] = i
			}
			g.depKeys[i] = 0
			if deps, ok := m[depsKey].([]interface{}); ok {
				g.depKeys[i] = len(deps)
				for _, dep := range deps {
					if depStr, ok := dep.(string); ok {
						depID := strings.ToLower(strings.TrimSpace(depStr))
						if depIdx, exists := g.idToIdx[depID]; exists {
							g.inDeg[i]++
							g.children[depIdx] = append(g.children[depIdx], i)
						}
					}
				}
			}
		}
	}
	return g
}

// sortDomainsByDepCount sorts domain indices by number of upstream dependencies (ascending),
// breaking ties by original index (ascending).
// If all dependencies are resolvable (no unresolved deps), returns original order.
// If some dependencies are unresolved, sorts by total dependency count.
func sortDomainsByDepCount(items []interface{}) []int {
	g := buildDependencyGraph(items, "upstream_dependencies")

	// Check if all dependencies are resolvable
	allResolved := true
	for i := 0; i < len(items) && allResolved; i++ {
		if g.inDeg[i] != g.depKeys[i] {
			allResolved = false
		}
	}

	if allResolved {
		original := make([]int, len(items))
		for i := range items {
			original[i] = i
		}
		return original
	}

	// Sort by total dependency count (ascending), then by index (ascending)
	type depEntry struct {
		index    int
		depCount int
	}
	entries := make([]depEntry, len(items))
	for i := range items {
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

// sortWorkstreams sorts workstreams by their dependencies (topological order).
// Workstreams with no dependencies come first, then workstreams whose dependencies are satisfied.
func sortWorkstreams(items []interface{}) []interface{} {
	g := buildDependencyGraph(items, "dependencies")

	// Kahn's algorithm
	var queue []int
	for i := 0; i < len(items); i++ {
		if g.inDeg[i] == 0 {
			queue = append(queue, i)
		}
	}

	result := make([]interface{}, 0, len(items))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, items[node])
		for _, child := range g.children[node] {
			g.inDeg[child]--
			if g.inDeg[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	// Add any remaining items (cycles) - track by id to avoid map comparison
	processed := make(map[string]bool)
	for _, r := range result {
		if rm, ok := r.(map[string]interface{}); ok {
			if id, ok := rm["id"].(string); ok {
				processed[strings.ToLower(strings.TrimSpace(id))] = true
			}
		}
	}
	for _, item := range items {
		if idMap, ok := item.(map[string]interface{}); ok {
			if id, ok := idMap["id"].(string); ok {
				key := strings.ToLower(strings.TrimSpace(id))
				if !processed[key] {
					result = append(result, item)
					processed[key] = true
				}
			}
		}
	}

	return result
}

// domainMapAt returns the domain map at the given original index

// domainMapAt returns the domain map at the given original index
func domainMapAt(items []interface{}, idx int) map[string]interface{} {
	if idx < 0 || idx >= len(items) {
		return nil
	}
	if m, ok := items[idx].(map[string]interface{}); ok {
		return m
	}
	return nil
}

// RunStack is the stack-based workflow dispatcher.
func (o *Orchestrator) Run(task string, subdir string) {
	currentState := state.NewWorkflowStateBuilder(nil).
		WithTask(wrapText(task)).
		WithSubdir(subdir).
		WithDomains(immutable.NewMap[string, *state.DomainState](nil)).
		WithInvestigationResults(immutable.NewMap[string, interface{}](nil)).
		WithCompletedWorkstreams(immutable.NewMap[string, interface{}](nil)).
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

// executeStep dispatches to the appropriate step handler.
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
	candidates := []map[string]interface{}{}
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
			[]string{step.State().Subdir()},
		).(map[string]interface{})
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
	o.Stack.NewStep(state.StepPMSynthesize, state.NewWorkflowStateBuilder(step.State()).WithPMCandidates(state.ListMapToImmutableList(candidates)).WithChoices(choices).Build()).WithIteration(0).Push()
}

func (o *Orchestrator) executePMSynthesize(step *state.WorkflowStep) {
	payload := ""
	if step.Payload() != nil {
		payload = step.Payload().(string)
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
	rephrasedTask := RunJSONAgent(o.Agents.PMSynth, prompt, invocationID, []string{step.State().Subdir()}).(map[string]interface{})
	modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(state.MapToImmutable(rephrasedTask)).Build()
	o.Stack.NewStep(state.StepPMReview, modState).WithIteration(0).Push()
}

func (o *Orchestrator) executePMReview(step *state.WorkflowStep) {
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
		[]string{step.State().Subdir()},
	).(map[string]interface{})

	if reviewOk(review) {
		specVal, _ := rephrasedTask.Get("speculative_expansions")
		speculativeExpansions, _ := specVal.([]interface{})
		hasExpansions := false
		if len(speculativeExpansions) > 0 {
			hasExpansions = true
		}
		modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(rephrasedTask).WithSpeculativeExpansions(state.SliceToImmutableList(speculativeExpansions)).Build()
		if hasExpansions {
			o.Stack.NewStep(state.StepPMExpansionCleanup, modState).Push()
			return
		}

		rephrasedTaskReg := state.ImmutableMapToRegular(rephrasedTask)
		pmFilepath := MarkdownDocumentGenerator(rephrasedTaskReg, "product_manager_final", []string{step.State().Subdir()})

		expandedModState := state.NewWorkflowStateBuilder(modState).WithOut(buildPMtask(rephrasedTaskReg)).WithPMFilepath(pmFilepath).Build()
		o.Stack.NewStep(state.StepBoundary, expandedModState).Push()
		return
	}

	var revisionPrompt string
	if shouldReset(review) {
		o.Agents.PMSynth.Reset(fmt.Sprintf("%d", iteration))
		revisionPrompt = fmt.Sprintf(
			"ORIGINAL USER REQUEST:\n%s\n\n%sPREVIOUS SYNTHESIZED SPECIFICATION:\n%s\nREVIEW FEEDBACK:\n%s\nTASK:\nRevise the synthesized specification to address the review feedback while preserving the original user intent and keeping the scope minimal.",
			step.State().Task(), step.State().Choices(), MarshalJSON(rephrasedTask), MarshalJSON(review),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE SYNTHESIZED SPECIFICATION based on feedback:\n%s", MarshalJSON(review))
	}

	o.Stack.NewStep(state.StepPMReview, step.State()).WithIteration(iteration + 1).Push()
	o.Stack.NewStep(state.StepPMSynthesize, step.State()).WithPayload(revisionPrompt).WithIteration(iteration + 1).Push()
}

func (o *Orchestrator) executePMExpansionCleanup(step *state.WorkflowStep) {
	rephrasedTask := state.ImmutableMapToRegular(step.State().RephrasedTask())
	speculativeExpansions := state.ImmutableListToRegularSlice(step.State().SpeculativeExpansions())
	for {
		cleanSpeculative := RunJSONAgent(
			o.Agents.PMExpansionCleanup,
			fmt.Sprintf("INPUT JSON:\n%s", MarshalJSON(map[string]interface{}{"lines": speculativeExpansions})),
			"pm-expansion-cleanup",
			[]string{step.State().Subdir()},
		).(map[string]interface{})
		cleanLines, _ := cleanSpeculative["lines"].([]interface{})
		if len(cleanLines) == len(speculativeExpansions) {
			speculativeExpansions = cleanLines
			break
		}
		speculativeExpansions = cleanLines
	}
	rephrasedTask["speculative_expansions"] = speculativeExpansions
	pmFilepath := MarkdownDocumentGenerator(rephrasedTask, "product_manager_final", []string{step.State().Subdir()})
	modState := state.NewWorkflowStateBuilder(step.State()).WithRephrasedTask(state.MapToImmutable(rephrasedTask)).WithOut(buildPMtask(rephrasedTask)).WithPMFilepath(pmFilepath).Build()
	o.Stack.NewStep(state.StepBoundary, modState).Push()
}

func buildPMtask(rephrasedTask map[string]interface{}) string {
	out := ""
	if raw, ok := rephrasedTask["task_specification"]; ok {
		if ts, ok := raw.(string); ok {
			out = ts
		}
	}
	if raw, ok := rephrasedTask["files"]; ok {
		if files, ok := raw.([]interface{}); ok {
			for i, f := range files {
				if s, ok := f.(string); ok {
					if i == 0 {
						out += fmt.Sprintf("\n\nMentioned files:\n* %s", s)
					} else {
						out += fmt.Sprintf("\n* %s", s)
					}
				}
			}
		}
	}
	if raw, ok := rephrasedTask["proper_nouns"]; ok {
		if pn, ok := raw.([]interface{}); ok {
			for i, p := range pn {
				if s, ok := p.(string); ok {
					if i == 0 {
						out += fmt.Sprintf("\n\nMentioned proper nouns:\n* %s", s)
					} else {
						out += fmt.Sprintf("\n* %s", s)
					}
				}
			}
		}
	}
	if raw, ok := rephrasedTask["facts"]; ok {
		if facts, ok := raw.([]interface{}); ok {
			for i, f := range facts {
				if s, ok := f.(string); ok {
					if i == 0 {
						out += fmt.Sprintf("\n\nStated facts:\n* %s", s)
					} else {
						out += fmt.Sprintf("\n* %s", s)
					}
				}
			}
		}
	}
	if raw, ok := rephrasedTask["missing_but_necessary_details"]; ok {
		if mbnd, ok := raw.([]interface{}); ok {
			for i, f := range mbnd {
				if s, ok := f.(string); ok {
					if i == 0 {
						out += fmt.Sprintf("\n\nAdditional considerations:\n* %s", s)
					} else {
						out += fmt.Sprintf("\n* %s", s)
					}
				}
			}
		}
	}
	if raw, ok := rephrasedTask["speculative_expansions"]; ok {
		if se, ok := raw.([]interface{}); ok {
			for i, f := range se {
				if s, ok := f.(string); ok {
					if i == 0 {
						out += fmt.Sprintf("\n\n\nOut of scope:\n* %s", s)
					} else {
						out += fmt.Sprintf("\n* %s", s)
					}
				}
			}
		}
	}
	out += "\n"
	return out
}

func (o *Orchestrator) executeClassification(step *state.WorkflowStep) {
	classification := RunJSONAgent(
		o.Agents.InvestigationClassifier,
		fmt.Sprintf("REFINED TASK SPECIFICATION:\n%s", wrapText(step.State().Out())),
		"investigation-classifier",
		[]string{step.State().Subdir()},
	).(map[string]interface{})

	modState := state.NewWorkflowStateBuilder(step.State()).WithClassification(state.MapToImmutable(classification)).Build()
	taskType, _ := classification["type"].(string)
	logStep(fmt.Sprintf("Task classified as: %s", taskType), "CLASSIFICATION")
	logStep(fmt.Sprintf("Reasoning: %s", classification["reasoning"]), "CLASSIFICATION")
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
		// Revision: extract review from payload
		review, _ := step.Payload().(map[string]interface{})
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
	decompositionResult := Nudge(100, o.Agents.Decomposition, prompt, invocationID, []string{step.State().Subdir()}, false, o.Agents.NextStepsCleanup)
	lastResult := decompositionResult[len(decompositionResult)-1]
	resultMap, _ := lastResult.(map[string]interface{})
	AssertNotEmpty(resultMap, "DECOMPOSITION")
	o.Stack.NewStep(state.StepDecompositionReview, state.NewWorkflowStateBuilder(step.State()).WithDecompositionResult(state.MapToImmutable(resultMap)).Build()).WithReviewIteration(iteration).Push()
}

func (o *Orchestrator) executeDecompositionReview(step *state.WorkflowStep) {
	iteration := step.ReviewIteration()
	resultMap := step.State().DecompositionResult()
	decompositionReview := RunJSONAgent(
		o.Agents.DecompositionReview,
		fmt.Sprintf("TASK:\n%s\nATTEMPT: %d/%d\nDECOMPOSITION TO REVIEW:\n%s",
			wrapText(step.State().Out()), iteration+1, MAXPlanIters, MarshalJSON(resultMap)),
		fmt.Sprintf("decomposition-review-%d", iteration),
		[]string{step.State().Subdir()},
	).(map[string]interface{})

	o.Stack.NewStep(state.StepDecompositionOrchestrator, step.State()).WithPayload(decompositionReview).WithReviewIteration(iteration).Push()
}

func (o *Orchestrator) executeInvestigationPlanGenerate(step *state.WorkflowStep) {
	iteration := step.Iteration()
	var plan map[string]interface{}
	wrappedTask := wrapText(step.State().Out())
	if step.State().FinalInvestigationTask() != "" {
		wrappedTask = step.State().FinalInvestigationTask()
	}
	if iteration == 0 {
		// Initial plan generation
		plan = RunJSONAgent(
			o.Agents.InvestigatorPlanner,
			fmt.Sprintf("TASK:\n%s", wrappedTask),
			"investigation-plan",
			[]string{step.State().Subdir(), "investigation"},
		).(map[string]interface{})
	} else {
		// Synthesis/revision: use payload as prompt
		payload := step.Payload().(string)
		plan = RunJSONAgent(
			o.Agents.InvestigatorPlanner,
			payload,
			fmt.Sprintf("investigation-plan-%d", iteration),
			[]string{step.State().Subdir(), "investigation"},
		).(map[string]interface{})
	}
	o.Stack.NewStep(state.StepInvestigationPlanReview, state.NewWorkflowStateBuilder(step.State()).WithInvestigationPlan(state.MapToImmutable(plan)).Build()).WithIteration(iteration).Push()
}

func (o *Orchestrator) executeInvestigationPlanReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	plan := state.ImmutableMapToRegular(step.State().InvestigationPlan())

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
		[]string{step.State().Subdir(), "investigation"},
	).(map[string]interface{})

	o.Stack.NewStep(state.StepInvestigationPlanStructuralReview, step.State()).WithIteration(iteration).WithPayload(qualityReview).Push()
}

func (o *Orchestrator) executeInvestigationPlanStructuralReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	plan := state.ImmutableMapToRegular(step.State().InvestigationPlan())
	qualityReview := step.Payload().(map[string]interface{})

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
		[]string{step.State().Subdir(), "investigation"},
	).(map[string]interface{})

	if reviewOk(qualityReview) && reviewOk(structReview) {
		MarkdownDocumentGenerator(plan, "investigation_plan", []string{step.State().Subdir(), "investigation"})
		workstreams, _ := plan["workstreams"].([]interface{})
		if len(workstreams) == 0 {
			return
		}
		modState := state.NewWorkflowStateBuilder(step.State()).WithWorkstreams(state.SliceToImmutableList(workstreams)).WithCompletedWorkstreams(immutable.NewMap[string, interface{}](nil)).Build()

		// Push only the first workstream step; subsequent steps are pushed by
		// executeInvestigationFactReview or executeInvestigationWorkstream
		// (when a workstream is skipped due to missing hypotheses/data_sources)
		// after each workstream completes, carrying the accumulated
		// CompletedWorkstreams forward so the allComplete check works.
		wsMap, _ := workstreams[0].(map[string]interface{})
		wsID := strings.ToLower(strings.TrimSpace(wsMap["id"].(string)))
		o.Stack.NewStep(state.StepInvestigationWorkstream, modState).WithWorkstreamID(wsID).WithWorkstreamIndex(0).Push()
		return
	}

	combinedReview := map[string]interface{}{
		"quality_review": qualityReview,
		"struct_review":  structReview,
	}

	var revisionPrompt string
	if shouldReset(qualityReview) || shouldReset(structReview) {
		o.Agents.InvestigatorPlanner.Reset(fmt.Sprintf("investigation-plan-%d", iteration))
		revisionPrompt = fmt.Sprintf(
			"TASK:\n%s\nPREVIOUS PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the investigation plan from scratch using the original task and review feedback.",
			wrappedTask, MarshalJSON(plan), MarshalJSON(combinedReview),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE PLAN based on feedback:\n%s", MarshalJSON(combinedReview))
	}

	o.Stack.NewStep(state.StepInvestigationPlanGenerate, step.State()).WithPayload(revisionPrompt).WithIteration(iteration + 1).Push()
}

func (o *Orchestrator) executeInvestigationWorkstream(step *state.WorkflowStep) {
	workstreamID := step.WorkstreamID()
	if _, exists := step.State().CompletedWorkstreams().Get(workstreamID); exists {
		// Only skip if not a revision (revisions carry a payload)
		if step.Payload() == nil {
			return
		}
	}

	workstreamIndex := step.WorkstreamIndex()
	domainIntID := workstreamIndex + 1
	sessionSuffix := fmt.Sprintf("d%d-start", domainIntID)
	iteration := step.Iteration()

	workstreams := step.State().Workstreams()
	wsMap, _ := workstreams.Get(workstreamIndex).(map[string]interface{})
	wsMap["id"] = workstreamID

	// Check dependencies
	// Check if dependencies are string refs (need lookup) or already populated (keep as-is)
	deps, _ := wsMap["dependencies"].([]interface{})
	if len(deps) > 0 {
		if _, ok := deps[0].(string); ok {
			// String refs: look up in CompletedWorkstreams
			var inputs []interface{}
			canRun := true
			for _, dep := range deps {
				if depStr, ok := dep.(string); ok {
					depID := strings.ToLower(strings.TrimSpace(depStr))
					if res, exists := step.State().CompletedWorkstreams().Get(depID); exists {
						if res != nil {
							inputs = append(inputs, res)
						}
					} else {
						canRun = false
					}
				}
			}
			if !canRun {
				return
			}
			if len(inputs) > 0 {
				wsMap["dependencies"] = inputs
			}
		}
		// else: already populated with findings maps, keep as-is
	} else {
		delete(wsMap, "dependencies")
	}

	// Only reset for first iteration; revisions reuse the same session
	if iteration == 0 {
		o.Agents.InvestigatorExecutor.Reset(sessionSuffix)
	}

	hasHypotheses := wsMap["hypotheses"] != nil && len(wsMap["hypotheses"].([]interface{})) > 0
	hasDataSources := wsMap["data_sources"] != nil && len(wsMap["data_sources"].([]interface{})) > 0
	if !hasHypotheses || !hasDataSources {
		cw := step.State().CompletedWorkstreams()
		cw = cw.Set(workstreamID, nil)
		modState := state.NewWorkflowStateBuilder(step.State()).WithCompletedWorkstreams(cw).Build()
		// Push the next valid workstream
		workstreams := modState.Workstreams()
		for i := workstreamIndex + 1; i < workstreams.Len(); i++ {
			nextWs, ok := workstreams.Get(i).(map[string]interface{})
			if !ok {
				continue
			}
			nh := nextWs["hypotheses"] != nil
			if arr, ok := nextWs["hypotheses"].([]interface{}); ok && len(arr) == 0 {
				nh = false
			}
			nds := nextWs["data_sources"] != nil
			if arr, ok := nextWs["data_sources"].([]interface{}); ok && len(arr) == 0 {
				nds = false
			}
			if nh && nds {
				nid := strings.ToLower(strings.TrimSpace(nextWs["id"].(string)))
				o.Stack.NewStep(state.StepInvestigationWorkstream, modState).WithWorkstreamID(nid).WithWorkstreamIndex(i).Push()
				return
			}
		}
		return
	}

	// Check if this is a revision (payload contains revision prompt)
	var revisionPrompt string
	if step.Payload() != nil {
		revisionPrompt = step.Payload().(string)
	}

	subdir := []string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", domainIntID)}
	var prompt string
	if revisionPrompt != "" {
		prompt = revisionPrompt
	} else {
		prompt = fmt.Sprintf("WORKSTREAM:\n%s", MarshalJSON(wsMap))
	}
	workstreamInvID := fmt.Sprintf("investigation-workstream-%d", domainIntID)
	if iteration > 0 || revisionPrompt != "" {
		workstreamInvID = fmt.Sprintf("investigation-workstream-%d-%d", domainIntID, iteration)
	}
	findings := RunJSONAgent(
		o.Agents.InvestigatorExecutor,
		prompt,
		workstreamInvID,
		subdir,
	).(map[string]interface{})
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
	findings := findingsVal
	if !findingsOk || findings == nil {
		return
	}
	findingsMap, _ := findings.(map[string]interface{})
	wsMap, _ := step.State().Workstreams().Get(workstreamIndex).(map[string]interface{})

	gapReview := RunJSONAgent(
		o.Agents.GapAnalysisReviewer,
		fmt.Sprintf("WORKSTREAM:\n%s\nFINDINGS TO REVIEW:\n%s", MarshalJSON(wsMap), MarshalJSON(findingsMap)),
		fmt.Sprintf("investigation-gap-review-ws-%d-%d", domainIntID, iteration),
		[]string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", domainIntID)},
	).(map[string]interface{})

	o.Stack.NewStep(state.StepInvestigationFactReview, step.State()).WithIteration(iteration).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(gapReview).Push()
}

func (o *Orchestrator) executeInvestigationFactReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	workstreamID := step.WorkstreamID()
	workstreamIndex := step.WorkstreamIndex()
	gapReview := step.Payload().(map[string]interface{})
	findingsVal, findingsOk := step.State().CompletedWorkstreams().Get(workstreamID)
	findings := findingsVal
	if !findingsOk || findings == nil {
		return
	}
	findingsMap, _ := findings.(map[string]interface{})
	wsMap, _ := step.State().Workstreams().Get(workstreamIndex).(map[string]interface{})

	factReview := RunJSONAgent(
		o.Agents.FactCheckingReviewer,
		fmt.Sprintf("WORKSTREAM:\n%s\nFINDINGS TO REVIEW:\n%s", MarshalJSON(wsMap), MarshalJSON(findingsMap)),
		fmt.Sprintf("investigation-fact-review-ws-%d-%d", workstreamIndex+1, iteration),
		[]string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", workstreamIndex+1)},
	).(map[string]interface{})

	o.Stack.NewStep(state.StepInvestigationFactReviewOrchestrator, step.State()).WithIteration(iteration).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(map[string]interface{}{
		"gapReview":   gapReview,
		"factReview":  factReview,
		"findingsMap": findingsMap,
		"wsMap":       wsMap,
	}).Push()
}

func (o *Orchestrator) executeInvestigationSynthesis(step *state.WorkflowStep) {
	iteration := step.Iteration()

	// Gather all completed workstream results
	findingsList := []interface{}{}
	wsItr := step.State().Workstreams().Iterator()
	wsItr.First()
	for !wsItr.Done() {
		_, v := wsItr.Next()
		if wm, ok := v.(map[string]interface{}); ok {
			id := strings.ToLower(strings.TrimSpace(wm["id"].(string)))
			if wf, ok := step.State().CompletedWorkstreams().Get(id); ok {
				findingsList = append(findingsList, wf)
			}
		}
	}

	var revisionPrompt string
	if step.Payload() != nil {
		payload := step.Payload().(map[string]interface{})
		reportMap := payload["reportMap"].(map[string]interface{})
		consistencyReview := payload["consistencyReview"].(map[string]interface{})
		revisionPrompt = fmt.Sprintf(
			"REVIEW FEEDBACK:\n%s\nFINDINGS:\n%s\nPREVIOUS REPORT:\n%s\n\nRebuild the investigation report from scratch using the findings and review feedback.",
			MarshalJSON(consistencyReview), MarshalJSON(findingsList), MarshalJSON(reportMap),
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
		[]string{step.State().Subdir(), "investigation"},
	).(map[string]interface{})
	ir := step.State().InvestigationResults()
	ir = ir.Set("report", report)
	modState := state.NewWorkflowStateBuilder(step.State()).WithInvestigationResults(ir).Build()

	o.Stack.NewStep(state.StepInvestigationConsistencyReview, modState).WithIteration(iteration).Push()
}

func (o *Orchestrator) executeInvestigationConsistencyReview(step *state.WorkflowStep) {
	iteration := step.Iteration()
	reportRaw, _ := step.State().InvestigationResults().Get("report")
	reportMap, _ := reportRaw.(map[string]interface{})
	findingsList := []interface{}{}
	wsItr := step.State().Workstreams().Iterator()
	wsItr.First()
	for !wsItr.Done() {
		_, v := wsItr.Next()
		if wm, ok := v.(map[string]interface{}); ok {
			id := strings.ToLower(strings.TrimSpace(wm["id"].(string)))
			if wf, ok := step.State().CompletedWorkstreams().Get(id); ok {
				findingsList = append(findingsList, wf)
			}
		}
	}

	consistencyReview := RunJSONAgent(
		o.Agents.SynthesisConsistencyReview,
		fmt.Sprintf("REPORT TO REVIEW:\n%s\nSOURCE FINDINGS:\n%s", MarshalJSON(reportMap), MarshalJSON(findingsList)),
		fmt.Sprintf("investigation-consistency_review-final-%d", iteration),
		[]string{step.State().Subdir(), "investigation"},
	).(map[string]interface{})

	o.Stack.NewStep(state.StepInvestigationConsistencyReviewOrchestrator, step.State()).WithIteration(iteration).WithPayload(map[string]interface{}{
		"reportMap":         reportMap,
		"findingsList":      findingsList,
		"consistencyReview": consistencyReview,
	}).Push()
}

func (o *Orchestrator) executeDomainStart(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	if _, exists := step.State().Domains().Get(domainID); exists {
		return
	}

	domainIndex := step.DomainIndex()
	sessionSuffix := fmt.Sprintf("d%d-start", domainIntID)

	decompRaw, _ := step.State().DecompositionResult().Get("decomposition")
	domainsRaw := decompRaw.(map[string]interface{})["domains"].([]interface{})
	domainMap := domainMapAt(domainsRaw, domainIndex)

	integrationOwnership, _ := decompRaw.(map[string]interface{})["integration_ownership"].([]interface{})

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

	coderTask := RunJSONAgent(
		o.Agents.DesignCleanup,
		fmt.Sprintf("INPUT TEXT:\n%s", spec),
		fmt.Sprintf("d%d-design-cleanup", domainIntID),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
	).(map[string]interface{})

	wrappedTask := wrapText(spec + archExtra)
	wrappedCoderTask := wrapText(coderTask["text"].(string))

	ds := state.NewDomainStateBuilder(nil).
		WithSpec(spec).
		WithArchExtra(archExtra).
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
}

func (o *Orchestrator) executeArchitecture(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	revIterForNudge := reviewIter
	ds, _ := step.State().Domains().Get(domainID)

	ds = state.NewDomainStateBuilder(ds).WithCodeSummaries(immutable.NewList[string]()).WithFinalReviewPassed(false).WithMergedSummaries("").Build()

	decompVal, _ := step.State().DecompositionResult().Get("decomposition")
	logStep(fmt.Sprintf("Starting iteration %d/%d for domain %d/%d",
		iteration+1, MAXTopIterations, step.DomainIndex()+1,
		len(decompVal.(map[string]interface{})["domains"].([]interface{}))), "ITERATION")

	var initialPrompt string
	// Only generate initialPrompt for first-time architecture runs (no payload)
	if step.Payload() == nil {
		if ds.FinalFeedback() != nil && ds.FinalFeedback().Len() > 0 {
			initialPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE ARCHITECTURE based on feedback post implementation:\n%s", ds.PMFilepath(), MarshalJSON(ds.FinalFeedback()))
		} else {
			initialPrompt = fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s", ds.WrappedTask(), ds.PMFilepath())
		}
	}

	extraPrompt := "\nKeep architecture focused on component boundaries, ownership, and interactions. Avoid naming concrete functions, methods, language constructs, or exact code statements unless they are architecturally significant."

	// Check if this is a revision (payload contains review feedback)
	var reviewPrompt string
	if step.Payload() != nil {
		review, _ := step.Payload().(map[string]interface{})
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

	archResults := Nudge(
		100,
		o.Agents.Arch,
		initialPrompt+reviewPrompt+extraPrompt,
		fmt.Sprintf("d%d-arch-%d-%d", domainIntID, iteration, revIterForNudge),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		false,
		o.Agents.NextStepsCleanup,
	)
	arch, _ := archResults[len(archResults)-1].(map[string]interface{})
	ds = state.NewDomainStateBuilder(ds).WithArchitecture(state.MapToImmutable(arch)).Build()
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
	arch := state.ImmutableMapToRegular(ds.Architecture())

	archReviewResults := Nudge(
		100,
		o.Agents.ArchReview,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nATTEMPT: %d/%d\nARCHITECTURE TO REVIEW:\n%s",
			ds.WrappedTask(), ds.PMFilepath(), reviewIter+1, MAXPlanIters, MarshalJSON(arch)),
		fmt.Sprintf("d%d-arch-%d-review-%d", domainIntID, iteration, reviewIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		false,
		o.Agents.NextStepsCleanup,
	)
	archReview, _ := archReviewResults[len(archReviewResults)-1].(map[string]interface{})

	o.Stack.NewStep(state.StepArchitectureOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter).WithPayload(archReview).Push()
}

func (o *Orchestrator) executeDecompositionOrchestrator(step *state.WorkflowStep) {
	iteration := step.ReviewIteration()
	decompositionReview := step.Payload().(map[string]interface{})

	if reviewOk(decompositionReview) {
		resultMap := step.State().DecompositionResult()
		if raw, ok := resultMap.Get("decomposition"); ok {
			if dm, ok := raw.(map[string]interface{}); ok {
				delete(dm, "reviewer_notes")
			}
		}
		MarkdownDocumentGenerator(state.ImmutableMapToRegular(resultMap), "decomposition_final", []string{step.State().Subdir()})

		dmRaw, _ := resultMap.Get("decomposition")
		domainsRaw, _ := dmRaw.(map[string]interface{})["domains"].([]interface{})
		_, _ = dmRaw.(map[string]interface{})["integration_ownership"].([]interface{})

		if len(domainsRaw) == 0 {
			fmt.Println("No domains to execute.")
			return
		}

		sortedIndices := sortDomainsByDepCount(domainsRaw)
		firstSortedIdx := sortedIndices[0]
		domainMap := domainMapAt(domainsRaw, firstSortedIdx)
		if domainMap != nil {
			modState := state.NewWorkflowStateBuilder(step.State()).
				WithDomainIterationIndex(0).
				WithDomainIterationTotal(len(sortedIndices)).
				Build()
			o.Stack.NewStep(state.StepBoundary, modState).Push()
		}
		return
	}

	// Push to revision step with review result
	o.Stack.NewStep(state.StepDecomposition, step.State()).WithPayload(decompositionReview).WithIteration(iteration + 1).Push()
}

func (o *Orchestrator) executeArchitectureOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	archReview := step.Payload().(map[string]interface{})

	if reviewOk(archReview) {
		ds, _ := step.State().Domains().Get(domainID)
		arch := state.ImmutableMapToRegular(ds.Architecture())
		domainIntID := step.DomainIndex() + 1
		docSuffix := ""
		if iteration > 0 {
			docSuffix = fmt.Sprintf("%d", iteration+1)
		}
		MarkdownDocumentGenerator(arch, fmt.Sprintf("architecture_after_reviews%s", docSuffix), []string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)})
		if am, ok := arch["architecture"].(map[string]interface{}); ok {
			delete(am, "reviewer_notes")
		}

		modState := state.NewWorkflowStateBuilder(step.State()).WithDomainCurrentStage("architecture").Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	// Push to revision step with review result (always increment review)
	o.Stack.NewStep(state.StepArchitecture, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter + 1).WithPayload(archReview).Push()
}

func (o *Orchestrator) executePlanOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	planReview := step.Payload().(map[string]interface{})

	if reviewOk(planReview) {
		ds, _ := step.State().Domains().Get(domainID)
		plan := state.ImmutableMapToRegular(ds.Plan())
		domainIntID := step.DomainIndex() + 1
		docSuffix := ""
		if iteration > 0 {
			docSuffix = fmt.Sprintf("%d", iteration+1)
		}
		MarkdownDocumentGenerator(plan, fmt.Sprintf("tech_plan_after_reviews%s", docSuffix), []string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)})
		if pm, ok := plan["plan"].(map[string]interface{}); ok {
			delete(pm, "reviewer_notes")
		}

		// Reset AllChanges, CodeOutputs, TechLeadReviewCycle, and TechLeadReviewResult for new iteration
		ds, _ = step.State().Domains().Get(domainID)
		ds = state.NewDomainStateBuilder(ds).WithAllChanges(immutable.NewMap[string, string](nil)).WithCodeOutputs(state.SliceToImmutableList([]interface{}{})).WithTechLeadReviewCycle(0).WithTechLeadReviewResult(nil).Build()
		domains := step.State().Domains().Set(domainID, ds)
		newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
		modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("plan").Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	// Push to revision step with review result
	o.Stack.NewStep(state.StepPlan, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(planReview).WithReviewIteration(reviewIter).Push()
}

func (o *Orchestrator) executeCodeReviewOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	codeReviewIter := step.CodeReviewIteration()
	review := step.Payload().(map[string]interface{})

	if reviewOk(review) {
		ds, _ := step.State().Domains().Get(domainID)

		// Check if we're in a revision cycle (revision coder's code review passed)
		if ds.TechLeadRevisionCycle() {
			// Push StepTechLeadReview with tlIter from domain state (read tlIter from domain state)
			tlIter := ds.TechLeadReviewIter()
			// Increment TechLeadReviewIter for the NEXT revision cycle.
			// The current cycle's tlIter is preserved in the payload.
			ds = state.NewDomainStateBuilder(ds).WithTechLeadReviewIter(tlIter + 1).Build()
			domains := step.State().Domains().Set(domainID, ds)
			newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
			modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("code").Build()
			o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
			return
		}

		// Normal flow: push tech lead state update
		modState := state.NewWorkflowStateBuilder(step.State()).WithDomainCurrentStage("code").Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	// Push to revision step with review result (exclude plan, arch, wrappedCoderTask to avoid polluting coder prompt)
	revisionPayload := map[string]interface{}{
		"approved":            review["approved"],
		"approved_confidence": review["approved_confidence"],
		"approved_reason":     review["approved_reason"],
		"issues":              review["issues"],
		"resolved_issues":     review["resolved_issues"],
		"reset_reason":        review["reset_reason"],
		"should_reset":        review["should_reset"],
	}
	// For tech lead revision cycle, increment CodeReviewIteration
	crIter := codeReviewIter
	if ds, _ := step.State().Domains().Get(domainID); ds != nil && ds.TechLeadRevisionCycle() {
		tlIter := ds.TechLeadReviewIter()
		crIter = codeReviewIter + 1
		revisionPayload["tlIter"] = tlIter
	}
	o.Stack.NewStep(state.StepCodeImplementation, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(crIter).WithPayload(revisionPayload).Push()
}

func (o *Orchestrator) executeTechLeadReviewOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	payload := step.Payload().(map[string]interface{})
	techLeadFinalReview := payload["techLeadFinalReview"].(map[string]interface{})

	if reviewOk(techLeadFinalReview) {
		modState := state.NewWorkflowStateBuilder(step.State()).
			WithDomainCurrentStage("tech_lead_review").
			WithTechLeadDocSuffix(payload["docSuffix"].(string)).
			Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	// Push to revision step with review result
	// Read current domain state
	ds, _ := step.State().Domains().Get(domainID)

	// For revision coder: read tlIter from domain state (already incremented by
	// executeCodeReviewOrchestrator after the previous code review passed),
	// reset codeReviewIter to 0, set revision cycle flag
	tlIter := ds.TechLeadReviewIter()

	// Preserve coder outputs and allChanges across revision cycles
	// Keep TechLeadReviewIter at the current cycle's tlIter value;
	// the increment already happened in executeCodeReviewOrchestrator.
	revisionTlIter := tlIter
	ds = state.NewDomainStateBuilder(ds).
		WithTechLeadRevisionCycle(true).
		WithTechLeadReviewIter(revisionTlIter).
		WithTechLeadReviewCycle(ds.TechLeadReviewCycle() + 1).
		WithCodeReviewCounter(0).
		WithAllChanges(state.StringMapToImmutable(map[string]string{})).
		Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	o.Stack.NewStep(state.StepCodeImplementation, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(0).WithPayload(map[string]interface{}{
		"techLeadFinalReview": techLeadFinalReview,
		"docSuffix":           payload["docSuffix"],
		"tlIter":              revisionTlIter,
		"plan":                payload["plan"],
		"arch":                payload["arch"],
		"wrappedCoderTask":    payload["wrappedCoderTask"],
	}).Push()
}
func (o *Orchestrator) executeArchitectureFinalReviewOrchestrator(step *state.WorkflowStep) {
	domainID := step.DomainID()
	iteration := step.Iteration()
	finalFeedback := step.Payload().(map[string]interface{})
	ds, _ := step.State().Domains().Get(domainID)

	if reviewOk(finalFeedback) {
		ds = state.NewDomainStateBuilder(ds).WithFinalFeedback(state.MapToImmutable(finalFeedback)).WithFinalReviewPassed(true).Build()
		domains := step.State().Domains().Set(domainID, ds)
		newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
		modState := state.NewWorkflowStateBuilder(newWs).WithDomainCurrentStage("final_review").Build()
		o.Stack.NewStep(state.StepBoundary, modState).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
		return
	}

	// Retry: always increment iteration
	ds = state.NewDomainStateBuilder(ds).WithFinalFeedback(state.MapToImmutable(finalFeedback)).Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	o.Stack.NewStep(state.StepArchitecture, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration + 1).Push()
}

func (o *Orchestrator) executeInvestigationFactReviewOrchestrator(step *state.WorkflowStep) {
	iteration := step.Iteration()
	workstreamID := step.WorkstreamID()
	workstreamIndex := step.WorkstreamIndex()
	payload := step.Payload().(map[string]interface{})
	gapReview := payload["gapReview"].(map[string]interface{})
	factReview := payload["factReview"].(map[string]interface{})
	findingsMap := payload["findingsMap"].(map[string]interface{})
	wsMap := payload["wsMap"].(map[string]interface{})

	if reviewOk(gapReview) && reviewOk(factReview) {
		MarkdownDocumentGenerator(findingsMap, fmt.Sprintf("investigation_workstream_%d", workstreamIndex+1), []string{step.State().Subdir(), "investigation", fmt.Sprintf("%d", workstreamIndex+1)})
		cw := step.State().CompletedWorkstreams()
		cw = cw.Set(workstreamID, findingsMap)
		modState := state.NewWorkflowStateBuilder(step.State()).WithCompletedWorkstreams(cw).Build()
		allComplete := true
		wsItr := modState.Workstreams().Iterator()
		wsItr.First()
		for !wsItr.Done() {
			_, ws := wsItr.Next()
			if wm, ok := ws.(map[string]interface{}); ok {
				id := strings.ToLower(strings.TrimSpace(wm["id"].(string)))
				if _, completed := modState.CompletedWorkstreams().Get(id); !completed {
					allComplete = false
					break
				}
			}
		}
		if allComplete {
			o.Stack.NewStep(state.StepInvestigationSynthesis, modState).Push()
		} else {
			nextIdx := workstreamIndex + 1
			workstreams := modState.Workstreams()
			if nextIdx < workstreams.Len() {
				nextWs, ok := workstreams.Get(nextIdx).(map[string]interface{})
				if ok {
					nextID := strings.ToLower(strings.TrimSpace(nextWs["id"].(string)))
					o.Stack.NewStep(state.StepInvestigationWorkstream, modState).WithWorkstreamID(nextID).WithWorkstreamIndex(nextIdx).Push()
				}
			}
		}
		return
	}

	// Push to revision step with combined review
	combinedReview := map[string]interface{}{
		"gap_review":  gapReview,
		"fact_review": factReview,
	}
	var revisionPrompt string
	if shouldReset(gapReview) || shouldReset(factReview) {
		o.Agents.InvestigatorExecutor.Reset(fmt.Sprintf("investigation-workstream-%d-%d", workstreamIndex+1, iteration))
		revisionPrompt = fmt.Sprintf(
			"WORKSTREAM:\n%s\nPREVIOUS FINDINGS:\n%s\nREVIEW FEEDBACK:\n%s\n\nRevise the investigation findings for this workstream.",
			MarshalJSON(wsMap), MarshalJSON(findingsMap), MarshalJSON(combinedReview),
		)
	} else {
		revisionPrompt = fmt.Sprintf("REVISE INVESTIGATION FINDINGS based on feedback:\n%s", MarshalJSON(combinedReview))
	}

	o.Stack.NewStep(state.StepInvestigationWorkstream, step.State()).WithIteration(iteration + 1).WithWorkstreamID(workstreamID).WithWorkstreamIndex(workstreamIndex).WithPayload(revisionPrompt).Push()
}

func (o *Orchestrator) executeInvestigationConsistencyReviewOrchestrator(step *state.WorkflowStep) {
	iteration := step.Iteration()
	payload := step.Payload().(map[string]interface{})
	reportMap := payload["reportMap"].(map[string]interface{})
	findingsList := payload["findingsList"].([]interface{})
	consistencyReview := payload["consistencyReview"].(map[string]interface{})

	if reviewOk(consistencyReview) {
		MarkdownDocumentGenerator(reportMap, "investigation_report_final", []string{step.State().Subdir(), "investigation"})
		return
	}

	// Push to revision step with review result
	o.Stack.NewStep(state.StepInvestigationSynthesis, step.State()).WithIteration(iteration + 1).WithPayload(map[string]interface{}{
		"reportMap":         reportMap,
		"findingsList":      findingsList,
		"consistencyReview": consistencyReview,
	}).Push()
}

func (o *Orchestrator) executePlanReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	reviewIter := step.ReviewIteration()
	ds, _ := step.State().Domains().Get(domainID)
	arch := state.ImmutableMapToRegular(ds.Architecture())
	plan := state.ImmutableMapToRegular(ds.Plan())

	planReviewResults := Nudge(
		100,
		o.Agents.PlanReview,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nATTEMPT: %d/%d\nAPPROVED ARCHITECTURE:\n%s\nPLAN TO REVIEW:\n%s",
			ds.WrappedCoderTask(), ds.PMFilepath(), reviewIter+1, MAXPlanIters, MarshalJSON(arch), MarshalJSON(plan)),
		fmt.Sprintf("d%d-plan-%d-review-%d", domainIntID, iteration, reviewIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		false,
		o.Agents.NextStepsCleanup,
	)
	planReview, _ := planReviewResults[len(planReviewResults)-1].(map[string]interface{})

	o.Stack.NewStep(state.StepPlanOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(reviewIter).WithPayload(planReview).Push()
}

func (o *Orchestrator) executePlan(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)
	arch := state.ImmutableMapToRegular(ds.Architecture())

	planExtraPrompt := "\nPrefer concrete file-level changes, but avoid embedding exact code snippets unless the task is trivial and the code itself is the clearest representation of the change."

	var initialPrompt string
	planReviewIteration := 0
	if step.Payload() != nil {
		// Revision: use the review feedback from the payload
		review, _ := step.Payload().(map[string]interface{})
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
		// Initial plan creation
		initialPrompt = fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s%s",
			ds.WrappedCoderTask(), ds.PMFilepath(), MarshalJSON(arch), planExtraPrompt)
	}

	planResults := Nudge(
		100,
		o.Agents.TechLead,
		initialPrompt,
		fmt.Sprintf("d%d-plan-%d-%d", domainIntID, iteration, planReviewIteration),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		false,
		o.Agents.NextStepsCleanup,
	)
	plan, _ := planResults[len(planResults)-1].(map[string]interface{})
	ds = state.NewDomainStateBuilder(ds).WithPlan(state.MapToImmutable(plan)).Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()
	o.Stack.NewStep(state.StepPlanReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithReviewIteration(planReviewIteration).Push()
}

func (o *Orchestrator) executeCodeImplementation(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)
	plan := state.ImmutableMapToRegular(ds.Plan())

	// Read existing coderOutputs from domain state (accumulated across iterations)
	coderOutputs := make([]interface{}, 0)
	allChanges := make(map[string]string)
	// For tech lead revision cycles, start fresh for the first coder run
	// (matching original behavior where revision coder started with local coderOutputs=[])
	// Subsequent coder runs within the same cycle read from domain state to accumulate
	readFromDomainState := true
	if ds.TechLeadRevisionCycle() && step.CodeReviewIteration() == 0 {
		readFromDomainState = false
	}
	if readFromDomainState {
		if cos := ds.CodeOutputs(); cos != nil {
			coderOutputs = make([]interface{}, 0, cos.Len())
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

	// Determine invocation ID prefix based on whether this is a revision coder
	var invocationIDPrefix string
	var tlIter int
	if ds.TechLeadRevisionCycle() {
		// Revision coder: get tlIter from payload
		if step.Payload() != nil {
			if tlIterVal, ok := step.Payload().(map[string]interface{})["tlIter"]; ok {
				switch v := tlIterVal.(type) {
				case int:
					tlIter = v
				case float64:
					tlIter = int(v)
				}
			}
		}
		invocationIDPrefix = fmt.Sprintf("d%d-impl-revision-%d-%d", domainIntID, iteration, tlIter)
	} else {
		invocationIDPrefix = fmt.Sprintf("d%d-impl-%d", domainIntID, iteration)
	}

	// Determine coder invocation suffix based on revision type
	var coderInvSuffix string
	if ds.TechLeadRevisionCycle() {
		// Tech lead revision: coder at same iteration as step
		coderInvSuffix = fmt.Sprintf("-coder-%d", step.CodeReviewIteration())
	} else if step.Payload() != nil {
		// Regular revision: coder at step iteration + 1
		coderInvSuffix = fmt.Sprintf("-coder-%d", step.CodeReviewIteration()+1)
	} else {
		// Initial: coder at step iteration
		coderInvSuffix = fmt.Sprintf("-coder-%d", step.CodeReviewIteration())
	}

	var coderPrompt string
	if step.Payload() != nil {
		payload := step.Payload().(map[string]interface{})
		review := payload
		hasTLReview := false
		var tlReview map[string]interface{}
		if tlRev, ok := payload["techLeadFinalReview"]; ok && tlRev != nil {
			hasTLReview = true
			tlReview = tlRev.(map[string]interface{})
		}

		// Determine if we need to reset the coder
		var resetNeeded bool
		if hasTLReview {
			// TL revision: should_reset comes from tech lead review output
			if sr, ok := tlReview["should_reset"].(bool); ok {
				resetNeeded = sr
			}
		} else {
			// Regular revision: should_reset comes from code review output
			resetNeeded = shouldReset(review)
		}

		if resetNeeded {
			// Reset coder with appropriate session suffix
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
				// First TL revision coder: use plan + tech lead feedback
				coderPrompt = fmt.Sprintf(
					"APPROVED IMPLEMENTATION PLAN:\n%s\nTECH LEAD FEEDBACK:\n%s\n\nRe-implement from a clean context using the plan and tech lead feedback.",
					MarshalJSON(plan), MarshalJSON(tlReview),
				)
			} else {
				// Regular revision or subsequent TL revision coder: use plan + review feedback
				// Exclude tlIter from review JSON in prompt
				filteredReview := make(map[string]interface{})
				for k, v := range review {
					if k != "tlIter" {
						filteredReview[k] = v
					}
				}
				coderPrompt = fmt.Sprintf(
					"PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRe-implement from a clean context using the plan and review feedback. Do not assume prior implementation decisions are correct unless still justified.",
					MarshalJSON(plan), MarshalJSON(filteredReview),
				)
			}
		} else {
			if hasTLReview {
				// Tech lead revision: use tech lead feedback only
				coderPrompt = fmt.Sprintf("TECH LEAD FEEDBACK TO ADDRESS:\n%s", MarshalJSON(tlReview))
			} else {
				// Regular revision: use review feedback (filter out tlIter)
				filteredReview := make(map[string]interface{})
				for k, v := range review {
					if k != "tlIter" {
						filteredReview[k] = v
					}
				}
				coderPrompt = fmt.Sprintf("FEEDBACK TO ADDRESS:\n%s", MarshalJSON(filteredReview))
			}
		}
	} else {
		// Initial implementation
		coderPrompt = fmt.Sprintf("APPROVED IMPLEMENTATION PLAN:\n%s\n\nImplement the approved plan exactly as written.\nThe plan has already been reviewed and approved.\nDo not question whether planned file creation or modification should occur.", MarshalJSON(plan))
	}

	safeFlush(o.Watcher)
	coderResults := Nudge(
		MAXCodeIters,
		o.Agents.Coder,
		coderPrompt,
		fmt.Sprintf("%s%s", invocationIDPrefix, coderInvSuffix),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		true,
		nil,
	)
	if len(coderResults) > 0 {
		lastCoder, _ := coderResults[len(coderResults)-1].(map[string]interface{})
		outMap, _ := lastCoder["out"].(map[string]interface{})
		coderOutputs = append(coderOutputs, outMap)
		if fromCache, _ := lastCoder["from_cache"].(bool); !fromCache && watchmanHook == nil {
			time.Sleep(10 * time.Second)
		}
	}
	changes := safeFlush(o.Watcher)
	for k, v := range changes {
		allChanges[k] = v
	}
	ds = state.NewDomainStateBuilder(ds).WithCodeOutputs(state.SliceToImmutableList(coderOutputs)).WithAllChanges(state.StringMapToImmutable(allChanges)).WithCodeReviewCounter(step.CodeReviewIteration() + 1).Build()
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	reviewPayload := map[string]interface{}{}
	var codeReviewPushIter int
	if ds.TechLeadRevisionCycle() {
		reviewPayload["tlIter"] = tlIter
		codeReviewPushIter = step.CodeReviewIteration()
	} else if step.Payload() != nil {
		// Regular revision: CodeReview at step iteration + 1
		codeReviewPushIter = step.CodeReviewIteration() + 1
	} else {
		// Initial: CodeReview at step iteration
		codeReviewPushIter = step.CodeReviewIteration()
	}
	if codeReviewPushIter >= MAXCodeIters {
		o.Stack.NewStep(state.StepBoundary, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).Push()
	} else {
		o.Stack.NewStep(state.StepCodeReview, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(codeReviewPushIter).WithPayload(reviewPayload).Push()
	}
}
func (o *Orchestrator) executeCodeReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	codeReviewIter := step.CodeReviewIteration()
	ds, _ := step.State().Domains().Get(domainID)
	plan := state.ImmutableMapToRegular(ds.Plan())

	coderOutputs := ds.CodeOutputs()
	allChanges := ds.AllChanges()
	// Determine invocation ID prefix based on revision cycle
	invocationIDPrefix := fmt.Sprintf("d%d-impl-%d", domainIntID, iteration)
	if ds.TechLeadRevisionCycle() {
		tlIter := ds.TechLeadReviewIter()
		invocationIDPrefix = fmt.Sprintf("d%d-impl-revision-%d-%d", domainIntID, iteration, tlIter)
	}

	var recentChanges interface{}
	var allChangesList []interface{}
	coItr := coderOutputs.Iterator()
	coItr.First()
	for !coItr.Done() {
		_, v := coItr.Next()
		if m, ok := v.(map[string]interface{}); ok {
			changesArr, _ := m["changes"].([]interface{})
			allChangesList = append(allChangesList, changesArr)
			if len(changesArr) > 0 {
				recentChanges = changesArr
			} else {
				recentChanges = m
			}
		}
	}

	var pastChanges []interface{}
	if len(allChangesList) > 1 {
		pastChanges = allChangesList[:len(allChangesList)-1]
	}

	// Determine the task prompt context based on revision cycle
	var taskContext string
	if ds.TechLeadRevisionCycle() && ds.TechLeadReviewResult() != nil {
		// Revision code review: check if coder was reset
		tlr := state.ImmutableMapToRegular(ds.TechLeadReviewResult())
		if sr, ok := tlr["should_reset"].(bool); ok && sr {
			// Coder was reset: review based on plan + tech lead feedback
			planJSON := MarshalJSON(plan)
			taskContext = fmt.Sprintf("APPROVED IMPLEMENTATION PLAN:\n%s\nTECH LEAD FEEDBACK:\n%s\n\nRe-implement from a clean context using the plan and tech lead feedback.", planJSON, MarshalJSON(tlr))
		} else {
			// Coder was not reset: review based on tech lead feedback
			taskContext = fmt.Sprintf("TECH LEAD FEEDBACK TO ADDRESS:\n%s", MarshalJSON(tlr))
		}
	} else {
		// Normal code review: use approved implementation plan
		planJSON := MarshalJSON(plan)
		taskContext = fmt.Sprintf("APPROVED IMPLEMENTATION PLAN:\n%s\n\nImplement the approved plan exactly as written.\nThe plan has already been reviewed and approved.\nDo not question whether planned file creation or modification should occur.", planJSON)
	}

	reviewPrompt := fmt.Sprintf(
		"TASK:\n%s\nATTEMPT: %d/%d\n\nMOST RECENT CHANGES:\n%s\n%s\nchanges from past iterations for context:\n%s",
		taskContext,
		codeReviewIter+1,
		MAXCodeIters,
		MarshalJSON(recentChanges),
		changesPrompt(state.ImmutableStringMapToRegular(allChanges)),
		MarshalJSON(pastChanges),
	)

	// Check if this is a tech lead code review (payload contains coder outputs and all changes)
	isTechLeadReview := step.Payload() != nil && step.Payload().(map[string]interface{})["coderOutputs"] != nil

	var results []interface{}
	if isTechLeadReview {
		// Tech lead code review: use payload data
		payload := step.Payload().(map[string]interface{})
		coderOutputs := payload["coderOutputs"].([]interface{})
		allChanges := payload["allChanges"].(map[string]string)
		revisionInvPrefix := payload["revisionInvPrefix"].(string)
		initialPromptContext := payload["initialPromptContext"].(string)
		tlIter := ds.TechLeadReviewIter()

		iterCount := 0
		if ic, ok := payload["iterCount"]; ok {
			switch v := ic.(type) {
			case int:
				iterCount = v
			case float64:
				iterCount = int(v)
			}
		}

		for iterCount < MAXCodeIters {
			logStep(fmt.Sprintf("Iteration %d/%d", iterCount+1, MAXCodeIters), "CODER EXECUTION")

			var recentChanges interface{}
			var allChangesList []interface{}
			for _, v := range coderOutputs {
				if m, ok := v.(map[string]interface{}); ok {
					changesArr, _ := m["changes"].([]interface{})
					allChangesList = append(allChangesList, changesArr)
					if len(changesArr) > 0 {
						recentChanges = changesArr
					} else {
						recentChanges = m
					}
				}
			}

			var pastChanges []interface{}
			if len(allChangesList) > 1 {
				pastChanges = allChangesList[:len(allChangesList)-1]
			}
			reviewPrompt := fmt.Sprintf(
				"TASK:\n%s\nATTEMPT: %d/%d\n\nMOST RECENT CHANGES:\n%s\n%s\nchanges from past iterations for context:\n%s",
				initialPromptContext,
				iterCount+1,
				MAXCodeIters,
				MarshalJSON(recentChanges),
				changesPrompt(allChanges),
				MarshalJSON(pastChanges),
			)

			results = Nudge(
				MAXCodeIters,
				o.Agents.CodeReview,
				reviewPrompt,
				fmt.Sprintf("%s-code-review-%d", revisionInvPrefix, iterCount),
				[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
				false,
				o.Agents.NextStepsCleanup,
			)
			review, _ := results[len(results)-1].(map[string]interface{})

			if reviewOk(review) {
				break
			}

			o.Stack.NewStep(state.StepCodeReview, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(map[string]interface{}{
				"coderOutputs":         coderOutputs,
				"allChanges":           allChanges,
				"revisionInvPrefix":    revisionInvPrefix,
				"initialPromptContext": initialPromptContext,
				"iterCount":            iterCount,
				"tlIter":               tlIter,
				"techLeadFinalReview":  payload["techLeadFinalReview"],
				"docSuffix":            payload["docSuffix"],
				"plan":                 payload["plan"],
				"arch":                 payload["arch"],
				"wrappedCoderTask":     payload["wrappedCoderTask"],
			}).Push()
			return
		}
	} else {
		results = Nudge(
			MAXCodeIters,
			o.Agents.CodeReview,
			reviewPrompt,
			fmt.Sprintf("%s-code-review-%d", invocationIDPrefix, codeReviewIter),
			[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
			false,
			o.Agents.NextStepsCleanup,
		)
	}
	review, _ := results[len(results)-1].(map[string]interface{})

	// Build payload with coderOutputs, allChanges, plan, arch, wrappedCoderTask
	arch := state.ImmutableMapToRegular(ds.Architecture())
	wrappedCoderTask := ds.WrappedCoderTask()

	payload := map[string]interface{}{
		"plan":             plan,
		"arch":             arch,
		"wrappedCoderTask": wrappedCoderTask,
	}
	// Copy review fields into payload (excluding coderOutputs and allChanges)
	for k, v := range review {
		if k != "coderOutputs" && k != "allChanges" {
			payload[k] = v
		}
	}

	o.Stack.NewStep(state.StepCodeReviewOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithCodeReviewIteration(codeReviewIter).WithPayload(payload).Push()
}

func (o *Orchestrator) executeTechLeadReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)
	plan := state.ImmutableMapToRegular(ds.Plan())
	arch := state.ImmutableMapToRegular(ds.Architecture())
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
		outMap, _ := v.(map[string]interface{})
		summary, _ := outMap["summary"].(string)
		// Include all coder outputs with sequential numbering
		revisions = append(revisions, fmt.Sprintf("<revision%d>\n%s\n</revision%d>", i+1, summary, i+1))
	}
	codeSummary := fmt.Sprintf("%s\n%s", strings.Join(revisions, "\n"), changesPrompt(state.ImmutableStringMapToRegular(ds.AllChanges())))
	ds = state.NewDomainStateBuilder(ds).WithCodeSummary(codeSummary).Build()
	// Append to CodeSummaries for revision cycle tech lead reviews
	// (regular flow already appended in executeTechLeadStateUpdate)
	if step.Payload() != nil {
		payload := step.Payload().(map[string]interface{})
		if _, ok := payload["codeSummary"]; !ok {
			cs := ds.CodeSummaries().Append(codeSummary)
			ds = state.NewDomainStateBuilder(ds).WithCodeSummaries(cs).Build()
		}
	}
	domains := step.State().Domains().Set(domainID, ds)
	newWs := state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	// Check if this is a TL final next step (payload contains previous feedback)
	var extraPromptTL string
	if step.Payload() != nil {
		payload := step.Payload().(map[string]interface{})
		techLeadFinalReview := payload["techLeadFinalReview"].(map[string]interface{})
		if techLeadFinalReview != nil {
			extraPromptTL = fmt.Sprintf("PREVIOUS FEEDBACK:\n%s\n", MarshalJSON(techLeadFinalReview))
		}
	}

	tlIterVal := step.Payload()
	tlIter := 0
	if tlIterVal != nil {
		switch v := tlIterVal.(type) {
		case map[string]interface{}:
			if tlIterRaw, ok := v["tlIter"]; ok {
				switch tv := tlIterRaw.(type) {
				case int:
					tlIter = tv
				case float64:
					tlIter = int(tv)
				}
			}
		}
	}

	techLeadFinalResults := Nudge(
		100,
		o.Agents.TechLeadFinal,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s\nAPPROVED IMPLEMENTATION PLAN:\n%s\n%s<aggregate_implementation_summary>\n%s\n</aggregate_implementation_summary>\n",
			wrappedCoderTask, ds.PMFilepath(), MarshalJSON(arch), MarshalJSON(plan), extraPromptTL, codeSummary),
		fmt.Sprintf("d%d-tl-review-%d-%d", domainIntID, iteration, tlIter),
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
		false,
		o.Agents.NextStepsCleanup,
	)
	techLeadFinalReview, _ := techLeadFinalResults[len(techLeadFinalResults)-1].(map[string]interface{})

	// Store tech lead review result in domain state for code review prompt
	ds = state.NewDomainStateBuilder(ds).WithTechLeadReviewResult(state.MapToImmutable(techLeadFinalReview)).Build()
	domains = step.State().Domains().Set(domainID, ds)
	newWs = state.NewWorkflowStateBuilder(step.State()).WithDomains(domains).Build()

	o.Stack.NewStep(state.StepTechLeadReviewOrchestrator, newWs).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(map[string]interface{}{
		"techLeadFinalReview": techLeadFinalReview,
		"docSuffix":           docSuffix,
		"tlIter":              tlIter + 1,
		"plan":                plan,
		"arch":                arch,
		"wrappedCoderTask":    wrappedCoderTask,
		"codeSummary":         codeSummary,
	}).Push()
}

func (o *Orchestrator) executeTechLeadEnd(step *state.WorkflowStep) {
	payload := step.Payload().(map[string]interface{})
	docSuffix := payload["docSuffix"].(string)
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

	// ---- PM phase complete -> Classification step ----
	if ws.DomainIterationTotal() == 0 && ws.DecompositionResult() == nil && ws.Out() != "" {
		o.Stack.NewStep(state.StepClassification, ws).Push()
		return
	}

	// ---- Post-decomposition: start first domain ----
	if ws.DecompositionResult() != nil && ws.DecompositionResult().Len() > 0 && ws.DomainIterationTotal() > 0 && ws.DomainCurrentStage() == "" {
		total := ws.DomainIterationTotal()

		decompVal, _ := ws.DecompositionResult().Get("decomposition")
		dmRaw, ok := decompVal.(map[string]interface{})
		if !ok {
			o.Stack.NewStep(state.StepFinalInvestigation, ws).Push()
			return
		}
		domainsRaw, _ := dmRaw["domains"].([]interface{})
		if len(domainsRaw) == 0 {
			o.Stack.NewStep(state.StepFinalInvestigation, ws).Push()
			return
		}
		sortedIndices := sortDomainsByDepCount(domainsRaw)
		firstSortedIdx := sortedIndices[0]
		firstDomainMap := domainMapAt(domainsRaw, firstSortedIdx)
		if firstDomainMap == nil {
			o.Stack.NewStep(state.StepFinalInvestigation, ws).Push()
			return
		}
		firstDomainID := strings.ToLower(strings.TrimSpace(firstDomainMap["id"].(string)))
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

	// ---- Domain stage transitions ----
	if ws.DomainIterationTotal() > 0 {
		currentIdx := ws.DomainIterationIndex()
		total := ws.DomainIterationTotal()
		currentStage := ws.DomainCurrentStage()

		switch currentStage {
		case "architecture":
			// Architecture -> Plan
			archIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("plan").Build()
			o.Stack.NewStep(state.StepPlan, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(archIter).
				Push()

		case "plan":
			// Plan -> Code
			planIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("code").Build()
			o.Stack.NewStep(state.StepCodeImplementation, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(planIter).
				WithCodeReviewIteration(0).
				Push()

		case "code":
			// Code complete -> TechLeadReview
			// Read tlIter and techLeadFinalReview from domain state for revision cycle tracking
			ds, _ := ws.Domains().Get(step.DomainID())
			tlIter := 0
			var techLeadFinalReview map[string]interface{}
			if ds != nil {
				tlIter = ds.TechLeadReviewIter()
				if tr := ds.TechLeadReviewResult(); tr != nil {
					techLeadFinalReview = state.ImmutableMapToRegular(tr)
				}
			}
			codeIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("tech_lead_review").Build()
			o.Stack.NewStep(state.StepTechLeadReview, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(codeIter).
				WithPayload(map[string]interface{}{
					"tlIter":              tlIter,
					"techLeadFinalReview": techLeadFinalReview,
				}).
				Push()

		case "tech_lead_review":
			// TechLeadReview -> TechLeadEnd
			docSuffix := ws.TechLeadDocSuffix()
			tlReviewIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("tech_lead_end").Build()
			o.Stack.NewStep(state.StepTechLeadEnd, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(tlReviewIter).
				WithPayload(map[string]interface{}{"docSuffix": docSuffix}).
				Push()

		case "tech_lead_end":
			// TechLeadEnd -> ArchitectureFinalReview
			tlEndIter := step.Iteration()
			modState := state.NewWorkflowStateBuilder(ws).WithDomainCurrentStage("final_review").Build()
			o.Stack.NewStep(state.StepArchitectureFinalReview, modState).
				WithDomainID(step.DomainID()).
				WithDomainIndex(step.DomainIndex()).
				WithIteration(tlEndIter).
				Push()

		case "final_review":
			// Final review done -> check next domain or final investigation
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
			// Advance to next domain (currentIdx is position in sorted list)
			if currentIdx < total-1 {
				dmRaw, _ := ws.DecompositionResult().Get("decomposition")
				domainsRaw, _ := dmRaw.(map[string]interface{})["domains"].([]interface{})
				sortedIndices := sortDomainsByDepCount(domainsRaw)
				nextPosition := currentIdx + 1
				nextSortedIdx := sortedIndices[nextPosition]
				nextDomainMap := domainMapAt(domainsRaw, nextSortedIdx)
				if nextDomainMap != nil {
					nextDomainID := strings.ToLower(strings.TrimSpace(nextDomainMap["id"].(string)))
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

	// Fallback: push FinalInvestigation
	o.Stack.NewStep(state.StepFinalInvestigation, ws).Push()
}

func (o *Orchestrator) executeArchitectureFinalReview(step *state.WorkflowStep) {
	domainID := step.DomainID()
	domainIntID := step.DomainIndex() + 1
	iteration := step.Iteration()
	ds, _ := step.State().Domains().Get(domainID)
	arch := state.ImmutableMapToRegular(ds.Architecture())
	plan := state.ImmutableMapToRegular(ds.Plan())
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
		[]string{step.State().Subdir(), fmt.Sprintf("%d", domainIntID)},
	).(map[string]interface{})

	o.Stack.NewStep(state.StepArchitectureFinalReviewOrchestrator, step.State()).WithDomainID(domainID).WithDomainIndex(step.DomainIndex()).WithIteration(iteration).WithPayload(finalFeedback).Push()
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

// The underlying wman library panics on unmarshal errors, so we handle it here.
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
