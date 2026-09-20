package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"agent-go/pkg/loader"
)

func init() {
	loader.InitPromptLoader()
	loader.InitPrompts()
	initSubworkflows()
}

// ---------------------------------------------------------------------------
// Mock LLM runtime
// ---------------------------------------------------------------------------

type recordedPrompt struct {
	agent  string
	prompt string
}

// script drives the mock codex runtime. Decision agents (non-nil schema) get
// scripted JSON responses; content agents get scripted documents written to
// the pregenerated output path found in their prompt.
type script struct {
	t *testing.T

	// Driver scenario.
	DriverChoices         []string // consumed in order, one per valid driver response
	DriverReassessChoices []string // consumed in order, one per finish-reassessment response
	DriverReassessRaw     []string // raw JSON reassessment responses, consumed before DriverReassessChoices
	DriverFailAfter       int      // after this many driver calls, calls fail (0 = off)
	DriverBadFirst        int      // first N driver responses fail strict validation

	// Loop decider scenario.
	LoopRepeat   []bool // consumed in order per decider call; default (empty) = false
	LoopBadFirst int    // first N decider responses fail strict validation

	// Content agent scenario.
	SkipFile       map[string]bool // agent never writes its output file
	FailFirst      map[string]int  // agent -> number of invocations that fail with a runtime error
	EmptyFileFirst map[string]bool // agent writes an empty file on its first attempt

	driverCalls    int
	choiceIdx      int
	reassessIdx    int
	reassessRawIdx int
	loopCalls      int

	subdir        string
	indexSnapshot map[string]string // agent -> artifact index contents when it ran

	prompts []recordedPrompt
}

func newScript(t *testing.T) *script {
	return &script{
		t:              t,
		SkipFile:       map[string]bool{},
		FailFirst:      map[string]int{},
		EmptyFileFirst: map[string]bool{},
		indexSnapshot:  map[string]string{},
	}
}

var outputPathRe = regexp.MustCompile(`to this exact path:\n\s+(\S+)`)

func extractOutputPath(t *testing.T, prompt string) string {
	m := outputPathRe.FindStringSubmatch(prompt)
	if m == nil {
		preview := prompt
		if len(preview) > 400 {
			preview = preview[:400]
		}
		t.Fatalf("no output path found in prompt:\n%s", preview)
	}
	return m[1]
}

// assertNoArtifactPaths checks that a prompt carries no artifact file path
// other than the agent's own pregenerated output path (under OUTPUT
// REQUIREMENTS): agents receive only the artifact index and select the
// documents to study themselves.
func assertNoArtifactPaths(t *testing.T, prompt string, st *WorkflowState) {
	t.Helper()
	own := ""
	if m := outputPathRe.FindStringSubmatch(prompt); m != nil {
		own = m[1]
	}
	for i := range st.Artifacts {
		if st.Artifacts[i].Path == own {
			continue
		}
		if strings.Contains(prompt, st.Artifacts[i].Path) {
			t.Errorf("prompt must not contain artifact path %s", st.Artifacts[i].Path)
		}
	}
}

func (s *script) driverResponse(prompt string) (string, error) {
	s.driverCalls++
	if s.DriverFailAfter > 0 && s.driverCalls > s.DriverFailAfter {
		return "", fmt.Errorf("simulated driver failure")
	}
	if s.driverCalls <= s.DriverBadFirst {
		return `{"subworkflow":"classify","rationale":"broken","unexpected_field":true}`, nil
	}
	// Finish-reassessment turns carry the marker in their prompt; they
	// consume a separate scripted queue (default: confirm the finish).
	if strings.Contains(prompt, finishReassessmentMarker) {
		if s.reassessRawIdx < len(s.DriverReassessRaw) {
			raw := s.DriverReassessRaw[s.reassessRawIdx]
			s.reassessRawIdx++
			return raw, nil
		}
		choice := "finish"
		if s.reassessIdx < len(s.DriverReassessChoices) {
			choice = s.DriverReassessChoices[s.reassessIdx]
			s.reassessIdx++
		}
		if choice == "finish" && strings.Contains(prompt, bootstrapRunStatePrefix) {
			choice = "spec"
		}
		if choice == "finish" {
			resp := map[string]any{
				"decision":    "confirm_finish",
				"subworkflow": "finish",
				"rationale":   "Scripted rationale for finish",
				"task":        "Scripted task: cover the auth domain.",
			}
			return prettyJSON(resp), nil
		}
		resp := map[string]any{
			"decision":    "select_subworkflow",
			"subworkflow": choice,
			"rationale":   "Scripted rationale for " + choice,
			"task":        "Scripted task: cover the auth domain.",
		}
		return prettyJSON(resp), nil
	}
	choice := "finish"
	if s.choiceIdx < len(s.DriverChoices) {
		choice = s.DriverChoices[s.choiceIdx]
		s.choiceIdx++
	}
	resp := map[string]any{
		"subworkflow": choice,
		"rationale":   "Scripted rationale for " + choice,
		"task":        "Scripted task: cover the auth domain.",
	}
	return prettyJSON(resp), nil
}

func (s *script) loopResponse() (string, error) {
	s.loopCalls++
	if s.loopCalls <= s.LoopBadFirst {
		return `{"reason":"broken response without the repeat field"}`, nil
	}
	repeat := false
	if s.loopCalls-1 < len(s.LoopRepeat) {
		repeat = s.LoopRepeat[s.loopCalls-1]
	}
	return prettyJSON(map[string]any{
		"repeat": repeat,
		"reason": fmt.Sprintf("scripted decider reason %d", s.loopCalls),
	}), nil
}

var reviewAgents = map[string]bool{
	"pm_review":                         true,
	"system_decomposition_review":       true,
	"arch_review":                       true,
	"plan_review":                       true,
	"code_review":                       true,
	"investigation_plan_quality_review": true,
	"structure_review":                  true,
	"fact_checking_review":              true,
	"gap_analysis_review":               true,
	"synthesis_consistency_review":      true,
	"tech_lead_final":                   true,
	"arch_final":                        true,
}

