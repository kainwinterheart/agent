package main

import (
	"agent-go/pkg/loader"
	"fmt"
	"math/rand"
	"os"
	"time"
)

var resetHook func(agentName, sessionSuffix string)

type Agent[T any] struct {
	Name                string
	Subdir              string
	Session             string
	SessionSuffix       string
	RolePrompt          string
	Schema              map[string]interface{}
	Ephemeral           bool
	Timeout             string
	LastCorrectResponse *T
	ResumePrompt        string
}

func NewAgent[T any](
	name string,
	rolePrompt string,
	schema map[string]interface{},
	subdir string,
	opts ...AgentOption[T],
) *Agent[T] {
	a := &Agent[T]{
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

type AgentOption[T any] func(*Agent[T])

func WithEphemeral[T any](v bool) AgentOption[T] {
	return func(a *Agent[T]) { a.Ephemeral = v }
}

func WithTimeout[T any](t string) AgentOption[T] {
	return func(a *Agent[T]) { a.Timeout = t }
}

func WithResume[T any](r string) AgentOption[T] {
	return func(a *Agent[T]) { a.ResumePrompt = r }
}

func (a *Agent[T]) SessionKey() string {
	if a.Ephemeral && a.SessionSuffix == "" {
		a.resetInternal()
	}
	key := a.Name
	if a.SessionSuffix != "" {
		key += "@" + a.SessionSuffix
	}
	return key
}

func (a *Agent[T]) Run(inputText string) string {
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
				nextPrompt += fmt.Sprintf("PREVIOUS RESPONSE: %s\n\n\n", MarshalJSON(*a.LastCorrectResponse))
			} else {
				nextPrompt += "\n\n\n"
			}
			nextPrompt += loader.Followup
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
				loader.SchemaToExample(a.Schema),
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

func (a *Agent[T]) Reset(sessionSuffix ...string) {
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

func (a *Agent[T]) resetInternal(sessionSuffix ...string) {
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

func SaveSessionId(agentName string, sessionId string, subdir string) {
	filepath := fmt.Sprintf("%s/.state/.sessions/%s.session", subdir, agentName)
	AtomicWrite(filepath, sessionId)
}
