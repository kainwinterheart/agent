// =========================
// UTIL
// =========================
package main

import (
	"agent-go/state"
	jsonv2text "encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"github.com/benbjohnson/immutable"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// runCodexHook allows overriding RunCodex for testing.
var runCodexHook func(agentName, session, prompt string, schema map[string]interface{}, timeout string) (string, string, error)

func logStep(msg string, step string) {
	fmt.Fprintf(os.Stderr, ">> [%s] [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), step, msg)
}

// convertNestedImmutables recursively converts any nested immutable.Map and immutable.List
// values inside maps and slices to regular Go types, preventing jsonv2.Marshal from
// encountering unsupported immutable types within nested structures.
func convertNestedImmutables(v interface{}) interface{} {
	switch tv := v.(type) {
	case map[string]interface{}:
		for k, val := range tv {
			tv[k] = convertNestedImmutables(val)
		}
		return tv
	case []interface{}:
		for i, val := range tv {
			tv[i] = convertNestedImmutables(val)
		}
		return tv
	case *immutable.Map[string, interface{}]:
		regular := make(map[string]interface{})
		itr := tv.Iterator()
		itr.First()
		for !itr.Done() {
			k, val, _ := itr.Next()
			regular[k] = convertNestedImmutables(val)
		}
		return regular
	case *immutable.List[interface{}]:
		arr := make([]interface{}, tv.Len())
		itr := tv.Iterator()
		itr.First()
		for i := 0; i < tv.Len(); i++ {
			_, val := itr.Next()
			arr[i] = convertNestedImmutables(val)
		}
		return arr
	case *immutable.Map[string, string]:
		regular := make(map[string]string)
		itr := tv.Iterator()
		itr.First()
		for !itr.Done() {
			k, s, _ := itr.Next()
			regular[k] = s
		}
		return regular
	case *immutable.List[string]:
		arr := make([]string, tv.Len())
		itr := tv.Iterator()
		itr.First()
		for i := 0; i < tv.Len(); i++ {
			_, val := itr.Next()
			arr[i] = val
		}
		return arr
	case *immutable.Map[string, *state.DomainState]:
		regular := make(map[string]*state.DomainState)
		itr := tv.Iterator()
		itr.First()
		for !itr.Done() {
			k, ds, _ := itr.Next()
			regular[k] = ds
		}
		return regular
	}
	return v
}

// MarshalJSON marshals v to a JSON string with deterministic key ordering.
func MarshalJSON(v interface{}) string {
	// Recursively convert any nested immutable types inside maps/slices
	v = convertNestedImmutables(v)
	// Convert immutable types to regular Go types for JSON marshaling
	switch tv := v.(type) {
	case *immutable.Map[string, interface{}]:
		v = state.ToRegularInterfaceMap(tv)
	case *immutable.Map[string, string]:
		if tv == nil {
			return "null"
		}
		regular := make(map[string]string)
		itr := tv.Iterator()
		itr.First()
		for !itr.Done() {
			k, s, _ := itr.Next()
			regular[k] = s
		}
		v = regular
	case *immutable.Map[string, *state.DomainState]:
		if tv == nil {
			return "{}"
		}
		regular := make(map[string]interface{})
		itr := tv.Iterator()
		itr.First()
		for !itr.Done() {
			k, ds, _ := itr.Next()
			regular[k] = ds
		}
		v = regular
	case *immutable.List[interface{}]:
		if tv == nil || tv.Len() == 0 {
			return "[]"
		}
		arr := make([]interface{}, tv.Len())
		itr := tv.Iterator()
		itr.First()
		for i := 0; i < tv.Len(); i++ {
			_, val := itr.Next()
			arr[i] = val
		}
		v = arr
	case *immutable.List[string]:
		if tv == nil || tv.Len() == 0 {
			return "[]"
		}
		arr := make([]string, tv.Len())
		itr := tv.Iterator()
		itr.First()
		for i := 0; i < tv.Len(); i++ {
			_, val := itr.Next()
			arr[i] = val
		}
		v = arr
	case *immutable.List[map[string]interface{}]:
		if tv == nil || tv.Len() == 0 {
			return "[]"
		}
		arr := make([]map[string]interface{}, tv.Len())
		itr := tv.Iterator()
		itr.First()
		for i := 0; i < tv.Len(); i++ {
			_, val := itr.Next()
			arr[i] = val
		}
		v = arr
	}
	b, err := jsonv2.Marshal(v, jsonv2.Deterministic(true))
	if err != nil {
		panic(err)
	}
	return string(b)
}

// RunCodex runs the codex exec command synchronously.
// Returns (output, sessionID, error).
func RunCodex(
	agentName string,
	session string,
	prompt string,
	schema map[string]interface{},
	timeout string,
) (string, string, error) {
	var stdout, result string
	var err error
	if runCodexHook != nil {
		stdout, result, err = runCodexHook(agentName, session, prompt, schema, timeout)
	} else {
		stdout, result, err = realRunCodex(agentName, session, prompt, schema, timeout)
	}
	trace("run_codex", map[string]interface{}{
		"agent_name": agentName,
		"prompt":     prompt,
		"schema":     schema,
		"timeout":    timeout,
		"stdout":     stdout,
	})
	return stdout, result, err
}

// realRunCodex is the actual implementation that spawns the codex subprocess.
func realRunCodex(
	agentName string,
	session string,
	prompt string,
	schema map[string]interface{},
	timeout string,
) (string, string, error) {
	cmdArgs := []string{"codex", "exec"}
	if timeout != "" {
		cmdArgs = append([]string{"timeout", "-s", "9", timeout}, cmdArgs...)
	}

	var schemaPath string
	if os.Getenv("AC_AGENT_NO_SCHEMA") == "" {
		f, err := os.CreateTemp("", "schema-*.json")
		if err != nil {
			panic(err)
		}
		schemaPath = f.Name()
		b := []byte(MarshalJSON(schema))
		(*jsonv2text.Value)(&b).Indent()
		f.Write(b)
		cmdArgs = append(cmdArgs, "--output-schema", schemaPath)
	}
	if session != "" {
		cmdArgs = append(cmdArgs, "resume", session)
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Stdin = strings.NewReader(prompt)
	env := os.Environ()
	env = append(env, fmt.Sprintf("AC_AGENT_NAME=%s", agentName))
	cmd.Env = env

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	var stdoutBuf, stderrBuf strings.Builder
	doneStdout := make(chan struct{})
	doneStderr := make(chan struct{})

	go func() {
		io.Copy(&stdoutBuf, stdoutPipe)
		close(doneStdout)
	}()
	go func() {
		tee := io.TeeReader(stderrPipe, &stderrBuf)
		io.Copy(os.Stderr, tee)
		close(doneStderr)
	}()

	cmd.Wait()
	<-doneStdout
	<-doneStderr

	if schemaPath != "" {
		os.Remove(schemaPath)
	}

	sessionRegex := regexp.MustCompile(`session id:\s*(\S*)`)
	if match := sessionRegex.FindStringSubmatch(stderrBuf.String()); len(match) > 1 {
		session = match[1]
	}

	output := strings.TrimSpace(stdoutBuf.String())
	if output == "" {
		return "", session, fmt.Errorf("Empty output, likely timeout issue")
	}
	return output, session, nil
}

// ExtractJSON extracts the JSON object from text.
// Returns an error if no JSON object is found.
func ExtractJSON(text string) (string, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 {
		return text[start : end+1], nil
	}
	return "", fmt.Errorf("text does not contain a JSON object")
}

// AtomicWrite writes content to a file atomically.
func AtomicWrite(path string, content string) {
	dirPath := filepath.Dir(path)
	if dirPath != "" {
		os.MkdirAll(dirPath, 0o755)
	}
	tmpFile, err := os.CreateTemp(dirPath, "*.tmp")
	if err != nil {
		panic(fmt.Sprintf("atomic_write: failed to create temp file: %v", err))
	}
	tmpPath := tmpFile.Name()
	_, err = tmpFile.WriteString(content)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		panic(fmt.Sprintf("atomic_write: failed to write: %v", err))
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		panic(fmt.Sprintf("atomic_write: failed to rename: %v", err))
	}
}

func AssertNotEmpty(obj interface{}, step string) {
	if obj == nil || obj == "" {
		panic(fmt.Sprintf("%s produced empty output", step))
	}
}

// BuildPath builds a path from the subdir slice.
func BuildPath(subdir []string, section string) string {
	if len(subdir) == 0 {
		panic("`subdir` must be set")
	}
	rootDir := subdir[0]
	nested := subdir[1:]
	return filepath.Join(append([]string{rootDir, section}, nested...)...)
}

// reviewOk checks if a review passed all checks.
func reviewOk(review map[string]interface{}) bool {
	approved, _ := review["approved"].(bool)
	issues, _ := review["issues"].([]interface{})
	for _, issue := range issues {
		if m, ok := issue.(map[string]interface{}); ok {
			if sev, ok := m["severity"].(string); ok && sev == "high" {
				return false
			}
		}
	}
	return approved
}

// shouldReset checks whether the review recommends resetting coder context.
func shouldReset(review map[string]interface{}) bool {
	val, _ := review["should_reset"].(bool)
	return val
}

// normalizeJSONObjects normalizes embedded JSON objects and arrays in each line
// of the prompt. Iterates raw lines (no trimming): if a line ends with `}`
// it finds the leftmost `{` and tries to round-trip the JSON; same for `]`/`[`.
// Successful round-trips are serialized with encoding/json/v2 Deterministic,
// then written back into the line.
//
// Before serializing, sortJSONKeys recursively sorts all map keys so that
// nested structures are also deterministically ordered.
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
		if err := jsonv2.Unmarshal([]byte(extracted), &v); err != nil {
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

// sortJSONKeys recursively sorts all map keys in a decoded JSON value.
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
