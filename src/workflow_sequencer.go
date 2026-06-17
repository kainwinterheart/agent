// =========================
// WORKFLOW SEQUENCER
// =========================
package main

import (
	"fmt"
	"strings"
)

// workflowSequencer implements the dependency_respecting_executor and
// workflow_sequencer capabilities. It coordinates domain execution with
// topological ordering, made-progress retry, and post-execution investigation.
type workflowSequencer struct {
	o *Orchestrator
}

func newWorkflowSequencer(o *Orchestrator) *workflowSequencer {
	return &workflowSequencer{o: o}
}

// Run validates the decomposition, executes all domains respecting dependencies,
// then runs post-execution investigation.
func (s *workflowSequencer) Run(pmFilepath string, decomposition map[string]interface{}) {
	domains, _ := decomposition["decomposition"].(map[string]interface{})["domains"].([]interface{})
	if len(domains) == 0 {
		fmt.Println("No domains to execute.")
		return
	}

	s.executeDomains(pmFilepath, decomposition)

	report := s.o.InvestigationWorkflow(strings.TrimSpace(`
INVESTIGATION OBJECTIVE:
Determine whether the resulting system state faithfully realizes the intent of the original request.

The task is not to verify the presence of artifacts.
The task is to identify semantic mismatches between requested intent and resulting system behavior/structure.

ORIGINAL REQUEST:
` + s.o.Task + `

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
* Treat all implementation decisions as hypotheses requiring justification.
`))
	fmt.Println(report)
}

func (s *workflowSequencer) executeDomains(pmFilepath string, decomposition map[string]interface{}) {
	domains, _ := decomposition["decomposition"].(map[string]interface{})["domains"].([]interface{})
	integrationOwnership, _ := decomposition["decomposition"].(map[string]interface{})["integration_ownership"].([]interface{})

	completedWorkstreams := make(map[string]bool)
	haveWork := true
	madeProgress := true

	for haveWork {
		haveWork = false
		completedBefore := len(completedWorkstreams)

		for domainIndex, domain := range domains {
			domainMap, _ := domain.(map[string]interface{})
			domainID := strings.ToLower(strings.TrimSpace(domainMap["id"].(string)))
			if completedWorkstreams[domainID] {
				continue
			}

			spec, archExtra := BuildArchitectInput(domainMap, integrationOwnership)
			if spec == "" {
				completedWorkstreams[domainID] = true
				continue
			}
			haveWork = true

			canRun := true
			upstreamDeps, _ := domainMap["upstream_dependencies"].([]interface{})
			for _, dep := range upstreamDeps {
				if depStr, ok := dep.(string); ok {
					if !completedWorkstreams[strings.ToLower(depStr)] {
						canRun = false
						break
					}
				}
			}
			if !canRun {
				if madeProgress {
					continue
				} else {
					madeProgress = true
				}
			}

			s.o.DomainID = domainIndex + 1
			sessionSuffix := fmt.Sprintf("d%d-start", s.o.DomainID)
			s.o.Arch.Reset(sessionSuffix)
			s.o.TechLead.Reset(sessionSuffix)
			s.o.Coder.Reset(sessionSuffix)
			finalFeedback := map[string]interface{}{}

			coderTask := RunJSONAgent(
				s.o.DesignCleanup,
				fmt.Sprintf("INPUT TEXT:\n%s", spec),
				fmt.Sprintf("d%d-design-cleanup", s.o.DomainID),
				[]string{s.o.Subdir, fmt.Sprintf("%d", s.o.DomainID)},
			).(map[string]interface{})

			wrappedTask := wrapText(spec + archExtra)
			wrappedCoderTask := wrapText(coderTask["text"].(string))

			for iteration := 0; iteration < MAXTopIterations; iteration++ {
				docSuffix := ""
				if iteration > 0 {
					docSuffix = fmt.Sprintf("%d", iteration+1)
				}
				logStep(fmt.Sprintf("Starting iteration %d/%d for domain %d/%d", iteration+1, MAXTopIterations, domainIndex+1, len(domains)), "ITERATION")

				arch := s.o.ArchitectureDesignPhase(finalFeedback, wrappedTask, fmt.Sprintf("d%d-arch-%d", s.o.DomainID, iteration), pmFilepath)
				MarkdownDocumentGenerator(arch, fmt.Sprintf("architecture_after_reviews%s", docSuffix), []string{s.o.Subdir, fmt.Sprintf("%d", s.o.DomainID)})

				plan := s.o.PlanCreationPhase(arch, wrappedCoderTask, fmt.Sprintf("d%d-plan-%d", s.o.DomainID, iteration), pmFilepath)
				MarkdownDocumentGenerator(plan, fmt.Sprintf("tech_plan_after_reviews%s", docSuffix), []string{s.o.Subdir, fmt.Sprintf("%d", s.o.DomainID)})

				codeSummary := s.o.CodeImplementationPhase(plan, fmt.Sprintf("d%d-impl-%d", s.o.DomainID, iteration))

				codeSummaries := []string{codeSummary}
				var techLeadFinalReview map[string]interface{}

				for tlIter := 0; tlIter < MAXTopIterations; tlIter++ {
					techLeadFinalReview = s.o.TechLeadReviewPhase(
						codeSummary, arch, plan, wrappedCoderTask,
						techLeadFinalReview,
						fmt.Sprintf("d%d-tl-review-%d-%d", s.o.DomainID, iteration, tlIter),
						pmFilepath,
					)

					if reviewOk(techLeadFinalReview) {
						break
					}

					logStep("Tech lead feedback received - revising implementation", "SYSTEM")
					codeSummary = s.o.RevisionLoops(
						techLeadFinalReview, plan,
						fmt.Sprintf("d%d-impl-revision-%d-%d", s.o.DomainID, iteration, tlIter),
						shouldReset(techLeadFinalReview),
					)
					codeSummaries = append(codeSummaries, codeSummary)
				}

				mergedSummaries := ""
				for i, cs := range codeSummaries {
					mergedSummaries += fmt.Sprintf("<summary%d>\n%s\n</summary%d>\n", i+1, cs, i+1)
				}
				mergedSummaries = strings.TrimSuffix(mergedSummaries, "\n")
				MarkdownDocumentGenerator(mergedSummaries, fmt.Sprintf("code_summary%s", docSuffix), []string{s.o.Subdir, fmt.Sprintf("%d", s.o.DomainID)})
				finalFeedback = RunJSONAgent(
					s.o.ArchFinal,
					fmt.Sprintf("TASK:\n%s\nARCHITECTURE:\n%s\nAPPROVED IMPLEMENTATION PLAN:\n%s\n<aggregate_implementation_summary>\n%s\n</aggregate_implementation_summary>\n",
						wrappedTask,
						MarshalJSON(arch),
						MarshalJSON(plan),
						mergedSummaries,
					),
					fmt.Sprintf("d%d-arch-final-review-%d", s.o.DomainID, iteration),
					[]string{s.o.Subdir, fmt.Sprintf("%d", s.o.DomainID)},
				).(map[string]interface{})

				if reviewOk(finalFeedback) {
					break
				}
			}
			completedWorkstreams[domainID] = true
		}
		madeProgress = completedBefore != len(completedWorkstreams)
	}
}
