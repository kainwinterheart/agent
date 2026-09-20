package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// logStep prints a prefixed progress line to stderr.
func logStep(message, step string) {
	log.Printf("[%s] %s", step, message)
}

// AbsPath returns the absolute form of path, falling back to path itself.
func AbsPath(path string) string {
	if abs, err := os.Getwd(); err == nil {
		if !filepath.IsAbs(path) {
			return fmt.Sprintf("%s/%s", abs, path)
		}
	}
	return path
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
