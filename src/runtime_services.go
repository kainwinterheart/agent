// =========================
// RUNTIME SERVICES (Tier 3)
// =========================
package main

import (
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
)

// runJSONAgentHook allows overriding runJSONAgent for testing.
var runJSONAgentHook func(agentName, invocationID, prompt string)

// RunJSONAgent runs an agent and returns the parsed JSON result.
func RunJSONAgent(agent *Agent, inputText string, invocationID string, subdir []string) interface{} {
	return runJSONAgent(agent, inputText, invocationID, subdir, false)
}

func runJSONAgent(agent *Agent, inputText string, invocationID string, subdir []string, returnSystemState bool) interface{} {
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
			if err := jsonv2.Unmarshal([]byte(j), &x); err == nil {
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
`, agent.ResumePrompt, err, SchemaToExample(agent.Schema)))
			updated = true
			continue
		}
		var out interface{}
		if err := jsonv2.Unmarshal([]byte(j), &out); err != nil {
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
`, agent.ResumePrompt, err, SchemaToExample(agent.Schema)))
			updated = true
			continue
		}
		if outMap, ok := out.(map[string]interface{}); ok {
			if err := ValidateSchema(outMap, agent.Schema); err != nil {
				errMsg := err.Error()
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
`, agent.ResumePrompt, errMsg, SchemaToExample(agent.Schema)))
				updated = true
				continue
			}
			if agent.Ephemeral {
				agent.Reset()
			} else {
				agent.LastCorrectResponse = outMap
			}
			if returnSystemState {
				return map[string]interface{}{"out": outMap, "from_cache": !updated}
			}
			return outMap
		} else {
			raw = agent.Run(fmt.Sprintf(`
%s

<feedback>
Your previous output was not a JSON object (map). It was a %T with value: %v

Output MUST be a JSON object only:
%s
</feedback>
`, agent.ResumePrompt, out, out, SchemaToExample(agent.Schema)))
			updated = true
			continue
		}
	}
}

// Nudge runs an agent with iterative next-steps prompting.
func Nudge(
	maxIt int,
	agent *Agent,
	prompt string,
	invocationIDPrefix string,
	subdir []string,
	returnSystemState bool,
	nsc *Agent,
) []interface{} {
	nextPrompt := prompt
	results := []interface{}{}
	for i := 0; i < maxIt; i++ {
		result := runJSONAgent(agent, nextPrompt, fmt.Sprintf("%s-nudge%d", invocationIDPrefix, i), subdir, returnSystemState)
		results = append(results, result)
		currentResult := result
		if returnSystemState {
			if m, ok := result.(map[string]interface{}); ok {
				currentResult = m["out"]
			}
		}
		resultMap, _ := currentResult.(map[string]interface{})
		nextSteps, _ := resultMap["next_steps"].([]interface{})

		if nextSteps == nil || len(nextSteps) == 0 {
			break
		}
		if nsc != nil {
			nscResult := runJSONAgent(nsc, fmt.Sprintf("INPUT:\n%s\n\nReturn the filtered list of steps, exactly as written.\nDo not include any explanation or commentary.", MarshalJSON(map[string]interface{}{"next_steps": nextSteps})), fmt.Sprintf("%s-nudge%d-nsc", invocationIDPrefix, i), subdir, false)
			filtered, _ := nscResult.(map[string]interface{})["lines"].([]interface{})
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
			nextPrompt += fmt.Sprintf("PREVIOUS RESPONSE: %s\n", MarshalJSON(currentResult))
		}
		nextPrompt += fmt.Sprintf("ITERATION: %d/%d\n", i+1, maxIt)
		nextPrompt += "<feedback>\nADDRESS YOUR NEXT STEPS:\n"
		for _, ns := range nextSteps {
			if s, ok := ns.(string); ok {
				nextPrompt += fmt.Sprintf("* %s\n", s)
			}
		}
		nextPrompt += "</feedback>\n"
		if agent.Ephemeral {
			nextPrompt += "\n" + FOLLOWUP
		}
	}
	for i := range results {
		r, ok := results[i].(map[string]interface{})
		if !ok {
			continue
		}
		if returnSystemState {
			if out, ok := r["out"].(map[string]interface{}); ok {
				delete(out, "next_steps")
			}
		} else {
			delete(r, "next_steps")
		}
	}
	return results
}