func (s *script) codexHook(agentName, prompt, timeout string, schema map[string]any) (string, error) {
	s.prompts = append(s.prompts, recordedPrompt{agentName, prompt})

	if schema != nil {
		switch agentName {
		case "workflow_driver":
			return s.driverResponse(prompt)
		case "loop_decider":
			return s.loopResponse()
		}
		s.t.Fatalf("unexpected decision agent %q", agentName)
		return "", nil
	}

	if s.indexSnapshot[agentName] == "" {
		if data, err := os.ReadFile(indexPath(s.subdir)); err == nil {
			s.indexSnapshot[agentName] = string(data)
		}
	}

	if s.FailFirst[agentName] > 0 {
		s.FailFirst[agentName]--
		return "", fmt.Errorf("simulated runtime failure for %s", agentName)
	}

	path := extractOutputPath(s.t, prompt)
	if s.SkipFile[agentName] {
		return "ack", nil
	}
	if s.EmptyFileFirst[agentName] {
		s.EmptyFileFirst[agentName] = false
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			s.t.Fatalf("mock: cannot write %s: %v", path, err)
		}
		return "ack", nil
	}

	var content string
	if reviewAgents[agentName] {
		content = fmt.Sprintf(`# Review

## Verdict
**Verdict:** APPROVED

## Issues
None

## Notes
Scripted review by %s.
`, agentName)
	} else {
		content = fmt.Sprintf(`# Document

## Summary
Scripted %s document body.
`, agentName)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		s.t.Fatalf("mock: cannot write %s: %v", path, err)
	}
	return "ack", nil
}

// ---------------------------------------------------------------------------
// Scenario runner
// ---------------------------------------------------------------------------

type scenario struct {
	subdir string
	script *script
}

func runScenario(t *testing.T, task string, s *script, watchman map[string]string) *scenario {
	t.Helper()
	subdir := t.TempDir()
	s.subdir = subdir
	orch := NewOrchestrator(subdir)
	sc := &scenario{subdir: subdir, script: s}
	orch.contextFactory = func() *Context {
		c := NewContext()
		c.runCodexHook = s.codexHook
		if watchman != nil {
			c.watchmanHook = func() map[string]string { return watchman }
		}
		return c
	}
	if err := orch.Run(task, subdir); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	return sc
}

func (sc *scenario) state(t *testing.T) *WorkflowState {
	t.Helper()
	st, err := loadWorkflowState(sc.subdir)
	if err != nil {
		t.Fatalf("cannot load state: %v", err)
	}
	return st
}

func (sc *scenario) promptFor(t *testing.T, agent string, nth int) string {
	t.Helper()
	seen := 0
	for i := range sc.script.prompts {
		if sc.script.prompts[i].agent == agent {
			seen++
			if seen == nth {
				return sc.script.prompts[i].prompt
			}
		}
	}
	t.Fatalf("no prompt #%d recorded for agent %s", nth, agent)
	return ""
}

func (sc *scenario) promptCount(t *testing.T, agent string) int {
	t.Helper()
	n := 0
	for i := range sc.script.prompts {
		if sc.script.prompts[i].agent == agent {
			n++
		}
	}
	return n
}

