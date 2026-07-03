package main

import (
	"bufio"
	jsonv2text "encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"github.com/datadog/zstd"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	state "agent-go/state"
	td "agent-go/test_data"
	"github.com/benbjohnson/immutable"
)

type actionDetails struct {
	dataAction td.DataAction
}

func newActionDetails(a td.DataAction) actionDetails {
	return actionDetails{dataAction: a}
}

func (ad actionDetails) Action() string {
	return ad.dataAction.Action()
}

func (ad actionDetails) Details() *td.ActionDetails {
	return ad.dataAction.Details()
}

func (ad actionDetails) Agent() string {
	v := ad.Details().Agent()
	if v == nil {
		return ""
	}
	return *v
}

func (ad actionDetails) InvocationID() string {
	v := ad.Details().InvocationId()
	if v == nil {
		return ""
	}
	return *v
}

func (ad actionDetails) Prompt() string {
	v := ad.Details().Prompt()
	if v == nil {
		return ""
	}
	return *v
}

func (ad actionDetails) Schema() map[string]interface{} {
	items := ad.Details().Schema().Items()
	m := make(map[string]interface{}, len(items))
	for _, item := range items {
		m[item.Key] = item.Value
	}
	return m
}

func (ad actionDetails) Timeout() string {
	v := ad.Details().Timeout()
	if v == nil {
		return ""
	}
	return *v
}

func (ad actionDetails) Stdout() string {
	v := ad.Details().Stdout()
	if v == nil {
		return ""
	}
	return *v
}

func (ad actionDetails) SessionSuffix() string {
	v := ad.Details().SessionSuffix()
	if v == nil {
		return ""
	}
	return *v
}

func (ad actionDetails) StageName() string {
	v := ad.Details().StageName()
	if v == nil {
		return ""
	}
	return *v
}

func (ad actionDetails) Content() string {
	v := ad.Details().Content()
	if v == nil {
		return ""
	}
	return *v
}

func (ad actionDetails) Changes() map[string]string {
	items := ad.Details().Changes().Items()
	m := make(map[string]string, len(items))
	for _, item := range items {
		m[item.Key] = item.Value
	}
	return m
}

func normalizeTestCaseLine(line string) string {
	// line = regexp.MustCompile(`"approved", "resolved_issues", "approved_confidence", "approved_reason", "should_reset",`).ReplaceAllString(line, `"approved", "approved_confidence", "approved_reason", "resolved_issues", "should_reset",`)
	// line = regexp.MustCompile(`"status", "brief_summary", "blocked_reason", "exists_after_change",`).ReplaceAllString(line, `"status", "blocked_reason", "brief_summary", "exists_after_change",`)
	// line = regexp.MustCompile(`"reviewer_notes": {"type": "array",`).ReplaceAllString(line, `"reviewer_notes": {"type": ["array", "null"],`)
	// line = regexp.MustCompile(`"next_steps": {"type": "array",`).ReplaceAllString(line, `"next_steps": {"type": ["array", "null"],`)
	// line = regexp.MustCompile(`"dependencies": {"type": "array",`).ReplaceAllString(line, `"dependencies": {"type": ["array", "null"],`)
	// line = regexp.MustCompile(`"domain_specification": {`).ReplaceAllString(line, `"domain_specification": {"minLength": 1,`)
	return line
}

type testCase struct {
	fileName string
	testCase td.TestCase
}

func (tc testCase) Step() *state.WorkflowStep {
	return tc.testCase.Step()
}

func (tc testCase) Actions() []actionDetails {
	var actions []actionDetails
	if tc.testCase.Actions() != nil {
		for i := 0; i < tc.testCase.Actions().Len(); i++ {
			actions = append(actions, newActionDetails(tc.testCase.Actions().Get(i)))
		}
	}
	return actions
}

func (tc testCase) Followups() *immutable.List[state.WorkflowStep] {
	return tc.testCase.Followups()
}

func (tc testCase) Task() string {
	return tc.testCase.Step().State().Task()
}

func (tc testCase) Subdir() string {
	return tc.testCase.Step().State().Subdir()
}

func (tc testCase) followupSlice() []state.WorkflowStep {
	if tc.testCase.Followups() == nil {
		return nil
	}
	lst := (*immutable.List[state.WorkflowStep])(tc.testCase.Followups())
	result := make([]state.WorkflowStep, lst.Len())
	for i := 0; i < lst.Len(); i++ {
		result[i] = lst.Get(i)
	}
	return result
}

func fixup(step *state.WorkflowStep) *state.WorkflowStep {
	return step.Clone().WithState(step.State().Clone().WithSubdir("[redacted]").Build()).Build()
}

type e2eMockRunner struct {
	actions []actionDetails
	idx     int
	t       *testing.T
}

