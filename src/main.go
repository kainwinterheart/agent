package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-go/pkg/loader"
)

func readStdin() string {
	buf := make([]byte, 4096)
	var data []byte
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(data)
}

func main() {
	loader.InitPromptLoader()
	loader.InitPrompts()
	initSubworkflows()

	ts := time.Now().Format("2006-01-02_15-04-05")

	args := os.Args[1:]

	if len(args) > 0 && args[0] == "static-defs" {
		doc := StaticDefinitionsDocument()
		if len(args) > 1 && args[1] != "" {
			if err := os.MkdirAll(filepath.Dir(args[1]), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(args[1], []byte(doc), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "static definitions written to %s\n", args[1])
			return
		}
		fmt.Print(doc)
		return
	}

	if len(args) > 0 && args[0] == "exec" {
		args = args[1:]
	} else if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(os.Stderr, "usage: "+os.Args[0]+" exec [resume <session_id>] | static-defs [path]")
		os.Exit(0)
	} else {
		fmt.Fprintln(os.Stderr, "usage: "+os.Args[0]+" exec [resume <session_id>] | static-defs [path]")
		os.Exit(1)
	}

	isResume := false
	sessionID := ""

	if len(args) > 0 && args[0] == "resume" {
		isResume = true
		args = args[1:]
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Missing session_id")
			os.Exit(1)
		}
		sessionID = args[0]
	}

	subdir := sessionID
	if !isResume {
		subdir = fmt.Sprintf(".agent-%s", ts)
	}

	fmt.Fprintf(os.Stderr, "session id: %s\n", subdir)

	// On resume the workflow state file is authoritative; Run loads it when
	// task is empty.
	task := ""
	if !isResume {
		task = readStdin()
		task = strings.TrimSpace(task)
		if task == "" {
			panic("Task content must be provided")
		}
		os.MkdirAll(subdir, 0o755)
		taskFile := fmt.Sprintf("%s/%s-task.txt", subdir, ts)
		os.WriteFile(taskFile, []byte(task), 0o644)
	}

	orch := NewOrchestrator(subdir)
	if err := orch.Run(task, subdir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
