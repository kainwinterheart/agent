package main

import (
	"fmt"
	"sort"
	"strings"
)

// The static workflow definitions are the single source of truth for the
// system's wiring: which agents exist, what documents they consume and
// produce, and how they are grouped into subworkflows. The runtime agents and
// subworkflows are built from these definitions (initSubworkflows), the
// dumper renders them to the session trace (StaticDefinitionsDocument), and
// the tests verify the resulting input/output graph (static_defs_test.go).

// AgentDefinition is the static definition of one agent.
type AgentDefinition struct {
	Name    string // unique agent name; also the artifact type it produces
	Timeout string
	// Inputs lists the document types (agent names) this role is expected to
	// read from the artifact index. Documented, not enforced: agents receive
	// the index path and select the documents their role and the current
	// task require. For reviewer agents this includes the document under
	// review (their subworkflow's producer type).
	Inputs []string
	// Outputs lists the artifact types this agent produces. Content agents
	// produce exactly one document type: their own name. Decision agents
	// produce validated JSON decisions consumed by the orchestrator.
	Outputs []string
	// OutputTerminal marks outputs that are terminal deliverables or
	// control-plane inputs (read by the workflow driver, the loop decider,
	// the orchestrator, or a human) rather than by other agents.
	OutputTerminal bool
	// Decision marks decision agents (strictly validated JSON responses with a
	// schema enforced at generation time) as opposed to content agents
	// (Markdown documents at pregenerated paths).
	Decision bool
}

// StepDefinition is one agent position inside a subworkflow.
type StepDefinition struct {
	Agent        string // agent name
	ArtifactKind string // "what this file is" description of the produced document
	Reviewer     bool   // reviewer steps feed the loop decider
}

// SubworkflowDefinition is the static definition of one subworkflow: a short,
// independent sequence of closely tied agents.
type SubworkflowDefinition struct {
	ID          string
	Name        string
	Description string // what it does, shown to the driver
	Produces    string // what documents it produces, shown to the driver
	Steps       []StepDefinition
}

