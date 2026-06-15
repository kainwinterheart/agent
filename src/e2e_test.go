// =========================
// E2E TEST (data-driven)
// =========================
package main

import (
	jsonv2text "encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"testing"
)

type DataAction struct {
	Action  string           `json:"action"`
	Details jsonv2text.Value `json:"details"`
}

type ActionDetails struct {
	Action        string
	Agent         string
	InvocationID  string
	Prompt        string
	Schema        map[string]interface{}
	Timeout       string
	Stdout        string
	SessionSuffix interface{}
	StageName     string
	Content       string
	Text          string
	Changes       map[string]string
}

func parseDetails(raw jsonv2text.Value, action string) ActionDetails {
	var d ActionDetails
	d.Action = action
	switch action {
	case "user_input":
		var m map[string]interface{}
		jsonv2.Unmarshal(raw, &m)
		d.Text = m["text"].(string)
	case "prepare_to_run_agent":
		var m map[string]interface{}
		jsonv2.Unmarshal(raw, &m)
		d.InvocationID = m["invocation_id"].(string)
		d.Agent = m["agent"].(string)
		d.Prompt = m["prompt"].(string)
	case "run_codex":
		var m map[string]interface{}
		jsonv2.Unmarshal(raw, &m)
		d.Agent = m["agent_name"].(string)
		d.Prompt = m["prompt"].(string)
		d.Timeout = m["timeout"].(string)
		d.Stdout = m["stdout"].(string)
		if s, ok := m["schema"].(map[string]interface{}); ok {
			d.Schema = s
		}
	case "reset_agent":
		var m map[string]interface{}
		jsonv2.Unmarshal(raw, &m)
		d.Agent = m["agent"].(string)
		d.SessionSuffix = m["session_suffix"]
	case "write_markdown_doc":
		var m map[string]interface{}
		jsonv2.Unmarshal(raw, &m)
		d.StageName = m["stage_name"].(string)
		d.Content = m["content"].(string)
	case "watchman":
		var m map[string]interface{}
		jsonv2.Unmarshal(raw, &m)
		if changes, ok := m["changes"].(map[string]interface{}); ok {
			d.Changes = make(map[string]string)
			for k, v := range changes {
				d.Changes[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	return d
}

func loadDataActions(t *testing.T, filename string) []ActionDetails {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read test data file %s: %v", filename, err)
	}
	var actions []ActionDetails
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var da DataAction
		if err := jsonv2.Unmarshal([]byte(line), &da); err != nil {
			t.Fatalf("Failed to parse test data line: %q, err: %v", line, err)
		}
		actions = append(actions, parseDetails(da.Details, da.Action))
	}
	return actions
}

type e2eMockRunner struct {
	actions []ActionDetails
	idx     int
	t       *testing.T
}

func newMockRunner(t *testing.T, actions []ActionDetails, startIdx int) *e2eMockRunner {
	return &e2eMockRunner{actions: actions, idx: startIdx, t: t}
}

func (mr *e2eMockRunner) next(expectedAction string) ActionDetails {
	if mr.idx >= len(mr.actions) {
		mr.t.Fatalf("Unexpected call: expected %s but no more actions remain (idx=%d, total=%d)",
			expectedAction, mr.idx, len(mr.actions))
	}
	action := mr.actions[mr.idx]
	mr.idx++
	if action.Action != expectedAction {
		mr.t.Fatalf("Expected action %q at index %d (total=%d), got %q (agent=%q, invocation_id=%q, stage_name=%q)",
			expectedAction, mr.idx-1, len(mr.actions),
			action.Action, action.Agent, action.InvocationID, action.StageName)
	}
	return action
}

func (mr *e2eMockRunner) verifyAllConsumed() {
	if mr.idx < len(mr.actions) {
		mr.t.Fatalf("Not all actions were consumed: %d remaining (idx=%d, total=%d)",
			len(mr.actions)-mr.idx, mr.idx, len(mr.actions))
	}
}

func getTaskText(actions []ActionDetails) string {
	for _, a := range actions {
		if a.Action == "user_input" {
			return a.Text
		}
	}
	return ""
}

func normalizeErrorMessages(prompt string) string {
	re := regexp.MustCompile(`<error>\n(.*?)\n</error>`)
	return re.ReplaceAllStringFunc(prompt, func(match string) string {
		inner := strings.TrimPrefix(match, "<error>\n")
		inner = strings.TrimSuffix(inner, "\n</error>")
		// Normalize Go json.Unmarshal errors (version-dependent)
		if strings.Contains(inner, "invalid character") ||
			strings.Contains(inner, "Expecting ':' delimiter") ||
			(strings.Contains(inner, "line ") && strings.Contains(inner, "column ")) {
			return "<error>\nGO_JSON_UNMARSHAL_ERROR\n</error>"
		}
		// Normalize jsonschema validation errors
		if strings.Contains(inner, "Error within") ||
			strings.Contains(inner, "Additional properties") ||
			strings.Contains(inner, "doesn't validate with") {
			return "<error>\nSCHEMA_VALIDATION_ERROR\n</error>"
		}
		// Normalize ExtractJSON errors
		if strings.Contains(inner, "does not contain") && strings.Contains(inner, "JSON") {
			return "<error>\nEXTRACT_JSON_ERROR\n</error>"
		}
		return match
	})
}

func normalizePrompt(prompt string, agent string, actions []ActionDetails) string {
	result := normalizeJSONObjects(prompt)
	result = normalizeErrorMessages(result)
	// Normalize filepaths: replace any path ending with /document_stores/FILENAME.md
	result = regexp.MustCompile(`[^\n\r]+/document_stores/[^\n\r]+\.md`).ReplaceAllStringFunc(result, func(m string) string {
		idx := strings.Index(m, "/document_stores/")
		if idx < 0 {
			return m
		}
		filename := m[idx+len("/document_stores/"):]
		// Remove timestamp-like prefixes: "2026-06-12_09-36-28_", "product_manager_final_", etc.
		filename = regexp.MustCompile(`^[\d\-_+:]+[_-]?`).ReplaceAllString(filename, "")
		return "SUBDIR/document_stores/" + filename
	})
	return result
}

func expectedMarkdownFiles(actions []ActionDetails) map[string]string {
	result := make(map[string]string)
	for _, a := range actions {
		if a.Action == "write_markdown_doc" {
			stageNameClean := regexp.MustCompile(`[0-9]+$`).ReplaceAllString(a.StageName, "")
			result[stageNameClean] = a.Content
		}
	}
	return result
}

func TestE2E(t *testing.T) {
	actions := loadDataActions(t, "../test-data-1.json")

	userAction := actions[0]
	if userAction.Action != "user_input" {
		t.Fatalf("First action must be user_input, got %q", userAction.Action)
	}
	taskText := userAction.Text

	mr := newMockRunner(t, actions, 1)
	expectedMDFiles := expectedMarkdownFiles(actions)

	var hookSequence []string

	// --- Hook: prepare_to_run_agent ---
	runJSONAgentHook = func(agentName, invocationID, prompt string) {
		hookSequence = append(hookSequence, "prepare_to_run_agent")
		action := mr.next("prepare_to_run_agent")

		if action.InvocationID != invocationID {
			mr.t.Fatalf("prepare_to_run_agent invocation_id mismatch: expected %q, got %q",
				action.InvocationID, invocationID)
		}
		if action.Agent != agentName {
			mr.t.Fatalf("prepare_to_run_agent agent mismatch: expected %q, got %q",
				action.Agent, agentName)
		}
		normalizedDataPrompt := normalizePrompt(action.Prompt, action.Agent, actions)
		normalizedCodePrompt := normalizePrompt(prompt, action.Agent, actions)
		if normalizedDataPrompt != normalizedCodePrompt {
			mr.t.Fatalf("prepare_to_run_agent prompt mismatch:\n%s", mr.diff(normalizedDataPrompt, normalizedCodePrompt, "data", "code"))
		}
	}

	// --- Hook: run_codex ---
	runCodexHook = func(agentName, session, prompt string, schema map[string]interface{}, timeout string) (string, string, error) {
		hookSequence = append(hookSequence, "run_codex")
		action := mr.next("run_codex")

		if action.Agent != agentName {
			mr.t.Fatalf("run_codex agent_name mismatch: expected %q, got %q",
				action.Agent, agentName)
		}
		if action.Timeout != timeout {
			mr.t.Fatalf("run_codex timeout mismatch: expected %q, got %q",
				action.Timeout, timeout)
		}
		normalizedDataPrompt := normalizePrompt(action.Prompt, action.Agent, actions)
		normalizedCodePrompt := normalizePrompt(prompt, action.Agent, actions)
		if normalizedDataPrompt != normalizedCodePrompt {
			mr.t.Fatalf("run_codex prompt mismatch:\n%s", mr.diff(normalizedDataPrompt, normalizedCodePrompt, "data", "code"))
		}
		if action.Schema == nil {
			mr.t.Fatal("run_codex schema is nil in data file")
		}
		if len(schema) == 0 {
			mr.t.Fatal("run_codex schema is empty in code")
		}
		dataSchema := []byte(MarshalJSON(action.Schema))
		codeSchema := []byte(MarshalJSON(schema))
		if string(dataSchema) != string(codeSchema) {
			(*jsonv2text.Value)(&dataSchema).Indent()
			(*jsonv2text.Value)(&codeSchema).Indent()
			mr.t.Fatalf("run_codex schema mismatch:\n%s", mr.diff(string(dataSchema), string(codeSchema), "data_schema", "code_schema"))
		}
		return action.Stdout, agentName, nil
	}

	// --- Hook: reset_agent ---
	resetHook = func(agentName, sessionSuffix string) {
		hookSequence = append(hookSequence, "reset_agent")
		action := mr.next("reset_agent")

		if action.Agent != agentName {
			mr.t.Fatalf("reset_agent agent mismatch: expected %q, got %q",
				action.Agent, agentName)
		}
		expectedSuffix, _ := action.SessionSuffix.(string)
		if sessionSuffix != expectedSuffix {
			mr.t.Fatalf("reset_agent session_suffix mismatch: expected %q, got %q",
				expectedSuffix, sessionSuffix)
		}
	}

	// --- Hook: write_markdown_doc ---
	markdownDocHook = func(content interface{}, stageName string, subdir []string) string {
		hookSequence = append(hookSequence, "write_markdown_doc")
		action := mr.next("write_markdown_doc")

		if action.StageName != stageName {
			mr.t.Fatalf("write_markdown_doc stage_name mismatch: expected %q, got %q",
				action.StageName, stageName)
		}

		expectedContent := RenderMarkdownContent(content, stageName)
		if action.Content != expectedContent {
			mr.t.Fatalf("write_markdown_doc content mismatch for stage %q:\n%s",
				stageName, mr.diff(action.Content, expectedContent, "data", "code"))
		}

		return writeMarkdownDocument(stageName, expectedContent, subdir)
	}

	// --- Hook: watchman ---
	watchmanHook = func() map[string]string {
		hookSequence = append(hookSequence, "watchman")
		action := mr.next("watchman")
		return action.Changes
	}

	mr.t.Cleanup(func() {
		if mr.t.Failed() {
			mr.t.Logf("Hook call sequence (%d calls):", len(hookSequence))
			for i, call := range hookSequence {
				mr.t.Logf("  %3d: %s", i+1, call)
			}
		}
	})

	subdir := t.TempDir()
	orch := NewOrchestrator(taskText, subdir)
	orch.Run()

	mr.verifyAllConsumed()

	docStoresDir := filepath.Join(subdir, "document_stores")
	if _, err := os.Stat(docStoresDir); os.IsNotExist(err) {
		mr.t.Fatalf("document_stores directory not found at %s", docStoresDir)
	}

	actualFiles := make(map[string]string)
	err := filepath.Walk(docStoresDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			content, _ := os.ReadFile(path)
			base := strings.TrimSuffix(info.Name(), ".md")
			// Strip timestamp prefix: YYYY-MM-DD_HH-MM-SS_
			stageName := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}_`).ReplaceAllString(base, "")
			actualFiles[stageName] = string(content)
		}
		return nil
	})
	if err != nil {
		mr.t.Fatalf("Error walking document_stores: %v", err)
	}

	for stageName, expectedContent := range expectedMDFiles {
		actual, ok := actualFiles[stageName]
		if !ok {
			mr.t.Fatalf("Expected file for stage %q not found in document_stores", stageName)
		}
		if actual != expectedContent {
			mr.t.Fatalf("File content mismatch for stage %q:\n%s",
				stageName, mr.diff(actual, expectedContent, "data", "code"))
		}
	}
}

func (mr *e2eMockRunner) diff(left, right, leftLabel, rightLabel string) string {
	f1, _ := os.CreateTemp("", "e2e-diff-left-*.txt")
	f2, _ := os.CreateTemp("", "e2e-diff-right-*.txt")
	defer os.Remove(f1.Name())
	defer os.Remove(f2.Name())
	f1.WriteString(left)
	f2.WriteString(right)
	f1.Close()
	f2.Close()
	cmd := exec.Command("diff", "-u", "--label", leftLabel, "--label", rightLabel, f1.Name(), f2.Name())
	output, _ := cmd.CombinedOutput()
	return string(output)
}
