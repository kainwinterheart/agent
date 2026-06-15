// =========================
// AGENT
// =========================
package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

// resetHook allows overriding Agent.Reset for testing.
var resetHook func(agentName, sessionSuffix string)

// Agent represents a single AI agent with session management.
type Agent struct {
	Name                string
	Subdir              string
	Session             string
	SessionSuffix       string
	RolePrompt          string
	Schema              map[string]interface{}
	Ephemeral           bool
	Timeout             string
	LastCorrectResponse map[string]interface{}
	ResumePrompt        string
}

// NewAgent creates a new Agent with the given parameters.
func NewAgent(
	name string,
	rolePrompt string,
	schema map[string]interface{},
	subdir string,
	opts ...AgentOption,
) *Agent {
	a := &Agent{
		Name:         name,
		Subdir:       subdir,
		RolePrompt:   fmt.Sprintf("<role>\n\n%s\n\n</role>", rolePrompt),
		Schema:       schema,
		Ephemeral:    false,
		Timeout:      "",
		ResumePrompt: "YOU WERE INTERRUPTED, CONTINUE.\n\n",
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// AgentOption is a functional option for configuring an Agent.
type AgentOption func(*Agent)

// WithEphemeral sets the agent as ephemeral.
func WithEphemeral(v bool) AgentOption {
	return func(a *Agent) { a.Ephemeral = v }
}

// WithTimeout sets the agent timeout.
func WithTimeout(t string) AgentOption {
	return func(a *Agent) { a.Timeout = t }
}

// WithResume sets the resume prompt.
func WithResume(r string) AgentOption {
	return func(a *Agent) { a.ResumePrompt = r }
}

// SessionKey returns the session key, potentially resetting if ephemeral.
func (a *Agent) SessionKey() string {
	if a.Ephemeral && a.SessionSuffix == "" {
		a.resetInternal()
	}
	key := a.Name
	if a.SessionSuffix != "" {
		key += "@" + a.SessionSuffix
	}
	return key
}

// Run executes the agent with the given input text and returns the output string.
func (a *Agent) Run(inputText string) string {
	if !a.Ephemeral && a.Session == "" {
		sess := LoadSessionId(a.SessionKey(), a.Subdir)
		if sess != "" {
			a.Session = sess
		}
	}
	prompt := fmt.Sprintf("%s\n\nReturn ONLY valid JSON.", inputText)
	if a.Session == "" {
		prompt = a.RolePrompt + "\n\n" + prompt
	}
	lastError := ""
	prevSession := a.Session
	for {
		nextPrompt := prompt + lastError
		if a.Session != "" || a.LastCorrectResponse != nil {
			if a.LastCorrectResponse != nil {
				nextPrompt += "\n\n"
				nextPrompt += fmt.Sprintf("PREVIOUS RESPONSE: %s\n\n\n", MarshalJSON(a.LastCorrectResponse))
			} else {
				nextPrompt += "\n\n\n"
			}
			nextPrompt += FOLLOWUP
			nextPrompt += "\n"
		}
		out, newSession, err := RunCodex(
			a.Name,
			a.Session,
			nextPrompt,
			a.Schema,
			a.Timeout,
		)
		if err != nil {
			logStep(fmt.Sprintf("Failed to run %s, retrying...", a.Name), a.Name)
			lastError = fmt.Sprintf("\n%s\n\n<feedback>\nPrevious attempt to read your new response FAILED:\n<error>\n%v\n</error>\n\nOutput MUST be valid JSON only:\n%s\n</feedback>\n\n",
				a.ResumePrompt,
				err,
				SchemaToExample(a.Schema),
			)
			continue
		}
		a.Session = newSession
		if prevSession != a.Session && !a.Ephemeral {
			SaveSessionId(a.SessionKey(), a.Session, a.Subdir)
		}
		return out
	}
}

// Reset resets the agent's session state.
func (a *Agent) Reset(sessionSuffix ...string) {
	var suffix string
	if len(sessionSuffix) > 0 {
		suffix = sessionSuffix[0]
	}
	if resetHook != nil {
		resetHook(a.Name, suffix)
	}
	trace("reset_agent", map[string]interface{}{
		"session_suffix": func() interface{} {
			if suffix == "" {
				return nil
			}
			return suffix
		}(),
		"agent": a.Name,
	})
	a.resetInternal(sessionSuffix...)
}

func (a *Agent) resetInternal(sessionSuffix ...string) {
	var suffix string
	if len(sessionSuffix) > 0 {
		suffix = sessionSuffix[0]
	}
	if a.Ephemeral {
		if suffix != "" {
			panic("Session suffix can't be specified for ephemeral agents")
		}
		suffix = fmt.Sprintf("%d-%d", time.Now().UnixMilli(), rand.Int63())
	} else {
		if suffix == "" || suffix == a.SessionSuffix {
			panic("Session suffix must be changed upon non-ephemeral agent reset")
		}
	}
	a.Session = ""
	a.SessionSuffix = suffix
	a.LastCorrectResponse = nil
}

// LoadSessionId loads the session ID from the agent's session file.
func LoadSessionId(agentName string, subdir string) string {
	filepath := fmt.Sprintf("%s/.state/.sessions/%s.session", subdir, agentName)
	if _, err := os.Stat(filepath); err == nil {
		logStep(fmt.Sprintf("Reading last session ID: %s", filepath), agentName)
		data, err := os.ReadFile(filepath)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// SaveSessionId persists the session ID to the agent's session file.
func SaveSessionId(agentName string, sessionId string, subdir string) {
	filepath := fmt.Sprintf("%s/.state/.sessions/%s.session", subdir, agentName)
	AtomicWrite(filepath, sessionId)
}