// AgentDefinitions is the complete static roster of agents.
var AgentDefinitions = []AgentDefinition{
	{Name: "product_manager", Timeout: "30m", Outputs: []string{"product_manager"}},
	{Name: "pm_review", Timeout: "30m", Inputs: []string{"product_manager"}, Outputs: []string{"pm_review"}, OutputTerminal: true},

	{Name: "investigation_classifier", Timeout: "10m", Outputs: []string{"investigation_classifier"}, OutputTerminal: true},

	{Name: "system_decomposition", Timeout: "40m", Inputs: []string{"product_manager"}, Outputs: []string{"system_decomposition"}},
	{Name: "system_decomposition_review", Timeout: "30m", Inputs: []string{"product_manager", "system_decomposition"}, Outputs: []string{"system_decomposition_review"}, OutputTerminal: true},

	{Name: "arch", Timeout: "40m", Inputs: []string{"product_manager", "system_decomposition"}, Outputs: []string{"arch"}},
	{Name: "arch_review", Timeout: "30m", Inputs: []string{"product_manager", "system_decomposition", "arch"}, Outputs: []string{"arch_review"}, OutputTerminal: true},

	{Name: "plan", Timeout: "60m", Inputs: []string{"product_manager", "arch", "system_decomposition"}, Outputs: []string{"plan"}},
	{Name: "plan_review", Timeout: "30m", Inputs: []string{"product_manager", "arch", "system_decomposition", "plan"}, Outputs: []string{"plan_review"}, OutputTerminal: true},

	{Name: "coder", Timeout: "180m", Inputs: []string{"plan"}, Outputs: []string{"coder"}},
	{Name: "code_review", Timeout: "60m", Inputs: []string{"plan", "coder"}, Outputs: []string{"code_review"}, OutputTerminal: true},

	{Name: "tech_lead_final", Timeout: "60m", Inputs: []string{"product_manager", "system_decomposition", "arch", "plan", "coder"}, Outputs: []string{"tech_lead_final"}, OutputTerminal: true},
	{Name: "arch_final", Timeout: "60m", Inputs: []string{"system_decomposition", "arch", "plan", "coder"}, Outputs: []string{"arch_final"}, OutputTerminal: true},

	{Name: "investigator_planner", Timeout: "90m", Inputs: []string{"product_manager"}, Outputs: []string{"investigator_planner"}},
	{Name: "investigation_plan_quality_review", Timeout: "30m", Inputs: []string{"product_manager", "investigator_planner"}, Outputs: []string{"investigation_plan_quality_review"}, OutputTerminal: true},
	{Name: "structure_review", Timeout: "30m", Inputs: []string{"product_manager", "investigator_planner"}, Outputs: []string{"structure_review"}, OutputTerminal: true},

	{Name: "investigator_executor", Timeout: "180m", Inputs: []string{"investigator_planner", "investigator_executor"}, Outputs: []string{"investigator_executor"}},
	{Name: "fact_checking_review", Timeout: "60m", Inputs: []string{"investigator_planner", "investigator_executor"}, Outputs: []string{"fact_checking_review"}, OutputTerminal: true},
	{Name: "gap_analysis_review", Timeout: "60m", Inputs: []string{"investigator_planner", "investigator_executor"}, Outputs: []string{"gap_analysis_review"}, OutputTerminal: true},

	{Name: "synthesis", Timeout: "90m", Inputs: []string{"investigator_executor"}, Outputs: []string{"synthesis"}, OutputTerminal: true},
	{Name: "synthesis_consistency_review", Timeout: "60m", Inputs: []string{"investigator_executor", "synthesis"}, Outputs: []string{"synthesis_consistency_review"}, OutputTerminal: true},

	// Decision agents: no Markdown artifacts; strictly validated JSON
	// responses consumed by the orchestrator's control flow.
	{Name: "workflow_driver", Timeout: "10m", Outputs: []string{"driver_decision"}, OutputTerminal: true, Decision: true},
	{Name: "loop_decider", Timeout: "10m", Outputs: []string{"loop_decision"}, OutputTerminal: true, Decision: true},
}

