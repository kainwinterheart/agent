package main

import (
	"fmt"
)

// agentTurn is the in-memory conversation state of one multi-turn agent
// invocation (currently used by the workflow driver and the loop decider).
// The read-inputs call (phase 1) acquires a session id from the runtime;
// every later call of the turn (analyze, respond, and all retries) resumes
// that session. The id is never persisted to disk - a process restart
// simply starts the next turn from a clean session, and a runtime that
// reports no session id degrades the turn to self-contained one-shot calls
// (the pre-multi-turn behaviour).
type agentTurn struct {
	o           *Orchestrator
	agent       *Agent
	context     *Context
	stateBundle string // full self-contained prompt: role + task + state
	sessionID   string
}

func newAgentTurn(o *Orchestrator, agent *Agent, context *Context, stateBundle string) *agentTurn {
	return &agentTurn{o: o, agent: agent, context: context, stateBundle: stateBundle}
}

// readInputs is the first call of a multi-turn invocation: a model call in
// a fresh session, starting from clean context, whose only job is to read
// all the inputs of the turn. The session id the runtime reports is kept in
// memory (t.sessionID) and reused by every later call of the turn. A failed
// read is retried in a clean session: the conversation of a dead call is
// worthless context.
func (t *agentTurn) readInputs(readBlock string) {
	prompt := t.stateBundle + "\n\n" + readBlock
	attempt := 0
	for {
		attempt++
		t.context.Tracer.trace("agent_read_inputs", map[string]any{
			"agent":   t.agent.Name,
			"attempt": attempt,
			"prompt":  prompt,
		})
		_, sessionID, err := RunCodex(t.agent.Name, prompt, t.agent.Timeout, nil, "", t.context)
		if err == nil {
			t.sessionID = sessionID
			if sessionID == "" {
				logStep(fmt.Sprintf("%s: runtime reported no session id; the turn degrades to self-contained one-shot calls", t.agent.Name), "AGENT")
			}
			return
		}
		t.sessionID = ""
		logStep(fmt.Sprintf("%s read-inputs attempt %d failed: %s", t.agent.Name, attempt, err), "AGENT")
		t.context.Tracer.trace("agent_read_retry", map[string]any{
			"agent":   t.agent.Name,
			"attempt": attempt,
			"problem": err.Error(),
		})
	}
}

// invoke performs one model call of the turn. When the turn holds a session
// id it resumes that session and sends only the phase instruction - the
// role prompt, the state bundle and everything read in the first call are
// already in the conversation. Without a session id the call degrades to a
// self-contained one-shot: the full state bundle is prepended.
func (t *agentTurn) invoke(phasePrompt string, schema map[string]any) (string, error) {
	prompt := phasePrompt
	if t.sessionID == "" {
		prompt = t.stateBundle + "\n\n" + phasePrompt
	}
	stdout, sessionID, err := RunCodex(t.agent.Name, prompt, t.agent.Timeout, schema, t.sessionID, t.context)
	if sessionID != "" {
		t.sessionID = sessionID
	}
	if err != nil {
		// The session is dead or unusable: drop it so the next retry of
		// this turn degrades to self-contained one-shot calls.
		t.sessionID = ""
	}
	return stdout, err
}

// analyzeAndSave is the second call of a multi-turn invocation: resuming
// the acquired session, the agent analyzes the data it read with the
// intention of producing its JSON response and saves its analysis and
// reasoning to a pregenerated file. The orchestrator verifies the file on
// disk (exists, non-empty - the same predicate as content-agent
// deliverables) and retries within the same session until it passes. The
// retry loop is unbounded.
func (t *agentTurn) analyzeAndSave(analysisPath, analyzeBlock string) {
	ensureParentDir(analysisPath)
	cur := analyzeBlock
	attempt := 0
	for {
		attempt++
		t.context.Tracer.trace("agent_analyze", map[string]any{
			"agent":         t.agent.Name,
			"attempt":       attempt,
			"prompt":        cur,
			"analysis_path": analysisPath,
		})

		var problem string
		if _, err := t.invoke(cur, nil); err != nil {
			problem = "the agent run itself failed: " + err.Error()
		} else if verr := validateOutputFile(analysisPath); verr != nil {
			problem = verr.Error()
		} else {
			logStep(fmt.Sprintf("%s: analysis saved to %s", t.agent.Name, analysisPath), "AGENT")
			return
		}

		logStep(fmt.Sprintf("%s analysis attempt %d incomplete: %s", t.agent.Name, attempt, problem), "AGENT")
		t.context.Tracer.trace("agent_analyze_retry", map[string]any{
			"agent":   t.agent.Name,
			"attempt": attempt,
			"problem": problem,
		})
		cur = analyzeBlock + analysisFeedbackBlock(problem)
	}
}

// agentDecide performs a JSON-decision call of a multi-turn invocation -
// the respond phase, or (for the driver) the mandatory finish
// reassessment - resuming the acquired session, and retries within the
// session until the response passes strict validation. Retries send only
// the validation feedback: the role, the state bundle, the saved analysis
// and the previous attempt are already in the conversation. The retry loop
// is unbounded.
func agentDecide[T any](t *agentTurn, phase, phasePrompt string, schema map[string]any, example string, decode func(string) (T, error)) T {
	cur := phasePrompt
	attempt := 0
	for {
		attempt++
		t.context.Tracer.trace("decision_invoke", map[string]any{
			"agent":   t.agent.Name,
			"phase":   phase,
			"attempt": attempt,
			"prompt":  cur,
		})

		var out T
		var problem string
		stdout, err := t.invoke(cur, schema)
		if err != nil {
			problem = "the agent run itself failed: " + err.Error()
		} else {
			j, jerr := ExtractJSON(stdout)
			if jerr != nil {
				problem = "the response contains no JSON object: " + jerr.Error()
			} else {
				var derr error
				out, derr = decode(j)
				if derr != nil {
					problem = "the response failed strict JSON validation: " + derr.Error()
				} else {
					t.context.Tracer.trace("decision_result", map[string]any{
						"agent":    t.agent.Name,
						"phase":    phase,
						"attempt":  attempt,
						"response": prettyJSON(out),
					})
					logStep(fmt.Sprintf("%s %s: valid decision after %d attempt(s)", t.agent.Name, phase, attempt), "DECISION")
					return out
				}
			}
		}

		logStep(fmt.Sprintf("%s %s attempt %d invalid: %s", t.agent.Name, phase, attempt, problem), "DECISION")
		t.context.Tracer.trace("decision_retry", map[string]any{
			"agent":   t.agent.Name,
			"phase":   phase,
			"attempt": attempt,
			"problem": problem,
		})
		cur = jsonFeedbackBlock(problem, prettyJSON(schema), example)
	}
}

// analysisFeedbackBlock is the retry feedback for an incomplete analyze
// phase (the file was not written or is empty).
func analysisFeedbackBlock(problem string) string {
	return fmt.Sprintf(`

<feedback>
YOUR PREVIOUS ATTEMPT WAS INCOMPLETE.
<error>
%s
</error>

Write your complete analysis and reasoning to the exact path given under your PHASE 2 OF 3: ANALYZE AND SAVE instruction. The file must exist and be non-empty when you finish.
</feedback>
`, problem)
}
