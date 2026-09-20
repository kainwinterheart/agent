package loader

import (
	"strings"
	"testing"
)

// TestRenderPrompt_UnknownIDPanics pins D-03 (defect register in
// roadmap/2026-09-20_commit_c9831d9_correctness_review.md): rendering a prompt
// id that was never loaded must trip the guard at prompt_loader.go
// (panic "prompt %q not loaded") instead of returning an empty prompt.
func TestRenderPrompt_UnknownIDPanics(t *testing.T) {
	InitPromptLoader()

	const unknownID = PromptID("definitely_not_a_prompt_id")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("RenderPrompt(%q) did not panic", unknownID)
		}
		msg, isString := r.(string)
		if !isString || !strings.Contains(msg, `prompt "definitely_not_a_prompt_id" not loaded`) {
			t.Fatalf("unexpected panic value: %#v", r)
		}
	}()

	RenderPrompt(unknownID, nil)
}