func newMockRunner(t *testing.T, actions []actionDetails) *e2eMockRunner {
	return &e2eMockRunner{actions: actions, idx: 0, t: t}
}

func (mr *e2eMockRunner) next(actualAction string, desc string) actionDetails {
	if desc != "" {
		if os.Getenv("E2E_VERBOSE") != "" {
			desc = desc + "\n"
		} else {
			desc = ""
		}
	}
	if mr.idx >= len(mr.actions) {
		mr.t.Fatalf("%sUnexpected call: actual %s but no more actions remain (idx=%d, total=%d)",
			desc, actualAction, mr.idx, len(mr.actions))
	}
	action := mr.actions[mr.idx]
	mr.idx++
	if action.Action() != actualAction {
		mr.t.Fatalf("%sActual action %q at index %d (total=%d), expected %q (agent=%q, invocation_id=%q, stage_name=%q)",
			desc, actualAction, mr.idx-1, len(mr.actions),
			action.Action(), action.Agent(), action.InvocationID(), action.StageName())
	}
	return action
}

func (mr *e2eMockRunner) verifyAllConsumed() {
	if mr.idx < len(mr.actions) {
		mr.t.Fatalf("Not all actions were consumed: %d remaining (idx=%d, total=%d)",
			len(mr.actions)-mr.idx, mr.idx, len(mr.actions))
	}
}

func normalizeErrorMessages(prompt string) string {
	re := regexp.MustCompile(`<error>\n(.*?)\n</error>`)
	return re.ReplaceAllStringFunc(prompt, func(match string) string {
		inner := strings.TrimPrefix(match, "<error>\n")
		inner = strings.TrimSuffix(inner, "\n</error>")

		if strings.Contains(inner, "invalid character") ||
			strings.Contains(inner, "Expecting ':' delimiter") ||
			(strings.Contains(inner, "line ") && strings.Contains(inner, "column ")) {
			return "<error>\nGO_JSON_UNMARSHAL_ERROR\n</error>"
		}

		if strings.Contains(inner, "Error within") ||
			strings.Contains(inner, "Additional properties") ||
			strings.Contains(inner, "doesn't validate with") {
			return "<error>\nSCHEMA_VALIDATION_ERROR\n</error>"
		}

		if strings.Contains(inner, "does not contain") && strings.Contains(inner, "JSON") {
			return "<error>\nEXTRACT_JSON_ERROR\n</error>"
		}
		return match
	})
}

func normalizeDocumentStoresPath(text string) string {
	return regexp.MustCompile(`[^\n\r]+/document_stores/[^\n\r]+\.md`).ReplaceAllStringFunc(text, func(m string) string {
		idx := strings.Index(m, "/document_stores/")
		if idx < 0 {
			return m
		}
		filename := m[idx+len("/document_stores/"):]

		filename = regexp.MustCompile(`^[\d\-_+:]+[_-]?`).ReplaceAllString(filename, "")
		return "SUBDIR/document_stores/" + filename
	})
}

func normalizePrompt(prompt string, agent string, actions []actionDetails) string {
	result := strings.TrimSpace(prompt)
	result = regexp.MustCompile(`\n`).ReplaceAllString(result, "\n")
	result = regexp.MustCompile("\n\n\n").ReplaceAllString(result, "\n\n")
	result = normalizeJSONObjects(result)
	result = normalizeErrorMessages(result)
	result = normalizeDocumentStoresPath(result)
	return result
}

