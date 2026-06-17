// =========================
// EXECUTION FRAMEWORK
// =========================
package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	ac_wman "github.com/autumncoffee/wman-go"
)

// watchmanHook is set by tests to mock watcher.Flush() responses.
var watchmanHook func() map[string]string

// Configuration holds configuration for the unified code execution framework.
type Configuration struct {
	InitialPromptContext string
	CoderAgentRef        *Agent
	ReviewAgentRef       *Agent
	Plan                 map[string]interface{}
	MaxIterations        int
	NSC                  *Agent
}

// CodeExecutionFramework is the unified execution framework for coder+review loop.
type CodeExecutionFramework struct{}

// Execute runs the coder+review loop.
func (c *CodeExecutionFramework) Execute(
	config *Configuration,
	subdir []string,
	invocationIDPrefix string,
	watcher *ac_wman.Watchman,
) string {
	coderOutput, changes := collectChanges(
		config.MaxIterations,
		watcher,
		config.CoderAgentRef,
		config.InitialPromptContext,
		fmt.Sprintf("%s-coder-0", invocationIDPrefix),
		subdir,
	)
	allOutputs := []interface{}{coderOutput}
	allChanges := changes

	for iterCount := 0; iterCount < config.MaxIterations; iterCount++ {
		logStep(fmt.Sprintf("Iteration %d/%d", iterCount+1, config.MaxIterations), "CODER EXECUTION")

		var recentChanges interface{}
		var allChangesList []interface{}
		for _, v := range allOutputs {
			if m, ok := v.(map[string]interface{}); ok {
				changesArr, _ := m["changes"].([]interface{})
				allChangesList = append(allChangesList, changesArr)
				if len(changesArr) > 0 {
					recentChanges = changesArr
				} else {
					recentChanges = m
				}
			}
		}

		reviewPrompt := fmt.Sprintf(
			"TASK:\n%s\nATTEMPT: %d/%d\n\nMOST RECENT CHANGES:\n%s\n%s\nchanges from past iterations for context:\n%s",
			config.InitialPromptContext,
			iterCount+1,
			config.MaxIterations,
			MarshalJSON(recentChanges),
			changesPrompt(changes),
			MarshalJSON(allChangesList[:len(allChangesList)-1]),
		)

		results := Nudge(
			config.MaxIterations,
			config.ReviewAgentRef,
			reviewPrompt,
			fmt.Sprintf("%s-code-review-%d", invocationIDPrefix, iterCount),
			subdir,
			false,
			config.NSC,
		)
		review, _ := results[len(results)-1].(map[string]interface{})

		if reviewOk(review) {
			logStep("Code approved by reviewer", "REVIEW")
			break
		}

		var coderPrompt string
		if shouldReset(review) {
			logStep(fmt.Sprintf("Resetting coder agent context: %s", review["reset_reason"]), "SYSTEM")
			config.CoderAgentRef.Reset(fmt.Sprintf("%s-%d", invocationIDPrefix, iterCount))
			coderPrompt = fmt.Sprintf(
				"PLAN:\n%s\nREVIEW FEEDBACK:\n%s\n\nRe-implement from a clean context using the plan and review feedback. Do not assume prior implementation decisions are correct unless still justified.",
				MarshalJSON(config.Plan),
				MarshalJSON(review),
			)
		} else {
			coderPrompt = fmt.Sprintf("FEEDBACK TO ADDRESS:\n%s", MarshalJSON(review))
		}
		coderOutput, changes = collectChanges(
			config.MaxIterations,
			watcher,
			config.CoderAgentRef,
			coderPrompt,
			fmt.Sprintf("%s-coder-%d", invocationIDPrefix, iterCount+1),
			subdir,
		)
		allOutputs = append(allOutputs, coderOutput)
		for k, v := range changes {
			allChanges[k] = v
		}
	}

	var revisions []string
	for i, v := range allOutputs {
		outMap, _ := v.(map[string]interface{})
		summary, _ := outMap["summary"].(string)
		revisions = append(revisions, fmt.Sprintf("<revision%d>\n%s\n</revision%d>", i+1, summary, i+1))
	}

	return fmt.Sprintf("%s\n%s", strings.Join(revisions, "\n"), changesPrompt(allChanges))
}

// safeFlush calls watcher.Flush() but recovers from any panic (e.g., empty JSON response).
// The underlying wman library panics on unmarshal errors, so we handle it here.
func safeFlush(watcher *ac_wman.Watchman) map[string]string {
	defer func() {
		recover()
	}()
	var out map[string]string
	if watchmanHook != nil {
		out = watchmanHook()
	} else {
		out = watcher.Flush()
	}
	trace("watchman", map[string]interface{}{"changes": out})
	return out
}

func collectChanges(maxIt int, watcher *ac_wman.Watchman, agent *Agent, prompt string, invocationIDPrefix string, subdir []string) (interface{}, map[string]string) {
	// Flush before: clear any existing changes (baseline)
	// Mirrors Python's watcher.wait() + watcher.flush() sequence.
	safeFlush(watcher)

	results := Nudge(
		maxIt,
		agent,
		prompt,
		invocationIDPrefix,
		subdir,
		true,
		nil,
	)
	if len(results) == 0 {
		return nil, make(map[string]string)
	}
	last, _ := results[len(results)-1].(map[string]interface{})
	outMap, _ := last["out"].(map[string]interface{})

	if fromCache, _ := last["from_cache"].(bool); !fromCache && watchmanHook == nil {
		time.Sleep(10 * time.Second)
	}

	// Flush after: get actual filesystem changes since the agent ran
	// Mirrors Python's watcher.flush() called a second time.
	changes := safeFlush(watcher)
	return outMap, changes
}
func changesPrompt(changes map[string]string) string {
	if len(changes) == 0 {
		return "**AUTOMATED VERIFICATION FAILED: NO ACTUAL CHANGES DETECTED**\n"
	}
	out := "AUTOMATED VERIFICATION DETECTED POTENTIAL CHANGES TO FOLLOWING FILES, COMPLETENESS MUST BE ASSESSED:\n"
	names := make([]string, 0, len(changes))
	for name := range changes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out += fmt.Sprintf("* %s: %s\n", changes[name], name)
	}
	return out
}