// SubworkflowDefinitions is the complete static registry of subworkflows.
var SubworkflowDefinitions = []SubworkflowDefinition{
	{
		ID:          "spec",
		Name:        "Refine task specification",
		Description: "Refines the raw task into an engineering-ready task specification, then reviews it.",
		Produces:    "task specification document + specification review verdict",
		Steps: []StepDefinition{
			{Agent: "product_manager", ArtifactKind: "Engineering-ready task specification refined from the original request"},
			{Agent: "pm_review", ArtifactKind: "Review of the task specification against the original request", Reviewer: true},
		},
	},
	{
		ID:          "classify",
		Name:        "Classify task",
		Description: "Classifies the task as investigation or engineering, with reasoning.",
		Produces:    "classification document (type + reasoning)",
		Steps: []StepDefinition{
			{Agent: "investigation_classifier", ArtifactKind: "Classification of the task as investigation or engineering, with reasoning"},
		},
	},
	{
		ID:          "decompose",
		Name:        "Decompose system",
		Description: "Splits the engineering task into independent domains with explicit dependencies and integration ownership, then reviews the decomposition.",
		Produces:    "system decomposition document + decomposition review verdict",
		Steps: []StepDefinition{
			{Agent: "system_decomposition", ArtifactKind: "System decomposition into bounded domains with dependencies and integration ownership"},
			{Agent: "system_decomposition_review", ArtifactKind: "Review of the system decomposition", Reviewer: true},
		},
	},
	{
		ID:          "architect",
		Name:        "Design architecture",
		Description: "Designs the architecture for the scope named in the task (a domain or the whole system), then reviews it.",
		Produces:    "architecture document + architecture review verdict",
		Steps: []StepDefinition{
			{Agent: "arch", ArtifactKind: "Architecture design for the scope named in the driver task"},
			{Agent: "arch_review", ArtifactKind: "Review of the architecture design", Reviewer: true},
		},
	},
	{
		ID:          "plan",
		Name:        "Create implementation plan",
		Description: "Turns the approved architecture into a concrete ordered implementation plan for the scope named in the task, then reviews the plan.",
		Produces:    "implementation plan document + plan review verdict",
		Steps: []StepDefinition{
			{Agent: "plan", ArtifactKind: "Concrete ordered implementation plan for the scope named in the driver task"},
			{Agent: "plan_review", ArtifactKind: "Review of the implementation plan", Reviewer: true},
		},
	},
	{
		ID:          "implement",
		Name:        "Implement and review code",
		Description: "Implements the approved plan in the repository, then reviews the actual changes. Automated disk-change detection is included in the review input.",
		Produces:    "implementation report + code review verdict",
		Steps: []StepDefinition{
			{Agent: "coder", ArtifactKind: "Implementation report: verified changes made to the repository"},
			{Agent: "code_review", ArtifactKind: "Review of the latest code changes against the plan and reports", Reviewer: true},
		},
	},
	{
		ID:          "tech_lead_final",
		Name:        "Tech lead final review",
		Description: "Final integration review of the implementation against the specification, architecture, and plan.",
		Produces:    "tech lead final review document (verdict + assessment)",
		Steps: []StepDefinition{
			{Agent: "tech_lead_final", ArtifactKind: "Final tech lead review of the implementation against spec, architecture, and plan"},
		},
	},
	{
		ID:          "arch_final",
		Name:        "Architecture final review",
		Description: "Final architecture review of the implemented system against the approved architecture.",
		Produces:    "architecture final review document (verdict + assessment)",
		Steps: []StepDefinition{
			{Agent: "arch_final", ArtifactKind: "Final architecture review of the implemented system"},
		},
	},
	{
		ID:          "investigate_plan",
		Name:        "Plan investigation",
		Description: "Plans the investigation as bounded workstreams with declared sources, hypotheses, and dependencies, then reviews plan quality and structure.",
		Produces:    "investigation plan + quality and structural review verdicts",
		Steps: []StepDefinition{
			{Agent: "investigator_planner", ArtifactKind: "Investigation plan with bounded workstreams"},
			{Agent: "investigation_plan_quality_review", ArtifactKind: "Quality review of the investigation plan", Reviewer: true},
			{Agent: "structure_review", ArtifactKind: "Structural review of the investigation plan", Reviewer: true},
		},
	},
	{
		ID:          "investigate",
		Name:        "Run investigation workstream",
		Description: "Executes one investigation workstream (name it in the task), then fact-checks and gap-analyzes the findings.",
		Produces:    "investigation findings + fact-checking and gap-analysis verdicts",
		Steps: []StepDefinition{
			{Agent: "investigator_executor", ArtifactKind: "Grounded investigation findings for the workstream named in the driver task"},
			{Agent: "fact_checking_review", ArtifactKind: "Fact-checking review of the investigation findings", Reviewer: true},
			{Agent: "gap_analysis_review", ArtifactKind: "Gap-analysis review of the investigation findings", Reviewer: true},
		},
	},
	{
		ID:          "synthesize",
		Name:        "Synthesize final report",
		Description: "Writes the final investigation report from all findings, then audits it for internal consistency.",
		Produces:    "final investigation report + consistency review verdict",
		Steps: []StepDefinition{
			{Agent: "synthesis", ArtifactKind: "Final investigation report consolidating all findings"},
			{Agent: "synthesis_consistency_review", ArtifactKind: "Consistency review of the final investigation report", Reviewer: true},
		},
	},
}

// agentDefinition looks up an agent definition by name.
func agentDefinition(name string) *AgentDefinition {
	for i := range AgentDefinitions {
		if AgentDefinitions[i].Name == name {
			return &AgentDefinitions[i]
		}
	}
	return nil
}