func assertArtifactsOnDisk(t *testing.T, st *WorkflowState) {
	t.Helper()
	for i := range st.Artifacts {
		a := &st.Artifacts[i]
		info, err := os.Stat(a.Path)
		if err != nil {
			t.Errorf("artifact %s missing on disk: %v", a.ID, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("artifact %s is empty on disk", a.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

func TestOrchestrator_EngineeringHappyPath(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"classify", "spec", "decompose", "architect", "plan", "implement", "arch_final", "finish"}

	sc := runScenario(t, "Build an auth module with token refresh.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should be finished")
	}
	if len(st.History) != 8 {
		t.Fatalf("history = %d entries, want 8: %+v", len(st.History), st.History)
	}
	wantAgents := []string{
		"investigation_classifier",
		"product_manager", "pm_review",
		"system_decomposition", "system_decomposition_review",
		"arch", "arch_review",
		"plan", "plan_review",
		"coder", "code_review",
		"arch_final",
	}
	if len(st.Artifacts) != len(wantAgents) {
		t.Fatalf("artifacts = %d, want %d: %+v", len(st.Artifacts), len(wantAgents), st.Artifacts)
	}
	for i, agent := range wantAgents {
		if st.Artifacts[i].Agent != agent {
			t.Errorf("artifact %d agent = %s, want %s", i, st.Artifacts[i].Agent, agent)
		}
	}
	assertArtifactsOnDisk(t, st)

	// Outcomes recorded per iteration.
	for i, h := range st.History {
		if i == len(st.History)-1 {
			if h.Outcome != "finished" {
				t.Errorf("last history outcome = %q, want finished", h.Outcome)
			}
			continue
		}
		switch h.Subworkflow {
		case "classify", "arch_final":
			if h.Outcome != "completed" {
				t.Errorf("history %d outcome = %q, want completed", i, h.Outcome)
			}
		default:
			if !strings.HasPrefix(h.Outcome, "completed: loop stopped after 1 round(s); decider: ") {
				t.Errorf("history %d outcome = %q, want loop-stopped-after-1-round prefix", i, h.Outcome)
			}
		}
	}

	// Validated driver decisions are persisted as JSON files.
	for i, h := range st.History {
		if h.Subworkflow == "finish" {
			continue
		}
		data, err := os.ReadFile(h.DecisionPath)
		if err != nil {
			t.Errorf("history %d: decision file missing: %v", i, err)
			continue
		}
		var dd map[string]any
		if err := json.Unmarshal(data, &dd); err != nil {
			t.Errorf("history %d: decision file is not valid JSON: %v", i, err)
			continue
		}
		if dd["subworkflow"] != h.Subworkflow {
			t.Errorf("history %d: decision file subworkflow = %v, want %s", i, dd["subworkflow"], h.Subworkflow)
		}
	}

	// The static wiring is recorded in the session trace at run start.
	defs, err := os.ReadFile(filepath.Join(sc.subdir, ".state", "static_definitions.md"))
	if err != nil {
		t.Fatalf("static definitions trace missing: %v", err)
	}
	if !strings.Contains(string(defs), "## Agents") || !strings.Contains(string(defs), "## Wiring Graph") {
		t.Error("static definitions trace malformed")
	}

	// SUMMARY.md written with the driver's finish rationale.
	summary, err := os.ReadFile(filepath.Join(sc.subdir, "SUMMARY.md"))
	if err != nil {
		t.Fatalf("SUMMARY.md missing: %v", err)
	}
	if !strings.Contains(string(summary), "Scripted rationale for finish") {
		t.Errorf("SUMMARY.md should carry the finish rationale")
	}

	// Driver prompt carries the full subworkflow menu and progress marker.
	driverPrompt := sc.promptFor(t, "workflow_driver", 1)
	for _, id := range subworkflowIDs() {
		if !strings.Contains(driverPrompt, "* id: "+id+"\n") {
			t.Errorf("driver prompt missing subworkflow %q", id)
		}
	}
	if !strings.Contains(driverPrompt, "PROGRESS: this is driver iteration 1.") {
		t.Error("driver prompt missing progress marker")
	}

	// The artifact index exists with one section per execution, each headed
	// by the driver task.
	idxData, err := os.ReadFile(indexPath(sc.subdir))
	if err != nil {
		t.Fatalf("artifact index missing: %v", err)
	}
	idx := string(idxData)
	for _, heading := range []string{
		"## Iteration 1 — subworkflow: classify",
		"## Iteration 2 — subworkflow: spec",
		"## Iteration 7 — subworkflow: arch_final",
	} {
		if !strings.Contains(idx, heading) {
			t.Errorf("index missing section %q", heading)
		}
	}
	if !strings.Contains(idx, "Task: Scripted task: cover the auth domain.") {
		t.Error("index missing the driver task heading")
	}
}

func TestOrchestrator_LoopRepeatsThenStops(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}
	s.LoopRepeat = []bool{true, false}

	sc := runScenario(t, "Fix the login bug.", s, nil)
	st := sc.state(t)

	h := st.History[0]
	if !strings.HasPrefix(h.Outcome, "completed: loop stopped after 2 round(s); decider: ") {
		t.Fatalf("implement outcome = %q, want loop stopped after 2 rounds", h.Outcome)
	}

	// Two rounds: coder x2, code_review x2.
	var coders, reviews []Artifact
	for i := range st.Artifacts {
		a := &st.Artifacts[i]
		if a.Agent == "coder" {
			coders = append(coders, *a)
		}
		if a.Agent == "code_review" {
			reviews = append(reviews, *a)
		}
	}
	if len(coders) != 2 || len(reviews) != 2 {
		t.Fatalf("rounds wrong: %d coders, %d reviews", len(coders), len(reviews))
	}
	if coders[1].Round != 2 || reviews[1].Round != 2 {
		t.Errorf("round numbers wrong: %+v", st.Artifacts)
	}
	if !strings.Contains(coders[1].Path, "_r2") || !strings.Contains(reviews[1].Path, "_r2") {
		t.Errorf("round-2 artifacts should carry the _r2 suffix: %s, %s", coders[1].Path, reviews[1].Path)
	}

	// Agents receive the artifact index, not document paths: the round-2
	// coder finds the round-1 review in the index, the round-2 reviewer finds
	// the round-2 report there.
	idxPath := indexPath(sc.subdir)
	coderR2 := sc.promptFor(t, "coder", 2)
	if !strings.Contains(coderR2, idxPath) {
		t.Error("round-2 coder prompt missing the artifact index path")
	}
	if !strings.Contains(coderR2, "revision round 2") {
		t.Error("round-2 coder prompt missing round note")
	}
	assertNoArtifactPaths(t, coderR2, st)

	reviewR2 := sc.promptFor(t, "code_review", 2)
	if !strings.Contains(reviewR2, idxPath) {
		t.Error("round-2 review prompt missing the artifact index path")
	}
	assertNoArtifactPaths(t, reviewR2, st)

	// Both decider calls are pointed at the index; no document path is
	// inlined.
	dec1 := sc.promptFor(t, "loop_decider", 1)
	dec2 := sc.promptFor(t, "loop_decider", 2)
	for name, prompt := range map[string]string{"decider 1": dec1, "decider 2": dec2} {
		if !strings.Contains(prompt, idxPath) {
			t.Errorf("%s prompt missing the artifact index path", name)
		}
		assertNoArtifactPaths(t, prompt, st)
	}

	// The final index lists all four documents with their rounds.
	idxData, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("artifact index missing: %v", err)
	}
	for i := range st.Artifacts {
		a := &st.Artifacts[i]
		line := fmt.Sprintf("- %s — %s (agent: %s, round %d)", a.Path, a.Description, a.Agent, a.Round)
		if !strings.Contains(string(idxData), line) {
			t.Errorf("artifact index missing entry for %s", a.ID)
		}
	}
}

func TestOrchestrator_LoopUnboundedBeyondOldCap(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}
	s.LoopRepeat = []bool{true, true, true, true, false}

	sc := runScenario(t, "Fix the login bug.", s, nil)
	st := sc.state(t)

	if !strings.HasPrefix(st.History[0].Outcome, "completed: loop stopped after 5 round(s)") {
		t.Fatalf("implement outcome = %q, want loop stopped after 5 rounds (old cap was 3)", st.History[0].Outcome)
	}

	var maxRound int
	for i := range st.Artifacts {
		if st.Artifacts[i].Round > maxRound {
			maxRound = st.Artifacts[i].Round
		}
	}
	if maxRound != 5 {
		t.Errorf("max artifact round = %d, want 5", maxRound)
	}
	if len(st.Artifacts) != 10 {
		t.Errorf("artifacts = %d, want 10 (5 rounds x 2 agents)", len(st.Artifacts))
	}
}

