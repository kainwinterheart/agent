package main

import (
	"fmt"
)

// Agent is a single LLM role. Agents are one-shot by default: every
// invocation starts a fresh session, because all durable context flows
// through artifact files and validated decisions rather than through
// conversation state. The workflow driver and the loop decider are the
// exceptions: their invocations are multi-turn conversations whose session
// id is acquired from the runtime and kept solely in memory for the
// duration of one turn (see agentTurn in multiturn.go).
//
// Content agents (Schema == nil) deliver a Markdown document at their
// pregenerated output path. Decision agents (Schema != nil) respond with a
// strictly validated JSON document; the schema is also passed to the runtime
// via --output-schema.
type Agent struct {
	Name       string
	RolePrompt string
	Schema     map[string]any
	// Inputs lists the document types (agent names) this role is expected to
	// read from the artifact index to do its job (static definition).
	// Documented, not enforced: the runtime hands the agent the index path
	// and the agent selects its own input documents.
	Inputs []string
	// Outputs lists the artifact types this agent produces (static
	// definition).
	Outputs []string
	// OutputTerminal marks outputs consumed by the control plane (driver,
	// loop decider, orchestrator) or a human rather than by other agents.
	OutputTerminal bool
}

func NewAgent(name string, rolePrompt string) *Agent {
	return &Agent{
		Name:       name,
		RolePrompt: fmt.Sprintf("<role>\n\n%s\n\n</role>", rolePrompt),
	}
}

// WithSchema marks the agent as a decision agent bound to a JSON schema.
func (a *Agent) WithSchema(schema map[string]any) *Agent {
	a.Schema = schema
	return a
}

// Run invokes the LLM runtime once in a fresh session. For content agents
// the deliverable is the file written to the pregenerated output path; for
// decision agents it is the JSON response in stdout. Stateful
// conversations (the driver's multi-turn invocations) call RunCodex
// directly with the session id they keep in memory.
func (a *Agent) Run(prompt string, context *Context) (string, error) {
	stdout, _, err := RunCodex(a.Name, prompt, a.Schema, "", context)
	if err != nil {
		return "", err
	}
	return stdout, nil
}
