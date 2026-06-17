// =========================
// ORCHESTRATOR
// =========================
package main

import (
	"fmt"
	"os"
	"strings"

	ac_wman "github.com/autumncoffee/wman-go"
)

// Orchestrator manages the multi-agent workflow.
type Orchestrator struct {
	Task                       string
	Subdir                     string
	DomainID                   int
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
	Watcher                    *ac_wman.Watchman
	SynthesisConsistencyReview *Agent
}

func NewOrchestrator(task string, subdir string) *Orchestrator {
	trace("user_input", map[string]interface{}{"text": task})
	o := &Orchestrator{
		Task:   wrapText(task),
		Subdir: subdir,
	}

	o.ProductManager = NewAgent(
		"product_manager",
		PRODUCT_MANAGER_PROMPT,
		PRODUCT_MANAGER_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("30m"),
	)
	o.PMSynth = NewAgent(
		"pm_synth",
		PM_SYNTHESIZER_PROMPT,
		PM_SYNTHESIZER_SCHEMA,
		subdir,
		WithTimeout("30m"),
	)
	o.PMExpansionCleanup = NewAgent(
		"pm_expansion_cleanup",
		PM_EXPANSION_CLEANUP_PROMPT,
		PM_EXPANSION_CLEANUP_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("10m"),
	)
	o.NextStepsCleanup = NewAgent(
		"next_steps_cleanup",
		NON_CODER_NEXT_STEPS_CLEANUP_PROMPT,
		NON_CODER_NEXT_STEPS_CLEANUP_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("10m"),
	)
	o.PMReview = NewAgent(
		"pm_review",
		PM_REVIEW_PROMPT,
		PM_REVIEW_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	o.DesignCleanup = NewAgent(
		"design_cleanup",
		DESIGN_TO_IMPLEMENT_PHRASING_PROMPT,
		DESIGN_TO_IMPLEMENT_PHRASING_SCHEMA,
		subdir,
		WithEphemeral(true),
		WithTimeout("10m"),
	)

	o.Arch = NewAgent(
		"arch", ARCH_PROMPT, ARCH_SCHEMA, subdir, WithTimeout("40m"),
	)
	o.TechLead = NewAgent(
		"tech_lead", PLAN_PROMPT, PLAN_SCHEMA, subdir, WithTimeout("60m"),
	)
	o.Coder = NewAgent(
		"coder", CODER_PROMPT, CODER_SCHEMA, subdir, WithTimeout("180m"),
	)

	o.ArchReview = NewAgent(
		"arch_review", ARCH_REVIEW_PROMPT, ARCH_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	o.PlanReview = NewAgent(
		"plan_review", PLAN_REVIEW_PROMPT, PLAN_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	o.CodeReview = NewAgent(
		"code_review", CODE_REVIEW_PROMPT, CODE_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	o.TechLeadFinal = NewAgent(
		"tech_lead_final", TECH_LEAD_FINAL_PROMPT, TECH_LEAD_FINAL_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	o.ArchFinal = NewAgent(
		"arch_final", ARCH_FINAL_PROMPT, ARCH_FINAL_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	o.Decomposition = NewAgent(
		"decomposition", SYSTEM_DECOMPOSITION_PROMPT, SYSTEM_DECOMPOSITION_SCHEMA,
		subdir, WithTimeout("40m"),
	)
	o.DecompositionReview = NewAgent(
		"decomposition_review", SYSTEM_DECOMPOSITION_REVIEW_PROMPT,
		SYSTEM_DECOMPOSITION_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	o.InvestigationClassifier = NewAgent(
		"investigation_classifier", INVESTIGATION_CLASSIFIER_PROMPT,
		INVESTIGATION_CLASSIFIER_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("10m"),
	)
	o.InvestigatorPlanner = NewAgent(
		"investigator_planner", INVESTIGATOR_PLANNER_PROMPT,
		INVESTIGATOR_PLAN_SCHEMA, subdir, WithTimeout("90m"),
	)
	o.InvestigatorExecutor = NewAgent(
		"investigator_executor", INVESTIGATOR_EXECUTOR_PROMPT,
		INVESTIGATOR_FINDINGS_SCHEMA, subdir, WithTimeout("180m"),
	)
	o.SynthesisAgent = NewAgent(
		"synthesis_agent", SYNTHESIS_PROMPT, INVESTIGATION_REPORT_SCHEMA,
		subdir, WithEphemeral(true), WithTimeout("90m"),
	)
	o.GapAnalysisReviewer = NewAgent(
		"gap_analysis_reviewer", GAP_ANALYSIS_REVIEW_PROMPT,
		GAP_ANALYSIS_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)
	o.FactCheckingReviewer = NewAgent(
		"fact_checking_reviewer", FACT_CHECKING_REVIEW_PROMPT,
		FACT_CHECKING_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)
	o.StructuralReviewer = NewAgent(
		"structural_reviewer", STRUCTURE_REVIEW_PROMPT,
		STRUCTURAL_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)
	o.InvestigationPlanQuality = NewAgent(
		"investigation_plan_quality_reviewer", INVESTIGATION_PLAN_QUALITY_REVIEW_PROMPT,
		INVESTIGATION_PLAN_QUALITY_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("30m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)
	o.SynthesisConsistencyReview = NewAgent(
		"synthesis_consistency_reviewer", SYNTHESIS_CONSISTENCY_REVIEW_PROMPT,
		SYNTHESIS_CONSISTENCY_REVIEW_SCHEMA, subdir,
		WithEphemeral(true), WithTimeout("60m"),
		WithResume(REVIEWER_RESUME_PROMPT),
	)

	o.Watcher = ac_wman.NewWatchman(subdir)
	o.Watcher.Start()
	return o
}

func (o *Orchestrator) PMTransformationWorkflow() (string, string) {
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
			o.ProductManager,
			fmt.Sprintf("USER REQUEST:\n%s\n\nTASK:\nProduce a focused engineering-ready specification.\n%s", o.Task, bias),
			fmt.Sprintf("pm-spec-candidate-%d", idx),
			[]string{o.Subdir},
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

	rephrasedTask := RunJSONAgent(
		o.PMSynth,
		fmt.Sprintf("ORIGINAL USER REQUEST:\n%s\n\n%sTASK:\nSelect the single best interpretation of the original user request.Preserve only the minimum assumptions necessary.Reject speculative scope expansion.", o.Task, choices),
		"pm-spec-synthesis-0",
		[]string{o.Subdir},
	).(map[string]interface{})

	for iteration := 0; iteration < MAXPlanIters; iteration++ {
		review := RunJSONAgent(
			o.PMReview,
			fmt.Sprintf("ORIGINAL USER REQUEST:\n%s\n\nSYNTHESIZED SPECIFICATION:\n%s\n\nATTEMPT: %d/%d\n\nTASK:\nReview whether the synthesized specification correctly preserves the original user intent.Reject only if the specification is ambiguous, speculative, internally inconsistent, or over-expanded.",
				o.Task,
				MarshalJSON(rephrasedTask),
				iteration+1,
				MAXPlanIters,
			),
			fmt.Sprintf("pm-spec-review-%d", iteration),
			[]string{o.Subdir},
		).(map[string]interface{})

		if reviewOk(review) {
			break
		}

		var revisionPrompt string
		if shouldReset(review) {
			logStep(fmt.Sprintf("Resetting PM Synthesizer context: %s", review["reset_reason"]), "SYSTEM")
			o.PMSynth.Reset(fmt.Sprintf("%d", iteration))
			revisionPrompt = fmt.Sprintf(
				"ORIGINAL USER REQUEST:\n%s\n\n%sPREVIOUS SYNTHESIZED SPECIFICATION:\n%s\nREVIEW FEEDBACK:\n%s\nTASK:\nRevise the synthesized specification to address the review feedback while preserving the original user intent and keeping the scope minimal.",
				o.Task, choices, MarshalJSON(rephrasedTask), MarshalJSON(review),
			)
		} else {
			revisionPrompt = fmt.Sprintf("REVISE SYNTHESIZED SPECIFICATION based on feedback:\n%s", MarshalJSON(review))
		}
		rephrasedTask = RunJSONAgent(
			o.PMSynth,
			revisionPrompt,
			fmt.Sprintf("pm-spec-synthesis-%d", iteration+1),
			[]string{o.Subdir},
		).(map[string]interface{})
	}

	speculativeExpansions, _ := rephrasedTask["speculative_expansions"].([]interface{})
	if _, ok := rephrasedTask["speculative_expansions"]; ok && len(speculativeExpansions) > 0 {
		for {
			cleanSpeculative := RunJSONAgent(
				o.PMExpansionCleanup,
				fmt.Sprintf("INPUT JSON:\n%s", MarshalJSON(map[string]interface{}{"lines": speculativeExpansions})),
				"pm-expansion-cleanup",
				[]string{o.Subdir},
			).(map[string]interface{})
			cleanLines, _ := cleanSpeculative["lines"].([]interface{})
			if len(cleanLines) == len(speculativeExpansions) {
				speculativeExpansions = cleanLines
				break
			}
		}
		rephrasedTask["speculative_expansions"] = speculativeExpansions
	}

	pmFilepath := MarkdownDocumentGenerator(rephrasedTask, "product_manager_final", []string{o.Subdir})

	out := ""
	if ts, ok := rephrasedTask["task_specification"].(string); ok {
		out = ts
	}
	if files, ok := rephrasedTask["files"].([]interface{}); ok {
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
	if pn, ok := rephrasedTask["proper_nouns"].([]interface{}); ok {
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
	if facts, ok := rephrasedTask["facts"].([]interface{}); ok {
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
	if mbnd, ok := rephrasedTask["missing_but_necessary_details"].([]interface{}); ok {
		for i, f := range mbnd {
			if s, ok := f.(string); ok {
				if i == 0 {
					out += fmt.Sprintf("\n\n\nAdditional considerations:\n* %s", s)
				} else {
					out += fmt.Sprintf("\n* %s", s)
				}
			}
		}
	}
	if se, ok := rephrasedTask["speculative_expansions"].([]interface{}); ok {
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
	out += "\n"

	return out, pmFilepath
}

func (o *Orchestrator) Run() {
	o._Run()
}

func (o *Orchestrator) _Run() {
	pmOutput, pmFilepath := o.PMTransformationWorkflow()
	rootTask := wrapText(pmOutput)

	classification := RunJSONAgent(
		o.InvestigationClassifier,
		fmt.Sprintf("REFINED TASK SPECIFICATION:\n%s", rootTask),
		"investigation-classifier",
		[]string{o.Subdir},
	).(map[string]interface{})

	taskType, _ := classification["type"].(string)
	logStep(fmt.Sprintf("Task classified as: %s", taskType), "CLASSIFICATION")
	logStep(fmt.Sprintf("Reasoning: %s", classification["reasoning"]), "CLASSIFICATION")

	MarkdownDocumentGenerator(classification, "investigation_classification", []string{o.Subdir})

	if taskType == "investigation" {
		report := o.InvestigationWorkflow(pmOutput)
		fmt.Println(report)
		return
	}

	decomposition := o.DecompositionWorkflow(rootTask)

	seq := newWorkflowSequencer(o)
	seq.Run(pmFilepath, decomposition)
}

func (o *Orchestrator) DecompositionWorkflow(task string) map[string]interface{} {
	decompositionResult := Nudge(
		100,
		o.Decomposition,
		fmt.Sprintf("TASK:\n%s", task),
		"decomposition-0",
		[]string{o.Subdir},
		false,
		o.NextStepsCleanup,
	)
	lastResult := decompositionResult[len(decompositionResult)-1]
	resultMap, _ := lastResult.(map[string]interface{})
	AssertNotEmpty(resultMap, "DECOMPOSITION")

	for i := 0; i < MAXPlanIters; i++ {
		decompositionReview := RunJSONAgent(
			o.DecompositionReview,
			fmt.Sprintf("TASK:\n%s\nATTEMPT: %d/%d\nDECOMPOSITION TO REVIEW:\n%s",
				task, i+1, MAXPlanIters, MarshalJSON(resultMap)),
			fmt.Sprintf("decomposition-review-%d", i),
			[]string{o.Subdir},
		).(map[string]interface{})

		if reviewOk(decompositionReview) {
			break
		}

		var revisionPrompt string
		if shouldReset(decompositionReview) {
			logStep(fmt.Sprintf("Resetting decomposition context: %s", decompositionReview["reset_reason"]), "SYSTEM")
			o.Decomposition.Reset(fmt.Sprintf("%d", i))
			revisionPrompt = fmt.Sprintf(
				"TASK:\n%s\nPREVIOUS DECOMPOSITION:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the decomposition from scratch using the original task and review feedback.",
				task, MarshalJSON(resultMap), MarshalJSON(decompositionReview),
			)
		} else {
			revisionPrompt = fmt.Sprintf("REVISE DECOMPOSITION based on feedback:\n%s", MarshalJSON(decompositionReview))
		}
		decompositionResultLocal := Nudge(
			100,
			o.Decomposition,
			revisionPrompt,
			fmt.Sprintf("decomposition-%d", i+1),
			[]string{o.Subdir},
			false,
			o.NextStepsCleanup,
		)
		lastResultLocal := decompositionResultLocal[len(decompositionResultLocal)-1]
		resultMapLocal, _ := lastResultLocal.(map[string]interface{})
		resultMap = resultMapLocal
	}

	if dm, ok := resultMap["decomposition"].(map[string]interface{}); ok {
		delete(dm, "reviewer_notes")
	}
	MarkdownDocumentGenerator(resultMap, "decomposition_final", []string{o.Subdir})

	return resultMap
}

func (o *Orchestrator) InvestigationWorkflow(task string) string {
	subdir := []string{o.Subdir, "investigation"}
	wrappedTask := wrapText(task)

	plan := RunJSONAgent(
		o.InvestigatorPlanner,
		fmt.Sprintf("TASK:\n%s", wrappedTask),
		"investigation-plan",
		subdir,
	).(map[string]interface{})

	for i := 0; i < MAXPlanIters; i++ {
		qualityReview := RunJSONAgent(
			o.InvestigationPlanQuality,
			fmt.Sprintf("TASK:\n%s\nPLAN TO REVIEW:\n%s", wrappedTask, MarshalJSON(plan)),
			fmt.Sprintf("investigation-plan_quality-review-%d", i),
			subdir,
		).(map[string]interface{})
		structReview := RunJSONAgent(
			o.StructuralReviewer,
			fmt.Sprintf("TASK:\n%s\nPLAN TO REVIEW:\n%s", wrappedTask, MarshalJSON(plan)),
			fmt.Sprintf("investigation-struct-review-%d", i),
			subdir,
		).(map[string]interface{})

		if reviewOk(qualityReview) && reviewOk(structReview) {
			break
		}

		combinedReview := map[string]interface{}{
			"quality_review": qualityReview,
			"struct_review":  structReview,
		}

		var revisionPrompt string
		if shouldReset(qualityReview) || shouldReset(structReview) {
			logStep(fmt.Sprintf("Resetting investigator planner context: %s", combinedReview["reset_reason"]), "SYSTEM")
			o.InvestigatorPlanner.Reset(fmt.Sprintf("investigation-plan-%d", i))
			revisionPrompt = fmt.Sprintf(
				"TASK:\n%s\nPREVIOUS PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the investigation plan from scratch using the original task and review feedback.",
				wrappedTask, MarshalJSON(plan), MarshalJSON(combinedReview),
			)
		} else {
			revisionPrompt = fmt.Sprintf("REVISE PLAN based on feedback:\n%s", MarshalJSON(combinedReview))
		}
		plan = RunJSONAgent(
			o.InvestigatorPlanner,
			revisionPrompt,
			fmt.Sprintf("investigation-plan-%d", i+1),
			subdir,
		).(map[string]interface{})
	}

	MarkdownDocumentGenerator(plan, "investigation_plan", subdir)

	workstreams, _ := plan["workstreams"].([]interface{})
	if len(workstreams) == 0 {
		return "## Executive Summary\n\nNo investigation plan was produced."
	}

	completedWorkstreams := make(map[string]interface{})
	haveWork := true
	madeProgress := true

	for haveWork {
		haveWork = false
		completedBefore := len(completedWorkstreams)
		for N, workstream := range workstreams {
			wsMap, _ := workstream.(map[string]interface{})
			wsMap["id"] = strings.ToLower(strings.TrimSpace(wsMap["id"].(string)))
			if _, exists := completedWorkstreams[wsMap["id"].(string)]; exists {
				continue
			}
			haveWork = true
			canRun := true
			var inputs []interface{}
			deps, _ := wsMap["dependencies"].([]interface{})
			for _, dep := range deps {
				depStr := strings.ToLower(strings.TrimSpace(dep.(string)))
				if res, exists := completedWorkstreams[depStr]; exists {
					if res != nil {
						inputs = append(inputs, res)
					}
				} else {
					canRun = false
				}
			}
			if !canRun {
				if madeProgress {
					continue
				} else {
					madeProgress = true
				}
			}
			if len(inputs) > 0 {
				wsMap["dependencies"] = inputs
			} else {
				delete(wsMap, "dependencies")
			}
			o.DomainID = N + 1
			sessionSuffix := fmt.Sprintf("d%d-start", o.DomainID)
			o.InvestigatorExecutor.Reset(sessionSuffix)

			hasHypotheses := wsMap["hypotheses"] != nil && len(wsMap["hypotheses"].([]interface{})) > 0
			hasDataSources := wsMap["data_sources"] != nil && len(wsMap["data_sources"].([]interface{})) > 0
			if !hasHypotheses || !hasDataSources {
				completedWorkstreams[wsMap["id"].(string)] = nil
				continue
			}

			findings := RunJSONAgent(
				o.InvestigatorExecutor,
				fmt.Sprintf("WORKSTREAM:\n%s", MarshalJSON(wsMap)),
				fmt.Sprintf("investigation-workstream-%d", N+1),
				append(subdir, fmt.Sprintf("%d", o.DomainID)),
			).(map[string]interface{})

			for i := 0; i < MAXPlanIters; i++ {
				gapReview := RunJSONAgent(
					o.GapAnalysisReviewer,
					fmt.Sprintf("WORKSTREAM:\n%s\nFINDINGS TO REVIEW:\n%s", MarshalJSON(wsMap), MarshalJSON(findings)),
					fmt.Sprintf("investigation-gap-review-ws-%d-%d", N+1, i),
					append(subdir, fmt.Sprintf("%d", o.DomainID)),
				).(map[string]interface{})
				factReview := RunJSONAgent(
					o.FactCheckingReviewer,
					fmt.Sprintf("WORKSTREAM:\n%s\nFINDINGS TO REVIEW:\n%s", MarshalJSON(wsMap), MarshalJSON(findings)),
					fmt.Sprintf("investigation-fact-review-ws-%d-%d", N+1, i),
					append(subdir, fmt.Sprintf("%d", o.DomainID)),
				).(map[string]interface{})

				if reviewOk(gapReview) && reviewOk(factReview) {
					break
				}

				combinedReview := map[string]interface{}{
					"gap_review":  gapReview,
					"fact_review": factReview,
				}

				var revisionPrompt string
				if shouldReset(gapReview) || shouldReset(factReview) {
					logStep(fmt.Sprintf("Resetting investigator executor context: %s", combinedReview["reset_reason"]), "SYSTEM")
					o.InvestigatorExecutor.Reset(fmt.Sprintf("investigation-workstream-%d-%d", N+1, i))
					revisionPrompt = fmt.Sprintf(
						"WORKSTREAM:\n%s\nPREVIOUS FINDINGS:\n%s\nREVIEW FEEDBACK:\n%s\n\nRevise the investigation findings for this workstream.",
						MarshalJSON(wsMap), MarshalJSON(findings), MarshalJSON(combinedReview),
					)
				} else {
					revisionPrompt = fmt.Sprintf("REVISE INVESTIGATION FINDINGS based on feedback:\n%s", MarshalJSON(combinedReview))
				}
				findings = RunJSONAgent(
					o.InvestigatorExecutor,
					revisionPrompt,
					fmt.Sprintf("investigation-workstream-%d-%d", N+1, i+1),
					append(subdir, fmt.Sprintf("%d", o.DomainID)),
				).(map[string]interface{})
			}

			MarkdownDocumentGenerator(findings, fmt.Sprintf("investigation_workstream_%d", N+1), append(subdir, fmt.Sprintf("%d", o.DomainID)))
			completedWorkstreams[wsMap["id"].(string)] = findings
		}
		madeProgress = completedBefore != len(completedWorkstreams)
	}

	findingsList := []interface{}{}
	for _, v := range workstreams {
		if wm, ok := v.(map[string]interface{}); ok {
			id := strings.ToLower(strings.TrimSpace(wm["id"].(string)))
			findingsList = append(findingsList, completedWorkstreams[id])
		}
	}

	report := RunJSONAgent(
		o.SynthesisAgent,
		fmt.Sprintf("FINDINGS:\n%s", MarshalJSON(findingsList)),
		"investigation-synthesis",
		subdir,
	).(map[string]interface{})

	for i := 0; i < MAXPlanIters; i++ {
		consistencyReview := RunJSONAgent(
			o.SynthesisConsistencyReview,
			fmt.Sprintf("REPORT TO REVIEW:\n%s\nSOURCE FINDINGS:\n%s", MarshalJSON(report), MarshalJSON(findingsList)),
			fmt.Sprintf("investigation-consistency_review-final-%d", i),
			subdir,
		).(map[string]interface{})

		if reviewOk(consistencyReview) {
			break
		}

		revisionPrompt := fmt.Sprintf(
			"REVIEW FEEDBACK:\n%s\nFINDINGS:\n%s\nPREVIOUS REPORT:\n%s\n\nRebuild the investigation report from scratch using the findings and review feedback.",
			MarshalJSON(consistencyReview), MarshalJSON(findingsList), MarshalJSON(report),
		)
		report = RunJSONAgent(
			o.SynthesisAgent,
			revisionPrompt,
			fmt.Sprintf("investigation-synthesis-%d", i+1),
			subdir,
		).(map[string]interface{})
	}

	reportFilepath := MarkdownDocumentGenerator(
		report, "investigation_report_final", subdir,
	)

	reportContent, err := os.ReadFile(reportFilepath)
	if err != nil {
		panic(err)
	}
	return string(reportContent)
}

func (o *Orchestrator) ArchitectureDesignPhase(
	finalFeedback map[string]interface{},
	task string,
	invocationIDPrefix string,
	pmFilepath string,
) map[string]interface{} {
	var initialPrompt string
	if finalFeedback != nil && len(finalFeedback) > 0 {
		initialPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE ARCHITECTURE based on feedback post implementation:\n%s", pmFilepath, MarshalJSON(finalFeedback))
	} else {
		initialPrompt = fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s", task, pmFilepath)
	}

	extraPrompt := "\nKeep architecture focused on component boundaries, ownership, and interactions. Avoid naming concrete functions, methods, language constructs, or exact code statements unless they are architecturally significant."
	archResults := Nudge(
		100,
		o.Arch,
		initialPrompt+extraPrompt,
		fmt.Sprintf("%s-0", invocationIDPrefix),
		[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
		false,
		o.NextStepsCleanup,
	)
	arch, _ := archResults[len(archResults)-1].(map[string]interface{})

	for i := 0; i < MAXPlanIters; i++ {
		archReviewResults := Nudge(
			100,
			o.ArchReview,
			fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nATTEMPT: %d/%d\nARCHITECTURE TO REVIEW:\n%s",
				task, pmFilepath, i+1, MAXPlanIters, MarshalJSON(arch)),
			fmt.Sprintf("%s-review-%d", invocationIDPrefix, i),
			[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
			false,
			o.NextStepsCleanup,
		)
		archReview, _ := archReviewResults[len(archReviewResults)-1].(map[string]interface{})

		if reviewOk(archReview) {
			break
		}

		var revisionPrompt string
		if shouldReset(archReview) {
			logStep(fmt.Sprintf("Resetting architect context: %s", archReview["reset_reason"]), "SYSTEM")
			o.Arch.Reset(fmt.Sprintf("%s-%d", invocationIDPrefix, i))
			revisionPrompt = fmt.Sprintf(
				"TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nPREVIOUS ARCHITECTURE:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the architecture from scratch using the task and review feedback.",
				task, pmFilepath, MarshalJSON(arch), MarshalJSON(archReview),
			)
		} else {
			revisionPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE ARCHITECTURE based on feedback:\n%s", pmFilepath, MarshalJSON(archReview))
		}
		archResults = Nudge(
			100,
			o.Arch,
			revisionPrompt+extraPrompt,
			fmt.Sprintf("%s-%d", invocationIDPrefix, i+1),
			[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
			false,
			o.NextStepsCleanup,
		)
		arch, _ = archResults[len(archResults)-1].(map[string]interface{})
	}

	if am, ok := arch["architecture"].(map[string]interface{}); ok {
		delete(am, "reviewer_notes")
	}

	return arch
}

func (o *Orchestrator) PlanCreationPhase(
	arch map[string]interface{},
	task string,
	invocationIDPrefix string,
	pmFilepath string,
) map[string]interface{} {
	extraPrompt := "\nPrefer concrete file-level changes, but avoid embedding exact code snippets unless the task is trivial and the code itself is the clearest representation of the change."
	planResults := Nudge(
		100,
		o.TechLead,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s%s",
			task, pmFilepath, MarshalJSON(arch), extraPrompt),
		fmt.Sprintf("%s-0", invocationIDPrefix),
		[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
		false,
		o.NextStepsCleanup,
	)
	plan, _ := planResults[len(planResults)-1].(map[string]interface{})

	for i := 0; i < MAXPlanIters; i++ {
		planReviewResults := Nudge(
			100,
			o.PlanReview,
			fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nATTEMPT: %d/%d\nAPPROVED ARCHITECTURE:\n%s\nPLAN TO REVIEW:\n%s",
				task, pmFilepath, i+1, MAXPlanIters, MarshalJSON(arch), MarshalJSON(plan)),
			fmt.Sprintf("%s-review-%d", invocationIDPrefix, i),
			[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
			false,
			o.NextStepsCleanup,
		)
		planReview, _ := planReviewResults[len(planReviewResults)-1].(map[string]interface{})

		if reviewOk(planReview) {
			break
		}

		var revisionPrompt string
		if shouldReset(planReview) {
			logStep(fmt.Sprintf("Resetting tech lead context: %s", planReview["reset_reason"]), "SYSTEM")
			o.TechLead.Reset(fmt.Sprintf("%s-%d", invocationIDPrefix, i))
			revisionPrompt = fmt.Sprintf(
				"TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s\nPREVIOUS PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRebuild the plan from scratch using the task, architecture, and review feedback.",
				task, pmFilepath, MarshalJSON(arch), MarshalJSON(plan), MarshalJSON(planReview),
			)
		} else {
			revisionPrompt = fmt.Sprintf("BROAD PRODUCT SPECIFICATION: %s\nREVISE PLAN based on feedback:\n%s", pmFilepath, MarshalJSON(planReview))
		}
		planResults = Nudge(
			100,
			o.TechLead,
			revisionPrompt+extraPrompt,
			fmt.Sprintf("%s-%d", invocationIDPrefix, i+1),
			[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
			false,
			o.NextStepsCleanup,
		)
		plan, _ = planResults[len(planResults)-1].(map[string]interface{})
	}

	if pm, ok := plan["plan"].(map[string]interface{}); ok {
		delete(pm, "reviewer_notes")
	}

	return plan
}

func (o *Orchestrator) CodeImplementationPhase(
	plan map[string]interface{},
	invocationIDPrefix string,
) string {
	config := &Configuration{
		InitialPromptContext: fmt.Sprintf(
			"APPROVED IMPLEMENTATION PLAN:\n%s\n\nImplement the approved plan exactly as written.\nThe plan has already been reviewed and approved.\nDo not question whether planned file creation or modification should occur.",
			MarshalJSON(plan),
		),
		CoderAgentRef:  o.Coder,
		ReviewAgentRef: o.CodeReview,
		MaxIterations:  MAXCodeIters,
		Plan:           plan,
		NSC:            o.NextStepsCleanup,
	}

	cf := &CodeExecutionFramework{}
	return cf.Execute(
		config,
		[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
		invocationIDPrefix,
		o.Watcher,
	)
}

func (o *Orchestrator) TechLeadReviewPhase(
	codeSummary string,
	arch map[string]interface{},
	plan map[string]interface{},
	task string,
	techLeadFinalReview map[string]interface{},
	invocationID string,
	pmFilepath string,
) map[string]interface{} {
	extraPrompt := ""
	if techLeadFinalReview != nil {
		extraPrompt = fmt.Sprintf("PREVIOUS FEEDBACK:\n%s\n", MarshalJSON(techLeadFinalReview))
	}

	results := Nudge(
		100,
		o.TechLeadFinal,
		fmt.Sprintf("TASK:\n%s\n\nBROAD PRODUCT SPECIFICATION: %s\nAPPROVED ARCHITECTURE:\n%s\nAPPROVED IMPLEMENTATION PLAN:\n%s\n%s<aggregate_implementation_summary>\n%s\n</aggregate_implementation_summary>\n",
			task, pmFilepath, MarshalJSON(arch), MarshalJSON(plan), extraPrompt, codeSummary),
		invocationID,
		[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
		false,
		o.NextStepsCleanup,
	)
	return results[len(results)-1].(map[string]interface{})
}

func (o *Orchestrator) RevisionLoops(
	techLeadReview map[string]interface{},
	plan map[string]interface{},
	invocationIDPrefix string,
	resetCoder bool,
) string {
	var initialPromptContext string
	if resetCoder {
		logStep(fmt.Sprintf("Resetting coder context: %s", techLeadReview["reset_reason"]), "SYSTEM")
		o.Coder.Reset(fmt.Sprintf("%s-start", invocationIDPrefix))
		initialPromptContext = fmt.Sprintf(
			"APPROVED IMPLEMENTATION PLAN:\n%s\nTECH LEAD FEEDBACK:\n%s\n\nRe-implement from a clean context using the plan and tech lead feedback.",
			MarshalJSON(plan), MarshalJSON(techLeadReview),
		)
	} else {
		initialPromptContext = fmt.Sprintf("TECH LEAD FEEDBACK TO ADDRESS:\n%s", MarshalJSON(techLeadReview))
	}

	config := &Configuration{
		InitialPromptContext: initialPromptContext,
		CoderAgentRef:        o.Coder,
		ReviewAgentRef:       o.CodeReview,
		MaxIterations:        MAXCodeIters,
		Plan:                 plan,
		NSC:                  o.NextStepsCleanup,
	}

	cf := &CodeExecutionFramework{}
	return cf.Execute(
		config,
		[]string{o.Subdir, fmt.Sprintf("%d", o.DomainID)},
		invocationIDPrefix,
		o.Watcher,
	)
}

func wrapText(text string) string {
	return fmt.Sprintf("<text>\n%s\n</text>", text)
}