func TestOrchestrator_DriverUnboundedBeyondOldBudget(t *testing.T) {
	s := newScript(t)
	choices := make([]string, 0, 46)
	for i := 0; i < 45; i++ {
		choices = append(choices, "classify")
	}
	choices = append(choices, "finish")
	s.DriverChoices = choices

	sc := runScenario(t, "Classify repeatedly.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should be finished")
	}
	if len(st.History) != 46 {
		t.Fatalf("history = %d entries, want 46 (old budget was 40)", len(st.History))
	}
	n := 0
	for i := range st.Artifacts {
		if st.Artifacts[i].Agent == "investigation_classifier" {
			n++
		}
	}
	if n != 45 {
		t.Errorf("classifier artifacts = %d, want 45", n)
	}
}

func TestOrchestrator_DriverInvalidJSONRetried(t *testing.T) {
	s := newScript(t)
	s.DriverBadFirst = 1
	s.DriverChoices = []string{"classify", "finish"}

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should be finished despite the invalid first driver response")
	}
	if s.driverCalls != 4 {
		t.Fatalf("driver calls = %d, want 4 (1 invalid + classify + finish + reassessment-confirm)", s.driverCalls)
	}

	first := sc.promptFor(t, "workflow_driver", 1)
	if strings.Contains(first, "<feedback>") {
		t.Error("first driver prompt must not carry retry feedback")
	}
	second := sc.promptFor(t, "workflow_driver", 2)
	if !strings.Contains(second, "<feedback>") || !strings.Contains(second, "INVALID") {
		t.Error("second driver prompt should carry strict-validation feedback")
	}
	if !strings.Contains(second, `"additionalProperties": false`) {
		t.Error("retry feedback should re-state the JSON schema")
	}
	if !strings.Contains(second, "Example of a valid response") {
		t.Error("retry feedback should carry the example response")
	}
}

func TestOrchestrator_DeciderInvalidJSONRetried(t *testing.T) {
	s := newScript(t)
	s.LoopBadFirst = 1
	s.DriverChoices = []string{"implement", "finish"}
	s.LoopRepeat = []bool{false}

	sc := runScenario(t, "Fix the login bug.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should be finished despite the invalid first decider response")
	}
	if s.loopCalls != 2 {
		t.Fatalf("decider calls = %d, want 2 (1 invalid + 1 valid)", s.loopCalls)
	}
	second := sc.promptFor(t, "loop_decider", 2)
	if !strings.Contains(second, "<feedback>") || !strings.Contains(second, "missing required field") {
		t.Error("second decider prompt should carry strict-validation feedback naming the problem")
	}
}

func TestOrchestrator_ContentAgentEmptyFileRetried(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"classify", "finish"}
	s.EmptyFileFirst["investigation_classifier"] = true

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should be finished")
	}
	if sc.promptCount(t, "investigation_classifier") != 2 {
		t.Fatalf("classifier invocations = %d, want 2", sc.promptCount(t, "investigation_classifier"))
	}
	second := sc.promptFor(t, "investigation_classifier", 2)
	if !strings.Contains(second, "<feedback>") || !strings.Contains(second, "is empty") {
		t.Error("second classifier prompt should carry retry feedback naming the empty file")
	}
}

func TestOrchestrator_RuntimeFailureRetried(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}
	s.FailFirst["coder"] = 1

	sc := runScenario(t, "Fix the parser.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should be finished")
	}
	if sc.promptCount(t, "coder") != 2 {
		t.Fatalf("coder invocations = %d, want 2", sc.promptCount(t, "coder"))
	}
	second := sc.promptFor(t, "coder", 2)
	if !strings.Contains(second, "<feedback>") || !strings.Contains(second, "the agent run itself failed") {
		t.Error("second coder prompt should carry retry feedback naming the runtime failure")
	}
}

