// =========================
// MAIN
// =========================
package main

import (
	"fmt"
	"os"
	"strings"
	"time"
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
	ts := time.Now().Format("2006-01-02_15-04-05")

	args := os.Args[1:]

	if len(args) > 0 && args[0] == "exec" {
		args = args[1:]
	} else if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprintln(os.Stderr, "usage: "+os.Args[0]+" exec [resume <session_id>]")
		os.Exit(0)
	} else {
		fmt.Fprintln(os.Stderr, "usage: "+os.Args[0]+" exec [resume <session_id>]")
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

	task := ""
	if isResume {
		files, _ := os.ReadDir(subdir)
		for _, f := range files {
			if strings.HasSuffix(f.Name(), "-task.txt") {
				data, err := os.ReadFile(fmt.Sprintf("%s/%s", subdir, f.Name()))
				if err == nil {
					task = string(data)
					task = strings.TrimSpace(task)
				}
				break
			}
		}
	} else {
		task = readStdin()
		task = strings.TrimSpace(task)
	}
	if task == "" {
		panic("Task content must be provided")
	}
	if !isResume {
		os.MkdirAll(subdir, 0o755)
		taskFile := fmt.Sprintf("%s/%s-task.txt", subdir, ts)
		os.WriteFile(taskFile, []byte(task), 0o644)
	}
	orch := NewOrchestrator(task, subdir)
	orch.Run(task, subdir)
}
