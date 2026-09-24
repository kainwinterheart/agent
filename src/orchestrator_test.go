package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
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
	phase  string // driver phase: "read", "analyze", "respond", "reassess"; "" for other agents
	prompt string
}

// script drives the mock codex runtime. Decision agents (non-nil schema) get
// scripted JSON responses; content agents get scripted documents written to
// the pregenerated output path found in their prompt.
type script struct {
	t *testing.T

	// Driver scenario.
	DriverChoices         []string // consumed in order, one per valid driver response
	DriverNormalRaw       []string // raw JSON regular-turn responses, consumed before DriverChoices
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

	// Multi-turn driver scenario.
	SkipAnalysis  int  // first N analyze calls write no analysis file (0 = off)
	NoSessions    bool // the runtime reports no session id (fallback mode)
	FailReadFirst int  // first N read-inputs calls fail with a runtime error

	// Multi-turn loop decider scenario.
	SkipDeciderAnalysis int // first N decider analyze calls write no file (0 = off)

	driverCalls    int
	choiceIdx      int
	normalRawIdx   int
	reassessIdx    int
	reassessRawIdx int
	loopCalls      int

	subdir        string
	indexSnapshot map[string]string // agent -> artifact index contents when it ran

	prompts []recordedPrompt

	// Session simulation for the multi-turn invocations (driver and loop
	// decider).
	sessionsUsed    []string // session id received by each driver call, in order
	driverSchemas   []bool   // whether each driver call carried an output schema
	currentSession  string   // driver session minted by the most recent read call
	turnCount       int
	bootstrap       bool     // driver run state of the current turn (from the read prompt)
	deciderSessions []string // session id received by each decider call, in order
	deciderSchemas  []bool   // whether each decider call carried an output schema
	deciderSession  string   // decider session minted by the most recent read call
	deciderTurns    int
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

// driverPhaseOf identifies which phase of the driver's multi-turn
// invocation a prompt belongs to, keyed by the same markers the
// orchestrator's prompts carry.
func driverPhaseOf(prompt string) string {
	switch {
	case strings.Contains(prompt, finishReassessmentMarker):
		return "reassess"
	case strings.Contains(prompt, driverAnalyzeMarker):
		return "analyze"
	case strings.Contains(prompt, driverReadMarker):
		return "read"
	default:
		return "respond"
	}
}

// driverPhase simulates one call of the driver's multi-turn conversation:
// the read call mints a fresh session id; every later call of the turn must
// arrive resuming that exact session (asserted) and returns it.
func (s *script) driverPhase(phase, prompt, sessionID string) (string, string, error) {
	switch phase {
	case "read":
		if s.FailReadFirst > 0 {
			s.FailReadFirst--
			return "", "", fmt.Errorf("simulated read-inputs failure")
		}
		s.turnCount++
		if s.NoSessions {
			s.currentSession = ""
		} else {
			s.currentSession = fmt.Sprintf("mock-session-%d", s.turnCount)
		}
		s.bootstrap = strings.Contains(prompt, bootstrapRunStatePrefix)
		return "Inputs read: the artifact index and the documents it lists for this turn.", s.currentSession, nil
	case "analyze":
		s.assertDriverSession(phase, sessionID)
		if s.SkipAnalysis > 0 {
			s.SkipAnalysis--
			return "Analysis saved.", s.currentSession, nil
		}
		path := extractOutputPath(s.t, prompt)
		if err := os.WriteFile(path, []byte("# Driver analysis\n\nScripted analysis: state read, procedure walked, transition forced.\n"), 0o644); err != nil {
			s.t.Fatalf("mock: cannot write analysis %s: %v", path, err)
		}
		return "Analysis saved.", s.currentSession, nil
	case "respond":
		s.assertDriverSession(phase, sessionID)
		out, err := s.driverResponse()
		return out, s.currentSession, err
	case "reassess":
		s.assertDriverSession(phase, sessionID)
		out, err := s.reassessResponse()
		return out, s.currentSession, err
	}
	s.t.Fatalf("unknown driver phase")
	return "", "", nil
}

// assertDriverSession pins the multi-turn design: every call after the
// read-inputs call of a turn resumes the session that call acquired.
func (s *script) assertDriverSession(phase, sessionID string) {
	if s.currentSession != "" && sessionID != s.currentSession {
		s.t.Errorf("driver %s call did not resume the acquired session: got %q, want %q", phase, sessionID, s.currentSession)
	}
}

// deciderPhaseOf identifies which phase of the loop decider's multi-turn
// invocation a prompt belongs to, keyed by the same markers the
// orchestrator's prompts carry.
func deciderPhaseOf(prompt string) string {
	switch {
	case strings.Contains(prompt, loopDeciderAnalyzeMarker):
		return "analyze"
	case strings.Contains(prompt, loopDeciderReadMarker):
		return "read"
	default:
		return "respond"
	}
}

// deciderPhase simulates one call of the loop decider's multi-turn
// conversation: the read call mints a fresh session id; every later call of
// the turn must arrive resuming that exact session (asserted) and returns
// it. The decider has no reassessment phase.
func (s *script) deciderPhase(phase, prompt, sessionID string) (string, string, error) {
	switch phase {
	case "read":
		s.deciderTurns++
		s.deciderSession = fmt.Sprintf("mock-decider-session-%d", s.deciderTurns)
		return "Inputs read: this execution's index section and the round's documents.", s.deciderSession, nil
	case "analyze":
		if s.deciderSession != "" && sessionID != s.deciderSession {
			s.t.Errorf("loop decider %s call did not resume the acquired session: got %q, want %q", phase, sessionID, s.deciderSession)
		}
		if s.SkipDeciderAnalysis > 0 {
			s.SkipDeciderAnalysis--
			return "Analysis saved.", s.deciderSession, nil
		}
		path := extractOutputPath(s.t, prompt)
		if err := os.WriteFile(path, []byte("# Loop decider analysis\n\nScripted analysis: round documents studied, verdict forced.\n"), 0o644); err != nil {
			s.t.Fatalf("mock: cannot write decider analysis %s: %v", path, err)
		}
		return "Analysis saved.", s.deciderSession, nil
	case "respond":
		if s.deciderSession != "" && sessionID != s.deciderSession {
			s.t.Errorf("loop decider %s call did not resume the acquired session: got %q, want %q", phase, sessionID, s.deciderSession)
		}
		out, err := s.loopResponse()
		return out, s.deciderSession, err
	}
	s.t.Fatalf("unknown loop decider phase")
	return "", "", nil
}

// driverResponse simulates the respond phase (call 3): the JSON decision.
// Raw scripted responses first (verbatim, for simulating protocol
// violations), then intents rendered protocol-compliantly - a "finish"
// intent in bootstrap becomes "spec", since a compliant driver never emits
// finish before the run has started.
func (s *script) driverResponse() (string, error) {
	s.driverCalls++
	if s.DriverFailAfter > 0 && s.driverCalls > s.DriverFailAfter {
		return "", fmt.Errorf("simulated driver failure")
	}
	if s.driverCalls <= s.DriverBadFirst {
		return `{"subworkflow":"classify","rationale":"broken","unexpected_field":true}`, nil
	}
	if s.normalRawIdx < len(s.DriverNormalRaw) {
		raw := s.DriverNormalRaw[s.normalRawIdx]
		s.normalRawIdx++
		return raw, nil
	}
	choice := "finish"
	if s.choiceIdx < len(s.DriverChoices) {
		choice = s.DriverChoices[s.choiceIdx]
		s.choiceIdx++
	}
	if choice == "finish" && s.bootstrap {
		choice = "spec"
	}
	resp := map[string]any{
		"subworkflow": choice,
		"rationale":   "Scripted rationale for " + choice,
		"task":        "Scripted task: cover the auth domain.",
	}
	return prettyJSON(resp), nil
}

// reassessResponse simulates the mandatory finish reassessment (call 4): it
// consumes the separate scripted queue (default: confirm the finish).
func (s *script) reassessResponse() (string, error) {
	s.driverCalls++
	if s.DriverFailAfter > 0 && s.driverCalls > s.DriverFailAfter {
		return "", fmt.Errorf("simulated driver failure")
	}
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
	if choice == "finish" && s.bootstrap {
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

func (s *script) codexHook(agentName, prompt string, schema map[string]any, sessionID string) (string, string, error) {
	phase := ""
	switch agentName {
	case "workflow_driver":
		phase = driverPhaseOf(prompt)
		s.sessionsUsed = append(s.sessionsUsed, sessionID)
		s.driverSchemas = append(s.driverSchemas, schema != nil)
	case "loop_decider":
		phase = deciderPhaseOf(prompt)
		s.deciderSessions = append(s.deciderSessions, sessionID)
		s.deciderSchemas = append(s.deciderSchemas, schema != nil)
	}
	s.prompts = append(s.prompts, recordedPrompt{agentName, phase, prompt})

	if agentName == "workflow_driver" {
		return s.driverPhase(phase, prompt, sessionID)
	}
	if agentName == "loop_decider" {
		return s.deciderPhase(phase, prompt, sessionID)
	}

	if s.indexSnapshot[agentName] == "" {
		if data, err := os.ReadFile(indexPath(s.subdir)); err == nil {
			s.indexSnapshot[agentName] = string(data)
		}
	}

	if s.FailFirst[agentName] > 0 {
		s.FailFirst[agentName]--
		return "", "", fmt.Errorf("simulated runtime failure for %s", agentName)
	}

	path := extractOutputPath(s.t, prompt)
	if s.SkipFile[agentName] {
		return "ack", "", nil
	}
	if s.EmptyFileFirst[agentName] {
		s.EmptyFileFirst[agentName] = false
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			s.t.Fatalf("mock: cannot write %s: %v", path, err)
		}
		return "ack", "", nil
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
	return "ack", "", nil
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

// promptForPhase returns the nth prompt of one phase of the driver's
// multi-turn invocation (read/analyze/respond/reassess).
func (sc *scenario) promptForPhase(t *testing.T, agent, phase string, nth int) string {
	t.Helper()
	seen := 0
	for i := range sc.script.prompts {
		if sc.script.prompts[i].agent == agent && sc.script.prompts[i].phase == phase {
			seen++
			if seen == nth {
				return sc.script.prompts[i].prompt
			}
		}
	}
	t.Fatalf("no %s prompt #%d recorded for agent %s", phase, nth, agent)
	return ""
}

func (sc *scenario) promptCountPhase(t *testing.T, agent, phase string) int {
	t.Helper()
	n := 0
	for i := range sc.script.prompts {
		if sc.script.prompts[i].agent == agent && sc.script.prompts[i].phase == phase {
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

	// The read-inputs call (first driver prompt of the turn) carries the
	// full state bundle: subworkflow menu and progress marker.
	driverPrompt := sc.promptForPhase(t, "workflow_driver", "read", 1)
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

	// Both decider turns' read-inputs calls are pointed at the index; no
	// document path is inlined.
	dec1 := sc.promptForPhase(t, "loop_decider", "read", 1)
	dec2 := sc.promptForPhase(t, "loop_decider", "read", 2)
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

	first := sc.promptForPhase(t, "workflow_driver", "respond", 1)
	if strings.Contains(first, "<feedback>") {
		t.Error("first respond prompt must not carry retry feedback")
	}
	second := sc.promptForPhase(t, "workflow_driver", "respond", 2)
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
	second := sc.promptForPhase(t, "loop_decider", "respond", 2)
	if !strings.Contains(second, "<feedback>") || !strings.Contains(second, "missing required field") {
		t.Error("second decider respond prompt should carry strict-validation feedback naming the problem")
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
	if s.driverCalls != 3 {
		t.Fatalf("driver calls = %d, want 3 (spec, finish, reassess->confirm)", s.driverCalls)
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
	if s.driverCalls != 3 {
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

	// Both decider turns' read-inputs calls are pointed at the index, and
	// the final index lists all six documents of the two rounds.
	dec1 := sc.promptForPhase(t, "loop_decider", "read", 1)
	dec2 := sc.promptForPhase(t, "loop_decider", "read", 2)
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

	read2 := sc.promptForPhase(t, "workflow_driver", "read", 2)
	if !strings.Contains(read2, indexPath(sc.subdir)) {
		t.Error("driver read prompt missing the artifact index path")
	}
	if strings.Contains(read2, "ARTIFACT REGISTRY") {
		t.Error("driver prompt must not carry the old artifact registry")
	}
	assertNoArtifactPaths(t, read2, st)
	// Decision history still names the tasks and the artifact ids.
	if !strings.Contains(read2, "task: Scripted task: cover the auth domain.") {
		t.Error("decision history missing the driver task")
	}
	if !strings.Contains(read2, st.Artifacts[0].ID) {
		t.Error("decision history should still name the artifact ids")
	}
}

func TestDeciderPrompt_HasIndexPath(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}

	sc := runScenario(t, "Fix the login bug.", s, nil)
	st := sc.state(t)

	dec := sc.promptForPhase(t, "loop_decider", "read", 1)
	if !strings.Contains(dec, indexPath(sc.subdir)) {
		t.Error("decider read prompt missing the artifact index path")
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

// The reassessment backstop, with the source guard in the same run: turn 1
// (bootstrap) answers a premature "finish" that the orchestrator rejects at
// validation time and corrects; turn 2 (real work done) answers "finish"
// again, and the mandatory reassessment overrides it with the subworkflow
// that was actually needed.
func TestOrchestrator_FinishReassessmentOverridesFinish(t *testing.T) {
	s := newScript(t)
	// Turn 1, attempt 1 (bootstrap): a premature finish, verbatim.
	s.DriverNormalRaw = []string{
		`{"subworkflow":"finish","rationale":"The classification is done; the requested outcome is complete.","task":"The requested implementation has been completed and passed the required final reviews."}`,
	}
	// Turn 1, attempt 2 (after rejection feedback): classify.
	// Turn 2: a premature finish (non-bootstrap, so admissible at validation).
	s.DriverChoices = []string{"classify", "finish"}
	// The turn-2 reassessment corrects the course.
	s.DriverReassessChoices = []string{"spec"}

	sc := runScenario(t, "Build the thing.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should finish after the corrected course")
	}
	if s.driverCalls != 6 {
		t.Fatalf("driver calls = %d, want 6 (finish rejected, classify, finish, reassess->spec, finish, reassess->confirm)", s.driverCalls)
	}
	if len(st.History) != 3 {
		t.Fatalf("history = %d entries, want 3: %+v", len(st.History), st.History)
	}

	// Turn 1: the rejected bootstrap finish never reached the reassessment.
	h1 := st.History[0]
	if h1.Subworkflow != "classify" {
		t.Fatalf("history 1 subworkflow = %q, want classify", h1.Subworkflow)
	}
	if h1.FinishDecisionPath != "" || h1.FinishConfirmed {
		t.Errorf("history 1 must not reference a reassessment: %+v", h1)
	}
	retryPrompt := sc.promptForPhase(t, "workflow_driver", "respond", 2)
	if !strings.Contains(retryPrompt, "<feedback>") ||
		!strings.Contains(retryPrompt, "finish is impossible in BOOTSTRAP") {
		t.Error("turn-1 retry prompt missing the bootstrap rejection feedback")
	}

	// Turn 2 records the FINAL decision (spec), with the initial finish
	// decision preserved by reference.
	h2 := st.History[1]
	if h2.Subworkflow != "spec" {
		t.Fatalf("history 2 subworkflow = %q, want spec", h2.Subworkflow)
	}
	if h2.FinishDecisionPath == "" {
		t.Fatal("history 2 missing the initial finish decision path")
	}
	if h2.FinishConfirmed {
		t.Error("history 2 must not mark the finish as confirmed")
	}
	initial, err := os.ReadFile(h2.FinishDecisionPath)
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
	final, err := os.ReadFile(h2.DecisionPath)
	if err != nil {
		t.Fatalf("reassessment decision missing: %v", err)
	}
	var fdd map[string]any
	if err := json.Unmarshal(final, &fdd); err != nil {
		t.Fatalf("reassessment decision not JSON: %v", err)
	}
	if fdd["subworkflow"] != "spec" || fdd["decision"] != "select_subworkflow" {
		t.Errorf("reassessment decision = %v, want select_subworkflow/spec", fdd)
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

	// Turn 3: finish confirmed on reassessment.
	h3 := st.History[2]
	if h3.Subworkflow != "finish" || h3.Outcome != "finished" || !h3.FinishConfirmed {
		t.Errorf("history 3 must record the confirmed finish: %+v", h3)
	}

	// The turn-2 reassessment prompt carried the driver's FULL finish
	// response; the turn-2 read prompt carried the completed-execution run
	// state (in session mode the state bundle is stated once per turn, on
	// the read-inputs call).
	reassess := sc.promptForPhase(t, "workflow_driver", "reassess", 1)
	if !strings.Contains(reassess, finishReassessmentMarker) {
		t.Fatal("reassessment prompt is not a finish reassessment")
	}
	// (The turn-2 finish is the mock's standard scripted response; the raw
	// response was consumed - and rejected - on turn 1.)
	if !strings.Contains(reassess, "Scripted rationale for finish") ||
		!strings.Contains(reassess, "Scripted task: cover the auth domain.") {
		t.Error("reassessment prompt missing the driver's full finish response")
	}
	assertNoArtifactPaths(t, reassess, st)
	read2 := sc.promptForPhase(t, "workflow_driver", "read", 2)
	if !strings.Contains(read2, "RUN STATE: 1 completed workflow execution(s)") {
		t.Error("turn-2 read prompt missing the completed-execution run state")
	}
	// The turn-3 read prompt's decision history carries the overridden-
	// finish note and the updated run state.
	turn3 := sc.promptForPhase(t, "workflow_driver", "read", 3)
	if !strings.Contains(turn3, "RUN STATE: 2 completed workflow execution(s)") {
		t.Error("turn-3 driver prompt missing the completed-execution run state")
	}
	if !strings.Contains(turn3, "the mandatory finish reassessment overrode it") {
		t.Error("turn-3 decision history missing the overridden-finish note")
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

	reassess := sc.promptForPhase(t, "workflow_driver", "reassess", 1)
	if !strings.Contains(reassess, finishReassessmentMarker) {
		t.Fatal("reassessment prompt is not a finish reassessment")
	}
	if !strings.Contains(reassess, "Scripted rationale for finish") {
		t.Error("reassessment prompt missing the driver's full finish response")
	}
	assertNoArtifactPaths(t, reassess, st)
	// The run state is stated on the turn's read-inputs call.
	read2 := sc.promptForPhase(t, "workflow_driver", "read", 2)
	if !strings.Contains(read2, "RUN STATE: 1 completed workflow execution(s)") {
		t.Error("turn-2 read prompt missing the completed-execution run state")
	}
}

// Pins the reported incident at its source: on a bootstrap run the driver's
// regular turn answered "finish" while its own rationale stated that finish
// is invalid. The orchestrator rejects a bootstrap finish at validation time
// (before any reassessment round trip) and forces the corrected response.
func TestOrchestrator_BootstrapFinishRejectedAtSource(t *testing.T) {
	s := newScript(t)
	// Regular turn 1: the verbatim contradictory response, as observed in
	// production (decisions/001-driver.md).
	s.DriverNormalRaw = []string{
		`{"subworkflow":"finish","rationale":"Run state is BOOTSTRAP with zero completed workflow executions. Per RULE 1, the first driver decision must be the spec subworkflow; finish is invalid.","task":"The requested implementation has been completed and passed the required final reviews."}`,
	}

	sc := runScenario(t, "Build the thing.", s, nil)
	st := sc.state(t)

	if !st.Finished {
		t.Fatal("run should finish after the corrected course")
	}
	if s.driverCalls != 4 {
		t.Fatalf("driver calls = %d, want 4 (raw finish rejected, spec, finish, reassess->confirm)", s.driverCalls)
	}
	if len(st.History) != 2 {
		t.Fatalf("history = %d entries, want 2: %+v", len(st.History), st.History)
	}

	h1 := st.History[0]
	if h1.Subworkflow != "spec" {
		t.Fatalf("history 1 subworkflow = %q, want spec", h1.Subworkflow)
	}
	if h1.FinishDecisionPath != "" || h1.FinishConfirmed {
		t.Errorf("history 1 must not reference a reassessment (the finish never reached it): %+v", h1)
	}
	final, err := os.ReadFile(h1.DecisionPath)
	if err != nil {
		t.Fatalf("decision missing: %v", err)
	}
	var fdd map[string]any
	if err := json.Unmarshal(final, &fdd); err != nil {
		t.Fatalf("decision not JSON: %v", err)
	}
	if fdd["subworkflow"] != "spec" {
		t.Errorf("persisted decision subworkflow = %v, want spec", fdd["subworkflow"])
	}

	// The rejection fed back the structural reason; the corrected response
	// followed. (The second respond-phase call is the retry carrying the
	// feedback, in the same session.)
	retryPrompt := sc.promptForPhase(t, "workflow_driver", "respond", 2)
	if !strings.Contains(retryPrompt, "<feedback>") {
		t.Fatal("second driver prompt missing retry feedback")
	}
	if !strings.Contains(retryPrompt, "finish is impossible in BOOTSTRAP") {
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

// ---------------------------------------------------------------------------
// Multi-turn driver invocations
// ---------------------------------------------------------------------------

// Pins the multi-turn design end to end: call 1 of each turn starts a fresh
// session and reads the inputs; calls 2..n of the turn resume that exact
// session; the read/analyze calls carry no output schema while respond and
// reassess carry one; and the session id is never persisted to disk.
func TestDriverTurn_MultiTurnSession(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"spec", "finish"}

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)
	if !st.Finished {
		t.Fatal("run should be finished")
	}

	// Two turns: read, analyze, respond[, reassess].
	wantSessions := []string{"", "mock-session-1", "mock-session-1", "", "mock-session-2", "mock-session-2", "mock-session-2"}
	if len(s.sessionsUsed) != len(wantSessions) {
		t.Fatalf("driver calls = %d, want %d: %v", len(s.sessionsUsed), len(wantSessions), s.sessionsUsed)
	}
	for i, w := range wantSessions {
		if s.sessionsUsed[i] != w {
			t.Errorf("driver call %d session = %q, want %q", i+1, s.sessionsUsed[i], w)
		}
	}
	wantSchemas := []bool{false, false, true, false, false, true, true}
	if len(s.driverSchemas) != len(wantSchemas) {
		t.Fatalf("driver schema flags = %v, want %v", s.driverSchemas, wantSchemas)
	}
	for i, w := range wantSchemas {
		if s.driverSchemas[i] != w {
			t.Errorf("driver call %d hasSchema = %v, want %v", i+1, s.driverSchemas[i], w)
		}
	}

	// The session id is kept solely in memory: no file in the session
	// directory carries it (state, decisions, analysis, index, summary).
	for _, id := range []string{"mock-session-1", "mock-session-2"} {
		err := filepath.WalkDir(sc.subdir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if strings.Contains(string(data), id) {
				t.Errorf("session id %q persisted to disk: %s", id, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk session dir: %v", err)
		}
	}
}

// Pins phase 2: the driver saves its analysis and reasoning to the
// pregenerated file, the orchestrator verifies it on disk, and the respond
// phase is pointed at it. The analysis file is a control-plane document: it
// is not registered in the artifact index.
func TestDriverTurn_AnalysisSaved(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"spec", "finish"}

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)

	// One driver analysis file per driver turn (two turns in this run);
	// loop decider analysis files live alongside, under their own label.
	files, err := filepath.Glob(filepath.Join(sc.subdir, "analysis", "*-driver.md"))
	if err != nil {
		t.Fatalf("glob analysis: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("driver analysis files = %d, want 2: %v", len(files), files)
	}
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatalf("analysis file missing: %v", err)
		}
		if info.Size() == 0 {
			t.Errorf("analysis file %s is empty", f)
		}
	}

	// The respond phase of each turn references an analysis file, and no
	// analysis file is registered as an artifact.
	for n := 1; n <= 2; n++ {
		respond := sc.promptForPhase(t, "workflow_driver", "respond", n)
		found := false
		for _, f := range files {
			if strings.Contains(respond, f) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("respond prompt %d does not reference any analysis file: %v", n, files)
		}
	}
	for i := range st.Artifacts {
		if strings.HasPrefix(st.Artifacts[i].Path, filepath.Join(sc.subdir, "analysis")) {
			t.Errorf("analysis file registered as an artifact: %s", st.Artifacts[i].Path)
		}
	}
}

// Pins the phase-2 retry: when the analysis file is missing after an
// attempt, the orchestrator retries within the SAME session until the file
// exists on disk.
func TestDriverTurn_AnalysisMissingRetriedInSession(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"spec", "finish"}
	s.SkipAnalysis = 1

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)
	if !st.Finished {
		t.Fatal("run should be finished despite the missing first analysis")
	}

	// Turn 1: read, analyze (no file), analyze (file), respond - the whole
	// turn in the one session minted by the read call; call 5 is turn 2's
	// read, which mints a fresh session.
	want := []string{"", "mock-session-1", "mock-session-1", "mock-session-1", ""}
	if len(s.sessionsUsed) < len(want) {
		t.Fatalf("driver calls = %d, want at least %d: %v", len(s.sessionsUsed), len(want), s.sessionsUsed)
	}
	for i, w := range want {
		if s.sessionsUsed[i] != w {
			t.Errorf("driver call %d session = %q, want %q", i+1, s.sessionsUsed[i], w)
		}
	}
	if got := sc.promptCountPhase(t, "workflow_driver", "analyze"); got != 3 {
		t.Fatalf("analyze calls = %d, want 3 (2 for turn 1, 1 for turn 2)", got)
	}
	// The retry carried the validation feedback naming the missing file.
	retry := sc.promptForPhase(t, "workflow_driver", "analyze", 2)
	if !strings.Contains(retry, "<feedback>") || !strings.Contains(retry, "does not exist") {
		t.Error("analyze retry prompt missing the missing-file feedback")
	}
}

// Pins the fallback: when the runtime reports no session id, the turn
// degrades to self-contained one-shot calls - every phase prompt carries
// the full driver state bundle - and the run still completes.
func TestDriverTurn_NoSessionFallback(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"spec", "finish"}
	s.NoSessions = true

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)
	if !st.Finished {
		t.Fatal("run should be finished in no-session fallback mode")
	}
	for i, id := range s.sessionsUsed {
		if id != "" {
			t.Errorf("driver call %d session = %q, want empty", i+1, id)
		}
	}
	// Every phase prompt after the read call carries the full driver
	// prompt (role included), so the call is self-contained.
	n := 0
	for i := range s.prompts {
		if s.prompts[i].agent != "workflow_driver" || s.prompts[i].phase == "read" {
			continue
		}
		n++
		if !strings.Contains(s.prompts[i].prompt, "You are the workflow driver") {
			t.Errorf("%s prompt %d missing the full driver prompt in fallback mode", s.prompts[i].phase, n)
		}
	}
	if n == 0 {
		t.Fatal("no post-read driver prompts recorded")
	}
}

// Pins the read-inputs retry: a failed read is retried in a CLEAN session,
// and the turn's later calls resume the session the successful read minted.
func TestDriverTurn_ReadFailureRetriedInCleanSession(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"spec", "finish"}
	s.FailReadFirst = 1

	sc := runScenario(t, "Ship it.", s, nil)
	st := sc.state(t)
	if !st.Finished {
		t.Fatal("run should be finished despite the failed first read")
	}
	// Turn 1: read (fail), read (fresh session), analyze, respond;
	// turn 2: read (fresh session), analyze, respond, reassess.
	want := []string{"", "", "mock-session-1", "mock-session-1", "", "mock-session-2", "mock-session-2", "mock-session-2"}
	if len(s.sessionsUsed) != len(want) {
		t.Fatalf("driver calls = %d, want %d: %v", len(s.sessionsUsed), len(want), s.sessionsUsed)
	}
	for i, w := range want {
		if s.sessionsUsed[i] != w {
			t.Errorf("driver call %d session = %q, want %q", i+1, s.sessionsUsed[i], w)
		}
	}
	if got := sc.promptCountPhase(t, "workflow_driver", "read"); got != 3 {
		t.Errorf("read calls = %d, want 3 (2 for turn 1, 1 for turn 2)", got)
	}
}

// ---------------------------------------------------------------------------
// Multi-turn loop decider invocations
// ---------------------------------------------------------------------------

// Pins the loop decider's multi-turn design: call 1 of each decider turn
// starts a fresh session and reads the inputs; calls 2..3 resume that exact
// session; the read/analyze calls carry no output schema while respond
// carries one; and the session id is never persisted to disk.
func TestLoopDecider_MultiTurnSession(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}
	s.LoopRepeat = []bool{true, false} // two rounds -> two decider turns

	sc := runScenario(t, "Fix the login bug.", s, nil)
	st := sc.state(t)
	if !st.Finished {
		t.Fatal("run should be finished")
	}

	wantSessions := []string{"", "mock-decider-session-1", "mock-decider-session-1", "", "mock-decider-session-2", "mock-decider-session-2"}
	if len(s.deciderSessions) != len(wantSessions) {
		t.Fatalf("decider calls = %d, want %d: %v", len(s.deciderSessions), len(wantSessions), s.deciderSessions)
	}
	for i, w := range wantSessions {
		if s.deciderSessions[i] != w {
			t.Errorf("decider call %d session = %q, want %q", i+1, s.deciderSessions[i], w)
		}
	}
	wantSchemas := []bool{false, false, true, false, false, true}
	if len(s.deciderSchemas) != len(wantSchemas) {
		t.Fatalf("decider schema flags = %v, want %v", s.deciderSchemas, wantSchemas)
	}
	for i, w := range wantSchemas {
		if s.deciderSchemas[i] != w {
			t.Errorf("decider call %d hasSchema = %v, want %v", i+1, s.deciderSchemas[i], w)
		}
	}

	// The session id is kept solely in memory: no file in the session
	// directory carries it.
	for _, id := range []string{"mock-decider-session-1", "mock-decider-session-2"} {
		err := filepath.WalkDir(sc.subdir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if strings.Contains(string(data), id) {
				t.Errorf("session id %q persisted to disk: %s", id, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk session dir: %v", err)
		}
	}
}

// Pins the decider's phase 2: the decider saves its analysis and reasoning
// to the pregenerated file, the orchestrator verifies it on disk, and the
// respond phase is pointed at it. The analysis file is a control-plane
// document: it is not registered in the artifact index.
func TestLoopDecider_AnalysisSaved(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}
	s.LoopRepeat = []bool{true, false}

	sc := runScenario(t, "Fix the login bug.", s, nil)
	st := sc.state(t)

	// One decider analysis file per decider turn (two turns in this run).
	files, err := filepath.Glob(filepath.Join(sc.subdir, "analysis", "*-loop_decider.md"))
	if err != nil {
		t.Fatalf("glob analysis: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("decider analysis files = %d, want 2: %v", len(files), files)
	}
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatalf("analysis file missing: %v", err)
		}
		if info.Size() == 0 {
			t.Errorf("analysis file %s is empty", f)
		}
	}

	// The respond phase of each decider turn references an analysis file,
	// and no analysis file is registered as an artifact.
	for n := 1; n <= 2; n++ {
		respond := sc.promptForPhase(t, "loop_decider", "respond", n)
		found := false
		for _, f := range files {
			if strings.Contains(respond, f) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("decider respond prompt %d does not reference any analysis file: %v", n, files)
		}
	}
	for i := range st.Artifacts {
		if strings.HasPrefix(st.Artifacts[i].Path, filepath.Join(sc.subdir, "analysis")) {
			t.Errorf("analysis file registered as an artifact: %s", st.Artifacts[i].Path)
		}
	}
}

// Pins the decider's phase-2 retry: when the analysis file is missing after
// an attempt, the orchestrator retries within the SAME session until the
// file exists on disk.
func TestLoopDecider_AnalysisMissingRetriedInSession(t *testing.T) {
	s := newScript(t)
	s.DriverChoices = []string{"implement", "finish"}
	s.LoopRepeat = []bool{true, false}
	s.SkipDeciderAnalysis = 1

	sc := runScenario(t, "Fix the login bug.", s, nil)
	st := sc.state(t)
	if !st.Finished {
		t.Fatal("run should be finished despite the missing first decider analysis")
	}

	// Decider turn 1: read, analyze (no file), analyze (file), respond -
	// the whole turn in the one session minted by the read call.
	want := []string{"", "mock-decider-session-1", "mock-decider-session-1", "mock-decider-session-1"}
	if len(s.deciderSessions) < len(want) {
		t.Fatalf("decider calls = %d, want at least %d: %v", len(s.deciderSessions), len(want), s.deciderSessions)
	}
	for i, w := range want {
		if s.deciderSessions[i] != w {
			t.Errorf("decider call %d session = %q, want %q", i+1, s.deciderSessions[i], w)
		}
	}
	if got := sc.promptCountPhase(t, "loop_decider", "analyze"); got != 3 {
		t.Fatalf("decider analyze calls = %d, want 3 (2 for turn 1, 1 for turn 2)", got)
	}
	// The retry carried the validation feedback naming the missing file.
	retry := sc.promptForPhase(t, "loop_decider", "analyze", 2)
	if !strings.Contains(retry, "<feedback>") || !strings.Contains(retry, "does not exist") {
		t.Error("decider analyze retry prompt missing the missing-file feedback")
	}
}
