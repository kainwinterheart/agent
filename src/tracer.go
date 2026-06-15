// =========================
// TRACER
// =========================
package main

import (
	"os"
)

// trace writes a single trace event to the file pointed to by AGENT_TRACE_FILE.
// Each event is a JSON line with deterministic key ordering.
func trace(action string, details map[string]interface{}) {
	dest := os.Getenv("AGENT_TRACE_FILE")
	if dest == "" {
		return
	}
	f, err := os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	event := map[string]interface{}{
		"action":  action,
		"details": details,
	}
	data := MarshalJSON(event)
	f.Write([]byte(data))
	f.Write([]byte("\n"))
}
