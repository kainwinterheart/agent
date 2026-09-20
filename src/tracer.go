package main

import (
	"encoding/json"
	"io"
)

// TraceEvent is one structured log line describing an orchestrator decision point.
type TraceEvent struct {
	Action  string         `json:"action"`
	Details map[string]any `json:"details,omitempty"`
}

// Tracer collects the events of one step execution.
type Tracer struct {
	Events []TraceEvent
}

func (t *Tracer) trace(action string, details map[string]any) {
	t.Events = append(t.Events, TraceEvent{Action: action, Details: details})
}

// writeEvents emits all collected events as JSON lines to w.
func writeEvents(w io.Writer, events []TraceEvent) {
	for _, e := range events {
		line, err := json.Marshal(e)
		if err != nil {
			continue
		}
		w.Write(line)
		w.Write([]byte("\n"))
	}
}