func TestOrchestrator_Resume(t *testing.T) {
	subdir := t.TempDir()

	// Phase 1: a run that produces classify + spec documents.
	s1 := newScript(t)
	s1.DriverChoices = []string{"classify", "spec", "finish"}
	orch1 := NewOrchestrator(subdir)
	orch1.contextFactory = func() *Context {
		c := NewContext()
		c.runCodexHook = s1.codexHook
		return c
	}
	if err := orch1.Run("Investigate the outage.", subdir); err != nil {
		t.Fatalf("phase 1: %v", err)
	}
	st1, err := loadWorkflowState(subdir)
	if err != nil {
		t.Fatalf("phase 1 state: %v", err)
	}
	if len(st1.Artifacts) != 3 {
		t.Fatalf("phase 1 artifacts = %d, want 3", len(st1.Artifacts))
	}
	phase1Paths := map[string]bool{}
	for i := range st1.Artifacts {
		phase1Paths[st1.Artifacts[i].Path] = true
	}

	// Simulate a crash after the second driver turn: rewind the state to right
	// after iteration 2 (drop the finish turn, unmark finished).
	st1.Finished = false
	st1.Outcome = ""
	st1.History = st1.History[:2]
	st1.DriverIteration = 2
	if err := st1.save(subdir); err != nil {
		t.Fatalf("rewind state: %v", err)
	}

	// Phase 2: fresh orchestrator, same session dir, driver finishes the run.
	s2 := newScript(t)
	s2.DriverChoices = []string{"finish"}
	orch2 := NewOrchestrator(subdir)
	orch2.contextFactory = func() *Context {
		c := NewContext()
		c.runCodexHook = s2.codexHook
		return c
	}
	if err := orch2.Run("", subdir); err != nil {
		t.Fatalf("phase 2 resume: %v", err)
	}
	st2, _ := loadWorkflowState(subdir)
	if !st2.Finished {
		t.Fatal("phase 2 should finish the run")
	}
	if st2.DriverIteration != 3 {
		t.Errorf("driver iteration = %d, want 3", st2.DriverIteration)
	}
	if len(st2.Artifacts) != 3 {
		t.Errorf("resumed state lost artifacts: %d", len(st2.Artifacts))
	}
	// The resumed driver prompt points at the artifact index, which the
	// system rewrote with the phase-1 documents.
	seen := 0
	for i := range s2.prompts {
		if s2.prompts[i].agent == "workflow_driver" {
			seen++
			if seen == 1 {
				if !strings.Contains(s2.prompts[i].prompt, indexPath(subdir)) {
					t.Error("resumed driver prompt missing the artifact index path")
				}
			}
		}
	}
	idxData, err := os.ReadFile(indexPath(subdir))
	if err != nil {
		t.Fatalf("artifact index missing after resume: %v", err)
	}
	for p := range phase1Paths {
		if !strings.Contains(string(idxData), p) {
			t.Errorf("artifact index missing phase-1 artifact %s", p)
		}
	}
	// No duplicate artifact ids survived the resume.
	idSeen := map[string]bool{}
	for i := range st2.Artifacts {
		if idSeen[st2.Artifacts[i].ID] {
			t.Errorf("duplicate artifact id %s after resume", st2.Artifacts[i].ID)
		}
		idSeen[st2.Artifacts[i].ID] = true
	}
}

func TestOrchestrator_AlreadyFinishedNoop(t *testing.T) {
	subdir := t.TempDir()
	s := newScript(t)
	s.DriverChoices = []string{"finish"}
	orch := NewOrchestrator(subdir)
	orch.contextFactory = func() *Context {
		c := NewContext()
		c.runCodexHook = s.codexHook
		return c
	}
	if err := orch.Run("Quick task.", subdir); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if s.driverCalls != 4 {
		t.Fatalf("driver calls = %d, want 4 (finish, reassess->spec, finish, reassess->confirm)", s.driverCalls)
	}
	// The bootstrap finish was corrected: the spec subworkflow ran before
	// the run could end.
	st, err := loadWorkflowState(subdir)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if st.History[0].Subworkflow != "spec" {
		t.Errorf("history 1 subworkflow = %q, want spec", st.History[0].Subworkflow)
	}
	// Re-running the finished session is a no-op.
	if err := orch.Run("", subdir); err != nil {
		t.Fatalf("rerun of finished session: %v", err)
	}
	if s.driverCalls != 4 {
		t.Errorf("finished session must not consult the driver again (calls=%d)", s.driverCalls)
	}
}

func TestOrchestrator_WatchmanBlockInCodeReview(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}

	sc := runScenario(t, "Patch the parser.", s, map[string]string{"parser.go": "modified", "util.go": "created"})
	_ = sc

	reviewPrompt := sc.promptFor(t, "code_review", 1)
	if !strings.Contains(reviewPrompt, "AUTOMATED CHANGE DETECTION") {
		t.Fatal("code review prompt missing change-detection block")
	}
	if !strings.Contains(reviewPrompt, "parser.go") || !strings.Contains(reviewPrompt, "util.go") {
		t.Error("code review prompt missing detected files")
	}
}

func TestOrchestrator_WatchmanEmptyBlockInCodeReview(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}

	sc := runScenario(t, "Patch the parser.", s, map[string]string{})
	reviewPrompt := sc.promptFor(t, "code_review", 1)
	if !strings.Contains(reviewPrompt, "NO ACTUAL FILE CHANGES DETECTED") {
		t.Error("code review prompt should warn about zero detected changes")
	}
}

func TestOrchestrator_MultiReviewerSubworkflow(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"investigate_plan", "finish"}
	s.LoopRepeat = []bool{true, false}

	sc := runScenario(t, "Audit the billing logs.", s, nil)
	st := sc.state(t)

	if !strings.HasPrefix(st.History[0].Outcome, "completed: loop stopped after 2 round(s)") {
		t.Fatalf("investigate_plan outcome = %q, want loop stopped after 2 rounds", st.History[0].Outcome)
	}

	counts := map[string]int{}
	for i := range st.Artifacts {
		counts[st.Artifacts[i].Agent]++
	}
	if counts["investigator_planner"] != 2 || counts["investigation_plan_quality_review"] != 2 || counts["structure_review"] != 2 {
		t.Errorf("round counts wrong: %v", counts)
	}

	// Reviewers receive the artifact index (not document paths) and select
	// the planner document themselves; no prompt inlines any artifact path.
	idxPath := indexPath(sc.subdir)
	structR1 := sc.promptFor(t, "structure_review", 1)
	qualityR1 := sc.promptFor(t, "investigation_plan_quality_review", 1)
	for name, prompt := range map[string]string{"structure_review": structR1, "quality_review": qualityR1} {
		if !strings.Contains(prompt, idxPath) {
			t.Errorf("%s prompt missing the artifact index path", name)
		}
		assertNoArtifactPaths(t, prompt, st)
	}

	// Both decider calls are pointed at the index, and the final index lists
	// all six documents of the two rounds.
	dec1 := sc.promptFor(t, "loop_decider", 1)
	dec2 := sc.promptFor(t, "loop_decider", 2)
	if !strings.Contains(dec1, idxPath) || !strings.Contains(dec2, idxPath) {
		t.Error("decider prompts missing the artifact index path")
	}
	assertNoArtifactPaths(t, dec1, st)
	assertNoArtifactPaths(t, dec2, st)
	idxData, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("artifact index missing: %v", err)
	}
	for i := range st.Artifacts {
		a := &st.Artifacts[i]
		line := fmt.Sprintf("- %s — %s (agent: %s, round %d)", a.Path, a.Description, a.Agent, a.Round)
		if !strings.Contains(string(idxData), line) {
			t.Errorf("artifact index missing entry for %s", a.ID)
		}
	}
}

