package main

import (
	"agent-go/gen"
	"agent-go/pkg/loader"
	td "agent-go/test_data"
	jsonv2text "encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	immutable "github.com/benbjohnson/immutable"
	"os"
	"path/filepath"
)

type CacheResult[T any] struct {
	Out       T
	FromCache bool
}

func RunJSONAgent[T any](agent *Agent[T], inputText string, invocationID string, subdir []string, context *Context) T {
	return runJSONAgent(agent, inputText, invocationID, subdir, context).Out
}

func runJSONAgent[T any](agent *Agent[T], inputText string, invocationID string, subdir []string, context *Context) CacheResult[T] {
	if context.runJSONAgentHook != nil {
		as := context.runJSONAgentHook(agent.Name, invocationID, inputText)
		if as == nil {
			panic("No agent state?")
		}
		agent.Session = as.Session()
		agent.SessionSuffix = as.SessionSuffix()
		if as.LastCorrectResponse() != nil {
			var lcr T
			jsonv2.Unmarshal([]byte(*as.LastCorrectResponse()), &lcr, jsonv2text.AllowDuplicateNames(true))
			agent.LastCorrectResponse = &lcr
		}
	}
	asb := td.NewAgentStateBuilder(nil).WithSession(agent.Session).WithSessionSuffix(agent.SessionSuffix)
	if agent.LastCorrectResponse != nil {
		lcr := MarshalJSON(agent.LastCorrectResponse)
		asb = asb.WithLastCorrectResponse(&lcr)
	}
	context.Tracer.trace("prepare_to_run_agent", td.NewActionDetailsBuilder(nil).WithInvocationId(&invocationID).WithAgent(&agent.Name).WithPrompt(&inputText).WithAgentState(asb.Build()).Build())

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
		raw = agent.Run(inputText, context)
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
`, agent.ResumePrompt, err, loader.SchemaToExample(agent.Schema)), context)
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
`, agent.ResumePrompt, valErr, loader.SchemaToExample(agent.Schema)), context)
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
`, agent.ResumePrompt, err, loader.SchemaToExample(agent.Schema)), context)
			updated = true
			continue
		}
		if agent.Ephemeral {
			agent.Reset(context)
		} else {
			agent.LastCorrectResponse = &out
		}
		return CacheResult[T]{Out: out, FromCache: !updated}
	}
}

type IWithNextStepsBuilder[
	TNextSteps any,
	TObject any,
	TBuilder any,
] interface {
	WithNextSteps(*TNextSteps) TBuilder
	Build() TObject
}

type IWithNextSteps[
	TNextSteps any,
	TObject any,
	TBuilder IWithNextStepsBuilder[
		TNextSteps,
		TObject,
		TBuilder,
	],
] interface {
	NextSteps() *TNextSteps
	Clone() TBuilder
}

func Nudge[
	TNextSteps ~*immutable.List[string],
	TObject IWithNextSteps[
		TNextSteps,
		TObject,
		TBuilder,
	],
	TBuilder IWithNextStepsBuilder[
		TNextSteps,
		TObject,
		TBuilder,
	],
](
	maxIt int,
	agent *Agent[TObject],
	prompt string,
	invocationIDPrefix string,
	subdir []string,
	nsc *Agent[dt.NonCoderNextStepsCleanup],
	context *Context,
) []CacheResult[TObject] {
	nextPrompt := prompt
	results := []CacheResult[TObject]{}
	for i := 0; i < maxIt; i++ {
		result := runJSONAgent(agent, nextPrompt, fmt.Sprintf("%s-nudge%d", invocationIDPrefix, i), subdir, context)

		var nextSteps TNextSteps
		nsPtr := result.Out.NextSteps()
		if nsPtr != nil {
			nextSteps = *nsPtr
			result.Out = result.Out.Clone().WithNextSteps(nil).Build()
			if !agent.Ephemeral {
				agent.LastCorrectResponse = &result.Out
			}
		}
		results = append(results, result)
		if nextSteps == nil || (*immutable.List[string])(nextSteps).Len() == 0 {
			break
		}
		if nsc != nil {
			nscResult := runJSONAgent(nsc, fmt.Sprintf("INPUT:\n%s\n\nReturn the filtered list of steps, exactly as written.\nDo not include any explanation or commentary.", MarshalJSON(map[string]interface{}{"next_steps": immutableListToSlice((*immutable.List[string])(nextSteps))})), fmt.Sprintf("%s-nudge%d-nsc", invocationIDPrefix, i), subdir, context)
			filtered := nscResult.Out.Lines()
			if filtered.Len() == 0 {
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
		}
		nextPrompt += fmt.Sprintf("ITERATION: %d/%d\n", i+1, maxIt)
		nextPrompt += "<feedback>\nADDRESS YOUR NEXT STEPS:\n"
		var s string
		nextStepsItr := (*immutable.List[string])(nextSteps).Iterator()
		nextStepsItr.First()
		for !nextStepsItr.Done() {
			_, s = nextStepsItr.Next()
			nextPrompt += fmt.Sprintf("* %s\n", s)
		}
		nextPrompt += "</feedback>\n"
		if agent.Ephemeral {
			nextPrompt += "\n" + loader.Followup
		}
	}
	return results
}
