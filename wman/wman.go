package wman

import (
    "encoding/json"
	"bufio"
    "io"
    "os"
	"os/exec"
	"path/filepath"
)

type Watchman struct {
    stateDir string
    cmd *exec.Cmd
    stdin *io.Writer
    stdout *bufio.Scanner
}

func NewWatchman(stateDir string) *Watchman {
    return &Watchman{stateDir, nil, nil, nil}
}

func (w *Watchman) Start() {
	wmanPath, _ := filepath.Abs(filepath.Join("wman", "wman.sh"))
    if envPath := os.Getenv("AC_WMAN_PATH"); envPath != "" {
        wmanPath = envPath
    }
	w.cmd = exec.Command(wmanPath, w.stateDir)
    stdin, _ := w.cmd.StdinPipe()
    stdout, _ := w.cmd.StdoutPipe()
    stdin2 := io.Writer(stdin)
    w.stdin = &stdin2
    w.stdout = bufio.NewScanner(stdout)
	w.cmd.Start()
}

func (w *Watchman) Flush() map[string]string {
	(*w.stdin).Write([]byte("flush\n"))
	w.stdout.Scan()
    var out map[string]string
	if err := json.Unmarshal(w.stdout.Bytes(), &out); err != nil {
        panic(err)
    }
    return out
}
