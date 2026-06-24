package main

import (
	dt "agent-go/gen"
	"agent-go/pkg/loader"
	jsonv2text "encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
)

type CacheResult[T any] struct {
	Out       T
	FromCache bool
}

var runJSONAgentHook func(agentName, invocationID, prompt string)

func RunJSONAgent[T any](agent *Agent[T], inputText string, invocationID string, subdir []string) T {
	return runJSONAgent[T](agent, inputText, invocationID, subdir).Out
}

func runJSONAgent[T any](agent *Agent[T], inputText string, invocationID string, subdir []string) CacheResult[T] {
	if runJSONAgentHook != nil {
		runJSONAgentHook(agent.Name, invocationID, inputText)
	}
	trace("prepare_to_run_agent", map[string]interface{}{
		"invocation_id": invocationID,
		"agent":         agent.Name,
		"prompt":        inputText,
	})

	var raw string
	updated := false
	cacheFile := filepath.Join(BuildPath(subdir, ".state"), fmt.Sprintf("%s_%s.out", agent.Name, invocationID))
	if data, err := os.ReadFile(cacheFile); err == nil {
		logStep(fmt.Sprintf("Using cached response: %s", cacheFile), invocationID)
		raw = string(data)
	}

	if raw == "" {
		stateDir := filepath.Join(BuildPath(subdir, ".state"), fmt.Sprintf("%s_%s.in", agent.Name, invocationID))
		AtomicWrite(stateDir, inputText)
		raw = agent.Run(inputText)
		updated = true
	} else {
		if j, err := ExtractJSON(raw); err == nil {
			var x map[string]interface{}
			if err := jsonv2.Unmarshal([]byte(j), &x, jsonv2text.AllowDuplicateNames(true)); err == nil {
				kludged := false
				if _, ok := x["approved"]; ok {
					if _, ok2 := x["approved_confidence"]; !ok2 {
						x["approved_confidence"] = "low"
						x["approved_reason"] = "backfill"
						x["resolved_issues"] = []interface{}{}
						kludged = true
					}
				}
				if (agent.Name == "tech_lead_final" || x["plan"] != nil) && x["next_steps"] == nil {
					x["next_steps"] = []interface{}{}
					kludged = true
				}
				if kludged {
					raw = MarshalJSON(x)
				}
			}
		}
	}

	for {
		if updated {
			AtomicWrite(cacheFile, raw)
		}
		j, err := ExtractJSON(raw)
		if err != nil {
			raw = agent.Run(fmt.Sprintf(`
%s

<feedback>
Your previous output failed JSON validation:
<error>
%v
</error>

Output MUST be valid JSON only:
%s
</feedback>
`, agent.ResumePrompt, err, loader.SchemaToExample(agent.Schema)))
			updated = true
			continue
		}
		if agent.Schema != nil {
			if valErr := loader.ValidateJSONBytes([]byte(j), agent.Schema); valErr != nil {
				raw = agent.Run(fmt.Sprintf(`
%s

<feedback>
Your previous output failed JSON validation:
<error>
%v
</error>

Output MUST be valid JSON only:
%s
</feedback>
`, agent.ResumePrompt, valErr, loader.SchemaToExample(agent.Schema)))
				updated = true
				continue
			}
		}
		var out T
		if err := jsonv2.Unmarshal([]byte(j), &out, jsonv2text.AllowDuplicateNames(true)); err != nil {
			raw = agent.Run(fmt.Sprintf(`
%s

<feedback>
Your previous output failed JSON validation:
<error>
%v
</error>

Output MUST be valid JSON only:
%s
</feedback>
`, agent.ResumePrompt, err, loader.SchemaToExample(agent.Schema)))
			updated = true
			continue
		}
		if agent.Ephemeral {
			agent.Reset()
		} else {
			agent.LastCorrectResponse = &out
		}
		return CacheResult[T]{Out: out, FromCache: !updated}
	}
}

func Nudge[T any](
	maxIt int,
	agent *Agent[T],
	prompt string,
	invocationIDPrefix string,
	subdir []string,
	nsc *Agent[dt.NonCoderNextStepsCleanupJson],
) []CacheResult[T] {
	nextPrompt := prompt
	results := []CacheResult[T]{}
	for i := 0; i < maxIt; i++ {
		result := runJSONAgent[T](agent, nextPrompt, fmt.Sprintf("%s-nudge%d", invocationIDPrefix, i), subdir)
		results = append(results, result)
		var nextSteps []string
		type nextStepper interface{ NextSteps() []string }
		if ns, ok := any(&result.Out).(nextStepper); ok {
			nextSteps = ns.NextSteps()
		} else {
			panic("Nudge: result type does not implement NextSteps")
		}

		if nextSteps == nil || len(nextSteps) == 0 {
			break
		}
		if nsc != nil {
			nscResult := runJSONAgent(nsc, fmt.Sprintf("INPUT:\n%s\n\nReturn the filtered list of steps, exactly as written.\nDo not include any explanation or commentary.", MarshalJSON(map[string]interface{}{"next_steps": nextSteps})), fmt.Sprintf("%s-nudge%d-nsc", invocationIDPrefix, i), subdir)
			filtered := nscResult.Out.Lines()
			if len(filtered) == 0 {
				break
			}
			nextSteps = filtered
		}
		if agent.Ephemeral || (i+1)%10 == 0 {
			nextPrompt = fmt.Sprintf("END GOAL:\n<reminder>\n%s\n</reminder>\n\n", prompt)
		} else {
			nextPrompt = ""
		}
		if agent.Ephemeral {
			nextPrompt += fmt.Sprintf("PREVIOUS RESPONSE: %s\n", MarshalJSON(result.Out))
		} else {
			agent.LastCorrectResponse = &result.Out
		}
		nextPrompt += fmt.Sprintf("ITERATION: %d/%d\n", i+1, maxIt)
		nextPrompt += "<feedback>\nADDRESS YOUR NEXT STEPS:\n"
		for _, s := range nextSteps {
			nextPrompt += fmt.Sprintf("* %s\n", s)
		}
		nextPrompt += "</feedback>\n"
		if agent.Ephemeral {
			nextPrompt += "\n" + loader.Followup
		}
	}
	return results
}