func TestE2E(t *testing.T) {
	files, err := filepath.Glob("../fixtures/*.json")
	if err != nil {
		t.Fatalf("Failed to glob fixtures directory: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("No .json files found in ../fixtures/")
	}
	for _, f := range files {
		fileName := strings.TrimSuffix(filepath.Base(f), ".json")
		t.Run(fmt.Sprintf("%s", fileName), func(t *testing.T) {
			runTestsFromFile(t, f, fileName)
		})
	}
}

func runTestsFromFile(t *testing.T, f string, fileName string) {
	t.Parallel()
	fh, err := os.Open(f)
	if err != nil {
		t.Fatalf("Failed to read test data file %s: %v", f, err)
	}
	defer fh.Close()
	r := zstd.NewReader(fh)
	defer r.Close()
	data := bufio.NewScanner(r)
	buf := make([]byte, 0, 10*1024*1024)
	data.Buffer(buf, 100*1024*1024)
	counter := 0
	for data.Scan() {
		counter++
		line := strings.TrimSpace(data.Text())
		if line == "" {
			continue
		}
		t.Run(fmt.Sprintf("%d", counter), func(t *testing.T) {
			runTestCase(t, fileName, counter, line)
		})
	}
}

func runTestCase(t *testing.T, fileName string, counter int, line string) {
	t.Parallel()
	line = normalizeTestCaseLine(line)
	var tc td.TestCase
	if err := jsonv2.Unmarshal([]byte(line), &tc, jsonv2text.AllowDuplicateNames(true)); err != nil {
		t.Fatalf("%s:%d: %v", fileName, counter, err)
	}
	runTestCaseImpl(t, testCase{
		fileName: fileName,
		testCase: tc,
	})
}

func runTestCaseImpl(t *testing.T, tc testCase) {
	actions := tc.Actions()

	mr := newMockRunner(t, actions)
	expectedMDFiles := make(map[string]string)

	var hookSequence []string

	c := NewContext()
	c.runJSONAgentHook = func(agentName, invocationID, prompt string) *td.AgentState {
		hookSequence = append(hookSequence, "prepare_to_run_agent")
		action := mr.next("prepare_to_run_agent", fmt.Sprintf("%s (%s)\n%s", invocationID, agentName, prompt))

		if action.InvocationID() != invocationID {
			mr.t.Fatalf("prepare_to_run_agent invocation_id mismatch: expected %q, got %q",
				action.InvocationID(), invocationID)
		}
		if action.Agent() != agentName {
			mr.t.Fatalf("prepare_to_run_agent agent mismatch: expected %q, got %q",
				action.Agent(), agentName)
		}
		normalizedDataPrompt := normalizePrompt(action.Prompt(), action.Agent(), actions)
		normalizedCodePrompt := normalizePrompt(prompt, agentName, actions)
		if normalizedDataPrompt != normalizedCodePrompt {
			mr.t.Fatalf("prepare_to_run_agent(%s, %s) prompt mismatch:\n%s", agentName, invocationID, mr.diff(normalizedDataPrompt, normalizedCodePrompt, "data", "code"))
		}
		return action.Details().AgentState()
	}

	c.runCodexHook = func(agentName, session, prompt string, schema map[string]interface{}, timeout string) (string, string, error) {
		hookSequence = append(hookSequence, "run_codex")
		action := mr.next("run_codex", fmt.Sprintf("%s\n%s", agentName, prompt))

		if action.Agent() != agentName {
			mr.t.Fatalf("run_codex agent_name mismatch: expected %q, got %q",
				action.Agent(), agentName)
		}
		if action.Timeout() != timeout {
			mr.t.Fatalf("run_codex timeout mismatch: expected %q, got %q",
				action.Timeout(), timeout)
		}
		normalizedDataPrompt := normalizePrompt(action.Prompt(), action.Agent(), actions)
		normalizedCodePrompt := normalizePrompt(prompt, agentName, actions)
		if normalizedDataPrompt != normalizedCodePrompt {
			desc := ""
			if os.Getenv("E2E_VERBOSE") != "" {
				desc = fmt.Sprintf("%s\n%s\n", agentName, normalizedCodePrompt)
			}
			mr.t.Fatalf("%srun_codex prompt mismatch:\n%s", desc, mr.diff(normalizedDataPrompt, normalizedCodePrompt, "data", "code"))
		}
		if action.Schema() == nil {
			mr.t.Fatal("run_codex schema is nil in data file")
		}
		if len(schema) == 0 {
			mr.t.Fatal("run_codex schema is empty in code")
		}
		dataSchema := []byte(MarshalJSON(action.Schema()))
		codeSchema := []byte(MarshalJSON(schema))
		if string(dataSchema) != string(codeSchema) {
			(*jsonv2text.Value)(&dataSchema).Indent()
			(*jsonv2text.Value)(&codeSchema).Indent()
			desc := ""
			if os.Getenv("E2E_VERBOSE") != "" {
				desc = fmt.Sprintf("%s\n", agentName)
			}
			mr.t.Fatalf("%srun_codex schema mismatch:\n%s", desc, mr.diff(string(dataSchema), string(codeSchema), "data_schema", "code_schema"))
		}
		if action.Stdout() == "" {
			return "", "", fmt.Errorf("Empty output, likely timeout issue")
		}
		return action.Stdout(), action.Agent(), nil
	}

	c.resetHook = func(agentName, sessionSuffix string) {
		hookSequence = append(hookSequence, "reset_agent")
		action := mr.next("reset_agent", agentName)

		if action.Agent() != agentName {
			mr.t.Fatalf("reset_agent agent mismatch: expected %q, got %q",
				action.Agent(), agentName)
		}
		expectedSuffix := action.SessionSuffix()
		if sessionSuffix != expectedSuffix {
			mr.t.Fatalf("reset_agent session_suffix mismatch: expected %q, got %q",
				expectedSuffix, sessionSuffix)
		}
	}

	c.markdownDocHook = func(content interface{}, stageNameRaw string, subdir []string) string {
		stageName := regexp.MustCompile(`[0-9]+$`).ReplaceAllString(stageNameRaw, "")
		hookSequence = append(hookSequence, "write_markdown_doc")
		action := mr.next("write_markdown_doc", stageNameRaw)

		if action.StageName() != stageName {
			mr.t.Fatalf("write_markdown_doc stage_name mismatch: expected %q, got %q",
				action.StageName(), stageName)
		}

		actualContent := RenderMarkdownContent(content)
		if stageName != "code_summary" {
			if action.Content() != actualContent {
				mr.t.Fatalf("write_markdown_doc content mismatch for stage %q:\n%s",
					stageNameRaw, mr.diff(action.Content(), actualContent, "data", "code"))
			}

			var filename string
			if len(subdir) > 1 {
				parts := []string{}
				parts = append(parts, subdir[1:]...)
				parts = append(parts, stageNameRaw)
				filename = filepath.Join(parts...)
			} else {
				filename = stageNameRaw
			}
			expectedMDFiles[filename] = action.Content()
		}
		return writeMarkdownDocument(stageNameRaw, actualContent, subdir)
	}

	c.watchmanHook = func() map[string]string {
		hookSequence = append(hookSequence, "watchman")
		action := mr.next("watchman", "")
		return action.Changes()
	}

	mr.t.Cleanup(func() {
		if mr.t.Failed() {
			mr.t.Logf("Hook call sequence (%d calls):", len(hookSequence))
			for i, call := range hookSequence {
				mr.t.Logf("  %3d: %s", i+1, call)
			}
		}
	})

	tempSubdir := mr.t.TempDir()

	// Clone the step with a temp subdir to avoid cached responses
	originalStep := tc.Step()
	newState := originalStep.State().Clone().WithSubdir(tempSubdir).Build()
	newStep := originalStep.Clone().WithState(newState).Build()

	orch := NewOrchestrator("", tempSubdir)
	// Capture stack length BEFORE executeStep to correctly identify followups
	nextStepIdx := orch.Stack.Len()
	orch.executeStep(newStep, c)
	followUpSteps := orch.Stack.GetFrom(nextStepIdx)

	// Validate followups using MarshalJSON + normalizeJSONObjects
	expectedFollowups := tc.followupSlice()
	if len(followUpSteps) != len(expectedFollowups) {
		mr.t.Fatalf("Followup count mismatch: expected %d, got %d", len(expectedFollowups), len(followUpSteps))
	}
	for i := range followUpSteps {
		expectedJSON := MarshalJSON(*fixup(&expectedFollowups[i]))
		actualJSON := MarshalJSON(*fixup(&followUpSteps[i]))
		normalizedExpected := normalizeDocumentStoresPath(normalizeJSONObjects(expectedJSON))
		normalizedActual := normalizeDocumentStoresPath(normalizeJSONObjects(actualJSON))
		if normalizedExpected != normalizedActual {
			mr.t.Fatalf("Followup step %d mismatch:\n%s", i, mr.diff(normalizedExpected, normalizedActual, "data", "code"))
		}
	}

	mr.verifyAllConsumed()

	if len(expectedMDFiles) > 0 {
		docStoresDir := filepath.Join(tempSubdir, "document_stores")
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

				stageName := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}_`).ReplaceAllString(base, "")
				relpath, _ := filepath.Rel(docStoresDir, path)
				actualFiles[filepath.Join(filepath.Dir(relpath), stageName)] = string(content)
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

func normalizeJSONObjects(prompt string) string {
	lines := strings.Split(prompt, "\n")
	for i, line := range lines {
		var startIdx int
		switch {
		case strings.HasSuffix(line, "}"):
			startIdx = strings.Index(line, "{")
		case strings.HasSuffix(line, "]"):
			startIdx = strings.Index(line, "[")
		default:
			continue
		}
		if startIdx < 0 {
			continue
		}

		extracted := line[startIdx:]
		var v any
		if err := jsonv2.Unmarshal([]byte(extracted), &v, jsonv2text.AllowDuplicateNames(true)); err != nil {
			continue
		}
		v = sortJSONKeys(v)
		b := []byte(MarshalJSON(v))
		(*jsonv2text.Value)(&b).Indent()

		prefix := line[:startIdx]
		lines[i] = prefix + string(b)
	}
	return strings.Join(lines, "\n")
}

func sortJSONKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		sorted := make(map[string]any, len(val))
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sorted[k] = sortJSONKeys(val[k])
		}
		return sorted
	case []any:
		for i, elem := range val {
			val[i] = sortJSONKeys(elem)
		}
	}
	return v
}
