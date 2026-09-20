package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"agent-go/wman"
)

// Context carries per-turn services and test seams.
type Context struct {
	Tracer *Tracer

	// watchmanHook replaces the real filesystem watcher in tests.
	watchmanHook func() map[string]string
	// runCodexHook replaces the real LLM invocation in tests.
	runCodexHook func(agentName, prompt, timeout string, schema map[string]any) (string, error)
}

func NewContext() *Context {
	return &Context{Tracer: &Tracer{}}
}

// RunCodex invokes the LLM runtime for one agent turn. When schema is
// non-nil (decision agents), it is written to a temp file and passed to the
// runtime as --output-schema so the model is constrained at generation time.
func RunCodex(agentName, prompt, timeout string, schema map[string]any, context *Context) (string, error) {
	var stdout string
	var err error
	if context.runCodexHook != nil {
		stdout, err = context.runCodexHook(agentName, prompt, timeout, schema)
	} else {
		stdout, err = realRunCodex(agentName, prompt, timeout, schema)
	}
	context.Tracer.trace("run_codex", map[string]any{
		"agent":     agentName,
		"prompt":    prompt,
		"timeout":   timeout,
		"hasSchema": schema != nil,
		"stdout":    stdout,
		"error":     errorString(err),
	})
	return stdout, err
}

func realRunCodex(agentName, prompt, timeout string, schema map[string]any) (string, error) {
	cmdArgs := []string{"codex", "exec"}
	if timeout != "" {
		cmdArgs = append([]string{"timeout", "-s", "9", timeout}, cmdArgs...)
	}

	var schemaPath string
	if schema != nil {
		f, err := os.CreateTemp("", "schema-*.json")
		if err != nil {
			panic(err)
		}
		schemaPath = f.Name()
		f.Write([]byte(prettyJSON(schema)))
		f.Close()
		defer os.Remove(schemaPath)
		cmdArgs = append(cmdArgs, "--output-schema", schemaPath)
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

	output := strings.TrimSpace(stdoutBuf.String())
	if output == "" {
		return "", fmt.Errorf("empty output from %s, likely timeout issue", agentName)
	}
	return output, nil
}

// AtomicWrite writes content to path via a temp file + rename.
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

// BuildPath joins the session subdir root with a section and nested dirs.
func BuildPath(subdir []string, section string) string {
	if len(subdir) == 0 {
		panic("`subdir` must be set")
	}
	rootDir := subdir[0]
	nested := subdir[1:]
	return filepath.Join(append([]string{rootDir, section}, nested...)...)
}

func wrapText(text string) string {
	return fmt.Sprintf("<text>\n%s\n</text>", text)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// safeFlush drains the filesystem watcher, returning detected changes since the
// last flush. Watcher problems degrade to an empty result, never a crash.
func safeFlush(watcher *wman.Watchman, context *Context) map[string]string {
	defer func() {
		recover()
	}()
	var out map[string]string
	if context.watchmanHook != nil {
		out = context.watchmanHook()
	} else if watcher != nil {
		out = watcher.Flush()
	}
	if out == nil {
		out = map[string]string{}
	}
	context.Tracer.trace("watchman", map[string]any{"changes": out})
	return out
}

// changesPrompt renders the automated change-detection block injected into the
// code review prompt.
func changesPrompt(changes map[string]string) string {
	if len(changes) == 0 {
		return "**AUTOMATED VERIFICATION: NO ACTUAL FILE CHANGES DETECTED ON DISK.** Assess whether the claimed implementation work really happened.\n"
	}
	out := "AUTOMATED CHANGE DETECTION (files changed on disk since the coder step started): the following changes were detected automatically; completeness must be assessed and every claim verified against the repository.\n"
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