func TestArtifactIndex_SectionsPerExecution(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"spec", "decompose", "architect", "finish"}
	s.LoopRepeat = []bool{true, false} // spec runs two rounds

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)

	idxData, err := os.ReadFile(indexPath(sc.subdir))
	if err != nil {
		t.Fatalf("artifact index missing: %v", err)
	}
	idx := string(idxData)

	// One section per execution, headed by the driver task.
	for i, sw := range []string{"spec", "decompose", "architect"} {
		heading := fmt.Sprintf("## Iteration %d — subworkflow: %s", i+1, sw)
		if !strings.Contains(idx, heading) {
			t.Errorf("index missing section %q", heading)
		}
	}
	if got := strings.Count(idx, "Task: Scripted task: cover the auth domain."); got != 3 {
		t.Errorf("index task lines = %d, want 3", got)
	}

	// Every produced document is listed exactly once, with kind, agent and
	// round - including the round-2 spec documents.
	for i := range st.Artifacts {
		a := &st.Artifacts[i]
		line := fmt.Sprintf("- %s — %s (agent: %s, round %d)", a.Path, a.Description, a.Agent, a.Round)
		if got := strings.Count(idx, line); got != 1 {
			t.Errorf("index entry for %s appears %d times, want 1", a.ID, got)
		}
	}
	if !strings.Contains(idx, "(agent: product_manager, round 2)") {
		t.Error("index missing the round-2 product manager entry")
	}

	// No phantom entries: the number of document lines equals the number of
	// registered artifacts.
	entries := 0
	for _, ln := range strings.Split(idx, "\n") {
		if strings.HasPrefix(ln, "- /") {
			entries++
		}
	}
	if entries != len(st.Artifacts) {
		t.Errorf("index document entries = %d, want %d", entries, len(st.Artifacts))
	}
}

func TestAgentsReceiveIndexOnly(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"spec", "architect", "plan", "implement", "finish"}

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)

	idxPath := indexPath(sc.subdir)
	// The coder and the architecture reviewer each get the index path and no
	// other artifact path: they select their own input documents.
	for _, agent := range []string{"coder", "arch_review"} {
		prompt := sc.promptFor(t, agent, 1)
		if !strings.Contains(prompt, idxPath) {
			t.Errorf("%s prompt missing the artifact index path", agent)
		}
		assertNoArtifactPaths(t, prompt, st)
	}

	// Mid-execution visibility: when the architecture reviewer runs, the
	// code-maintained index already lists the architecture document of its
	// own round.
	var archPath string
	for i := range st.Artifacts {
		if st.Artifacts[i].Agent == "arch" {
			archPath = st.Artifacts[i].Path
		}
	}
	if archPath == "" {
		t.Fatal("no architecture artifact produced")
	}
	if !strings.Contains(sc.script.indexSnapshot["arch_review"], archPath) {
		t.Error("index at arch_review invocation time missing the architecture document")
	}
}

func TestDriverPrompt_IndexNotRegistry(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"spec", "architect", "finish"}

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)

	second := sc.promptFor(t, "workflow_driver", 2)
	if !strings.Contains(second, indexPath(sc.subdir)) {
		t.Error("driver prompt missing the artifact index path")
	}
	if strings.Contains(second, "ARTIFACT REGISTRY") {
		t.Error("driver prompt must not carry the old artifact registry")
	}
	assertNoArtifactPaths(t, second, st)
	// Decision history still names the tasks and the artifact ids.
	if !strings.Contains(second, "task: Scripted task: cover the auth domain.") {
		t.Error("decision history missing the driver task")
	}
	if !strings.Contains(second, st.Artifacts[0].ID) {
		t.Error("decision history should still name the artifact ids")
	}
}

func TestDeciderPrompt_HasIndexPath(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}

	sc := runScenario(t, "Fix the login bug.", s, nil)
	st := sc.state(t)

	dec := sc.promptFor(t, "loop_decider", 1)
	if !strings.Contains(dec, indexPath(sc.subdir)) {
		t.Error("decider prompt missing the artifact index path")
	}
	assertNoArtifactPaths(t, dec, st)
}

