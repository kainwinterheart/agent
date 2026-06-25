package main

import (
	"agent-go/pkg/loader"
	jsonv2text "encoding/json/jsontext"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var runCodexHook func(agentName, session, prompt string, schema map[string]interface{}, timeout string) (string, string, error)

func logStep(msg string, step string) {
	fmt.Fprintf(os.Stderr, ">> [%s] [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), step, msg)
}

var MarshalJSON = loader.MarshalJSON

type IWithReviewerNotesBuilder[
	TReviewerNotes any,
	TObject any,
	TBuilder any,
] interface {
	WithReviewerNotes(*TReviewerNotes) TBuilder
	Build() TObject
}

type IWithReviewerNotes[
	TReviewerNotes any,
	TObject any,
	TBuilder IWithReviewerNotesBuilder[
		TReviewerNotes,
		TObject,
		TBuilder,
	],
] interface {
	ReviewerNotes() *TReviewerNotes
	Clone() TBuilder
}

func RemoveReviewerNotes[
	TReviewerNotes ~[]string,
	TObject IWithReviewerNotes[
		TReviewerNotes,
		TObject,
		TBuilder,
	],
	TBuilder IWithReviewerNotesBuilder[
		TReviewerNotes,
		TObject,
		TBuilder,
	],
](obj TObject) TObject {
	if obj.ReviewerNotes() == nil {
		return obj
	}
	return obj.Clone().WithReviewerNotes(nil).Build()
}

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

func ExtractJSON(text string) (string, error) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 {
		return text[start : end+1], nil
	}
	return "", fmt.Errorf("text does not contain a JSON object")
}

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

func BuildPath(subdir []string, section string) string {
	if len(subdir) == 0 {
		panic("`subdir` must be set")
	}
	rootDir := subdir[0]
	nested := subdir[1:]
	return filepath.Join(append([]string{rootDir, section}, nested...)...)
}