// subworkflowOfAgent returns the id of the subworkflow that uses the named
// agent, or "" for decision agents (they are control plane, not steps).
func subworkflowOfAgent(name string) string {
	for i := range SubworkflowDefinitions {
		for j := range SubworkflowDefinitions[i].Steps {
			if SubworkflowDefinitions[i].Steps[j].Agent == name {
				return SubworkflowDefinitions[i].ID
			}
		}
	}
	return ""
}

// StaticDefinitionsDocument renders the complete static wiring (agents,
// subworkflows, input/output graph) as a Markdown document. It is written to
// the session trace at run start and can be dumped standalone via the
// `static-defs` command.
func StaticDefinitionsDocument() string {
	var b strings.Builder
	b.WriteString("# Static Workflow Definitions\n\n")
	b.WriteString("Rendered from the static definitions in the code (src/definitions.go).\n")
	b.WriteString("Content agents produce Markdown documents at pregenerated paths; the document type is the producing agent's name. Decision agents respond in strictly validated JSON. All content agents receive the path to the artifact index and select the documents they read from it.\n\n")

	b.WriteString("## Agents\n\n")
	for i := range AgentDefinitions {
		d := &AgentDefinitions[i]
		kind := "content agent"
		if d.Decision {
			kind = "decision agent (strictly validated JSON)"
		}
		b.WriteString("### " + d.Name + " (" + kind + ")\n\n")
		if sw := subworkflowOfAgent(d.Name); sw != "" {
			b.WriteString("- subworkflow: " + sw + "\n")
		} else {
			b.WriteString("- role: control plane (not a subworkflow step)\n")
		}
		b.WriteString("- role prompt: pkg/loader/prompts/" + d.Name + ".txt\n")
		b.WriteString("- timeout: " + d.Timeout + "\n")
		if len(d.Inputs) > 0 {
			b.WriteString("- expected inputs (document types this role reads from the artifact index): " + strings.Join(d.Inputs, ", ") + "\n")
		} else {
			b.WriteString("- expected inputs: none (task documents only)\n")
		}
		terminal := ""
		if d.OutputTerminal {
			terminal = " [terminal: consumed by the driver / loop decider / orchestrator / human, not by other agents]"
		}
		b.WriteString("- outputs: " + strings.Join(d.Outputs, ", ") + terminal + "\n")
		if d.Name == "investigator_executor" {
			b.WriteString("- note: reads its own type from the index to receive the previous workstream's findings\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Subworkflows\n\n")
	for i := range SubworkflowDefinitions {
		sw := &SubworkflowDefinitions[i]
		b.WriteString("### " + sw.ID + " — " + sw.Name + "\n\n")
		b.WriteString("- description: " + sw.Description + "\n")
		b.WriteString("- produces: " + sw.Produces + "\n")
		b.WriteString("- steps:\n")
		for j, st := range sw.Steps {
			role := "producer"
			if st.Reviewer {
				role = "reviewer"
			}
			b.WriteString(fmt.Sprintf("  %d. %s (%s): %s\n", j+1, st.Agent, role, st.ArtifactKind))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Wiring Graph\n\n")
	b.WriteString("Each line lists the agents expected to read documents of the named type from the artifact index.\n\n")
	types := map[string][]string{}
	for i := range AgentDefinitions {
		d := &AgentDefinitions[i]
		for _, in := range d.Inputs {
			types[in] = append(types[in], d.Name)
		}
	}
	keys := make([]string, 0, len(types))
	for k := range types {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		consumers := types[k]
		sort.Strings(consumers)
		b.WriteString("- " + k + " -> " + strings.Join(consumers, ", ") + "\n")
	}
	b.WriteString("\nTerminal types (no agent readers by design): ")
	var terminals []string
	for i := range AgentDefinitions {
		d := &AgentDefinitions[i]
		if d.OutputTerminal {
			terminals = append(terminals, d.Name)
		}
	}
	sort.Strings(terminals)
	b.WriteString(strings.Join(terminals, ", "))
	b.WriteString("\n")
	return b.String()
}