func TestResume_FoldsInterruptedExecution(t *testing.T) {
	subdir := t.TempDir()

	// Phase 1: a completed run (classify + spec + finish).
	s1 := newScript(t)
	s1.DriverChoices = []string{"classify", "spec", "finish"}
	orch1 := NewOrchestrator(subdir)
	orch1.contextFactory = func() *Context {
		c := NewContext()
		c.runCodexHook = s1.codexHook
		return c
	}
	if err := orch1.Run("Investigate the outage.", subdir); err != nil {
		t.Fatalf("phase 1: %v", err)
	}
	st1, err := loadWorkflowState(subdir)
	if err != nil {
		t.Fatalf("phase 1 state: %v", err)
	}

	// Simulate a crash mid-spec-execution: drop the spec history entry and
	// the finish turn, mark the spec execution active, unmark finished. The
	// spec's artifacts remain registered (they were persisted as produced).
	st1.Finished = false
	st1.Outcome = ""
	st1.History = st1.History[:1]
	st1.DriverIteration = 2
	st1.Active = &ActiveExecution{Iteration: 2, Subworkflow: "spec", Task: "Scripted task: cover the auth domain."}
	if err := st1.save(subdir); err != nil {
		t.Fatalf("rewind state: %v", err)
	}

	// Phase 2: a fresh orchestrator resumes and finishes the run.
	s2 := newScript(t)
	s2.DriverChoices = []string{"finish"}
	orch2 := NewOrchestrator(subdir)
	orch2.contextFactory = func() *Context {
		c := NewContext()
		c.runCodexHook = s2.codexHook
		return c
	}
	if err := orch2.Run("", subdir); err != nil {
		t.Fatalf("phase 2 resume: %v", err)
	}
	st2, _ := loadWorkflowState(subdir)

	// The interrupted execution was folded into history.
	if len(st2.History) != 3 {
		t.Fatalf("history = %d entries, want 3: %+v", len(st2.History), st2.History)
	}
	folded := st2.History[1]
	if folded.Subworkflow != "spec" || folded.Outcome != "interrupted (resumed)" {
		t.Errorf("folded entry = %+v", folded)
	}
	if folded.Task != "Scripted task: cover the auth domain." {
		t.Errorf("folded entry lost the driver task: %q", folded.Task)
	}
	if len(folded.Artifacts) != 2 {
		t.Errorf("folded entry artifacts = %v, want the two spec documents", folded.Artifacts)
	}
	if st2.Active != nil {
		t.Error("active execution must be cleared after the fold")
	}

	// The index has exactly one section for the folded iteration.
	idxData, err := os.ReadFile(indexPath(subdir))
	if err != nil {
		t.Fatalf("artifact index missing: %v", err)
	}
	if got := strings.Count(string(idxData), "## Iteration 2 — subworkflow: spec"); got != 1 {
		t.Errorf("index sections for iteration 2 = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// Finish reassessment
// ---------------------------------------------------------------------------

// The reported bug: on a fresh run (zero artifacts) the driver answered
// "finish" on its very first turn. The mandatory reassessment must send the
// full response back, and the corrected choice (spec) must be executed.
func TestOrchestrator_FinishReassessmentOverridesBootstrapFinish(t *testing.T) {
	s := newScript(t)
	// Every regular driver turn answers "finish" (script default); the first
	// reassessment overrides it with "spec", the second confirms.
	s.DriverReassessChoices = []string{"spec"}

	sc := runScenario(t, "Build the thing.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should finish after the corrected course")
	}
	if s.driverCalls != 4 {
		t.Fatalf("driver calls = %d, want 4 (finish, reassess->spec, finish, reassess->confirm)", s.driverCalls)
	}
	if len(st.History) != 2 {
		t.Fatalf("history = %d entries, want 2: %+v", len(st.History), st.History)
	}

	// Turn 1 records the FINAL decision (spec), with the initial finish
	// decision preserved by reference.
	h1 := st.History[0]
	if h1.Subworkflow != "spec" {
		t.Fatalf("history 1 subworkflow = %q, want spec", h1.Subworkflow)
	}
	if h1.FinishDecisionPath == "" {
		t.Fatal("history 1 missing the initial finish decision path")
	}
	if h1.FinishConfirmed {
		t.Error("history 1 must not mark the finish as confirmed")
	}
	initial, err := os.ReadFile(h1.FinishDecisionPath)
	if err != nil {
		t.Fatalf("initial finish decision missing: %v", err)
	}
	var idd map[string]any
	if err := json.Unmarshal(initial, &idd); err != nil {
		t.Fatalf("initial finish decision not JSON: %v", err)
	}
	if idd["subworkflow"] != "finish" {
		t.Errorf("initial finish decision subworkflow = %v, want finish", idd["subworkflow"])
	}
	final, err := os.ReadFile(h1.DecisionPath)
	if err != nil {
		t.Fatalf("reassessment decision missing: %v", err)
	}
	var fdd map[string]any
	if err := json.Unmarshal(final, &fdd); err != nil {
		t.Fatalf("reassessment decision not JSON: %v", err)
	}
	if fdd["subworkflow"] != "spec" {
		t.Errorf("reassessment decision subworkflow = %v, want spec", fdd["subworkflow"])
	}
	if fdd["decision"] != "select_subworkflow" {
		t.Errorf("reassessment decision branch = %v, want select_subworkflow", fdd["decision"])
	}

	// The spec subworkflow actually ran: its documents exist.
	sawPM, sawPMReview := false, false
	for i := range st.Artifacts {
		switch st.Artifacts[i].Agent {
		case "product_manager":
			sawPM = true
		case "pm_review":
			sawPMReview = true
		}
	}
	if !sawPM || !sawPMReview {
		t.Errorf("spec documents missing from artifacts: %+v", st.Artifacts)
	}

	// Turn 2: finish confirmed on reassessment.
	h2 := st.History[1]
	if h2.Subworkflow != "finish" || h2.Outcome != "finished" {
		t.Fatalf("history 2 = %+v, want finish/finished", h2)
	}
	if h2.FinishDecisionPath == "" || !h2.FinishConfirmed {
		t.Errorf("history 2 must record the confirmed finish: %+v", h2)
	}
	if st.Outcome != "Scripted rationale for finish" {
		t.Errorf("outcome = %q, want the confirmation rationale", st.Outcome)
	}

	// The authoritative run state is stated in every driver prompt.
	first := sc.promptFor(t, "workflow_driver", 1)
	if !strings.Contains(first, "RUN STATE: BOOTSTRAP") {
		t.Error("first driver prompt missing the bootstrap run state")
	}
	reassess := sc.promptFor(t, "workflow_driver", 2)
	if !strings.Contains(reassess, finishReassessmentMarker) {
		t.Fatal("second driver prompt is not a finish reassessment")
	}
	if !strings.Contains(reassess, "Scripted rationale for finish") ||
		!strings.Contains(reassess, "Scripted task: cover the auth domain.") {
		t.Error("reassessment prompt missing the driver's full finish response")
	}
	if !strings.Contains(reassess, "RUN STATE: BOOTSTRAP") {
		t.Error("reassessment prompt missing the bootstrap run state")
	}
	assertNoArtifactPaths(t, reassess, st)
	second := sc.promptFor(t, "workflow_driver", 3)
	if !strings.Contains(second, "RUN STATE: 1 completed workflow execution(s)") {
		t.Error("turn-2 driver prompt missing the completed-execution run state")
	}
	if !strings.Contains(second, "the mandatory finish reassessment overrode it") {
		t.Error("turn-2 decision history missing the overridden-finish note")
	}
}

func TestOrchestrator_FinishReassessmentConfirms(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"classify", "finish"}

	sc := runScenario(t, "Classify it.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should be finished")
	}
	if s.driverCalls != 3 {
		t.Fatalf("driver calls = %d, want 3 (classify, finish, reassessment-confirm)", s.driverCalls)
	}
	last := st.History[len(st.History)-1]
	if last.Subworkflow != "finish" || last.Outcome != "finished" {
		t.Fatalf("last history = %+v, want finish/finished", last)
	}
	if last.FinishDecisionPath == "" {
		t.Fatal("confirmed finish must still record the initial finish decision path")
	}
	if !last.FinishConfirmed {
		t.Error("last history must mark the finish as confirmed")
	}
	initial, err := os.ReadFile(last.FinishDecisionPath)
	if err != nil {
		t.Fatalf("initial finish decision missing: %v", err)
	}
	var idd map[string]any
	if err := json.Unmarshal(initial, &idd); err != nil {
		t.Fatalf("initial finish decision not JSON: %v", err)
	}
	if idd["subworkflow"] != "finish" {
		t.Errorf("initial finish decision subworkflow = %v, want finish", idd["subworkflow"])
	}
	conf, err := os.ReadFile(last.DecisionPath)
	if err != nil {
		t.Fatalf("confirmation decision missing: %v", err)
	}
	var cdd map[string]any
	if err := json.Unmarshal(conf, &cdd); err != nil {
		t.Fatalf("confirmation decision not JSON: %v", err)
	}
	if cdd["decision"] != "confirm_finish" || cdd["subworkflow"] != "finish" {
		t.Errorf("confirmation decision = %v, want confirm_finish/finish", cdd)
	}

	reassess := sc.promptFor(t, "workflow_driver", 3)
	if !strings.Contains(reassess, finishReassessmentMarker) {
		t.Fatal("third driver prompt is not a finish reassessment")
	}
	if !strings.Contains(reassess, "Scripted rationale for finish") {
		t.Error("reassessment prompt missing the driver's full finish response")
	}
	if !strings.Contains(reassess, "RUN STATE: 1 completed workflow execution(s)") {
		t.Error("reassessment prompt missing the completed-execution run state")
	}
	assertNoArtifactPaths(t, reassess, st)
}

// Pins the reported incident: on a bootstrap run the driver's reassessment
// response declared confirm_finish while its own rationale argued that the
// only valid choice was spec. The orchestrator must reject a finish
// confirmation in bootstrap state (the run cannot be complete before it has
// started) and force a corrected response.
func TestOrchestrator_FinishReassessmentBootstrapConfirmRejected(t *testing.T) {
	s := newScript(t)
	// First reassessment: the contradictory confirmation, as observed.
	s.DriverReassessRaw = []string{
		`{"decision":"confirm_finish","subworkflow":"finish","rationale":"The run is in BOOTSTRAP with zero completed executions. Per RULE 1 the only valid choice in bootstrap is the spec subworkflow.","task":"Produce the durable refined task specification from the user's request."}`,
	}
	// Second reassessment (after rejection feedback): the corrected choice.
	s.DriverReassessChoices = []string{"spec"}

	sc := runScenario(t, "Build the thing.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should finish after the corrected course")
	}
	if s.driverCalls != 5 {
		t.Fatalf("driver calls = %d, want 5 (finish, contradictory-confirm rejected, spec, finish, confirm)", s.driverCalls)
	}
	if len(st.History) != 2 {
		t.Fatalf("history = %d entries, want 2: %+v", len(st.History), st.History)
	}

	h1 := st.History[0]
	if h1.Subworkflow != "spec" {
		t.Fatalf("history 1 subworkflow = %q, want spec", h1.Subworkflow)
	}
	if h1.FinishDecisionPath == "" || h1.FinishConfirmed {
		t.Errorf("history 1 must record the overridden finish: %+v", h1)
	}
	final, err := os.ReadFile(h1.DecisionPath)
	if err != nil {
		t.Fatalf("reassessment decision missing: %v", err)
	}
	var fdd map[string]any
	if err := json.Unmarshal(final, &fdd); err != nil {
		t.Fatalf("reassessment decision not JSON: %v", err)
	}
	if fdd["decision"] != "select_subworkflow" || fdd["subworkflow"] != "spec" {
		t.Errorf("persisted reassessment = %v, want select_subworkflow/spec", fdd)
	}

	// The rejection fed back the structural reason; the corrected response
	// followed. (Prompt 2 is the reassessment attempt that received the
	// contradictory response; prompt 3 carries the retry feedback.)
	retryPrompt := sc.promptFor(t, "workflow_driver", 3)
	if !strings.Contains(retryPrompt, "<feedback>") {
		t.Fatal("third driver prompt missing retry feedback")
	}
	if !strings.Contains(retryPrompt, "confirm_finish is impossible in BOOTSTRAP") {
		t.Error("retry feedback missing the bootstrap rejection reason")
	}

	// The spec subworkflow actually ran.
	sawPM, sawPMReview := false, false
	for i := range st.Artifacts {
		switch st.Artifacts[i].Agent {
		case "product_manager":
			sawPM = true
		case "pm_review":
			sawPMReview = true
		}
	}
	if !sawPM || !sawPMReview {
		t.Errorf("spec documents missing from artifacts: %+v", st.Artifacts)
	}
}
