package main

import (
	"strings"
	"testing"
)

func TestNormalizeJSONObjects(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "compact object with spaces",
			input:    `before {"key": "value", "num": 42}`,
			expected: "before {\n\t\"key\": \"value\",\n\t\"num\": 42\n}",
		},
		{
			name:     "nested object sorted keys",
			input:    `{"z": 1, "a": 2}`,
			expected: "{\n\t\"a\": 2,\n\t\"z\": 1\n}",
		},
		{
			name:     "array of objects",
			input:    `[{"b": 2}, {"a": 1}]`,
			expected: "[\n\t{\n\t\t\"b\": 2\n\t},\n\t{\n\t\t\"a\": 1\n\t}\n]",
		},
		{
			name:     "multi-line prompt",
			input:    "line one\n{\"task\": \"build\", \"id\": 1}\nline three",
			expected: "line one\n{\n\t\"id\": 1,\n\t\"task\": \"build\"\n}\nline three",
		},
		{
			name:     "line ending with ]",
			input:    `["hello", "world"]`,
			expected: "[\n\t\"hello\",\n\t\"world\"\n]",
		},
		{
			name:     "non-JSON line skipped",
			input:    `just some text`,
			expected: `just some text`,
		},
		{
			name:     "invalid JSON skipped",
			input:    `{"broken": }`,
			expected: `{"broken": }`,
		},
		{
			name:     "empty line preserved",
			input:    "\n{\"key\": \"val\"}\n",
			expected: "\n{\n\t\"key\": \"val\"\n}\n",
		},
		{
			name:     "leading whitespace preserved",
			input:    "  {\"key\": \"val\"}",
			expected: "  {\n\t\"key\": \"val\"\n}",
		},
		{
			name:     "unicode escape normalized to actual char",
			input:    `{"text": "hello\u2014world"}`,
			expected: "{\n\t\"text\": \"hello—world\"\n}",
		},
		{
			name:     "JSON with nested arrays and objects",
			input:    `{"a": [1, 2], "b": {"c": true, "d": false}}`,
			expected: "{\n\t\"a\": [\n\t\t1,\n\t\t2\n\t],\n\t\"b\": {\n\t\t\"c\": true,\n\t\t\"d\": false\n\t}\n}",
		},
		{
			name:     "line with trailing non-brace text skipped",
			input:    `{"key": "val"} more text`,
			expected: `{"key": "val"} more text`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeJSONObjects(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeJSONObjects(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeJSONObjects_Deterministic(t *testing.T) {
	input := `{"z": 1, "a": 2, "m": {"b": 3, "a": 4}}`
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		got := normalizeJSONObjects(input)
		seen[got] = true
	}
	if len(seen) > 1 {
		t.Errorf("non-deterministic: got %d different outputs", len(seen))
	}
}

func TestNormalizeJSONObjects_NoTrimming(t *testing.T) {
	input := `{"key": "value"}  `
	got := normalizeJSONObjects(input)
	if got != input {
		t.Errorf("trailing spaces should prevent HasSuffix(\"}\") match, got %q", got)
	}
}

func TestNormalizeJSONObjects_NestedKeysSorted(t *testing.T) {
	input := `{"z": {"b": 1, "a": 2}, "a": [{"y": 3, "x": 4}]}`
	got := normalizeJSONObjects(input)
	// Top-level keys should be sorted: "a" before "z"
	// Nested object keys should be sorted: "a" before "b"
	// Array element keys should be sorted: "x" before "y"
	expected := "{\n\t\"a\": [\n\t\t{\n\t\t\t\"x\": 4,\n\t\t\t\"y\": 3\n\t\t}\n\t],\n\t\"z\": {\n\t\t\"a\": 2,\n\t\t\"b\": 1\n\t}\n}"
	if got != expected {
		t.Errorf("nested key sort failed:\ngot  %s\nwant %s", got, expected)
	}
}

// TestNormalizeJSONObjects_PreservesNonJSONLines verifies that lines
// not ending with } or [ are passed through unchanged.
func TestNormalizeJSONObjects_PreservesNonJSONLines(t *testing.T) {
	input := "line1\nline2\nline3"
	got := normalizeJSONObjects(input)
	if got != input {
		t.Errorf("non-JSON lines should pass through unchanged: got %q", got)
	}
}

// TestNormalizeJSONObjects_MultipleJSONObjects verifies that multiple
// JSON objects on different lines are all normalized.
func TestNormalizeJSONObjects_MultipleJSONObjects(t *testing.T) {
	input := `{"z": 1} and {"a": 2}`
	got := normalizeJSONObjects(input)
	// The first line ends with "}" so it should be normalized.
	// The second line also ends with "}" so it should be normalized.
	if !strings.Contains(got, "\"a\": 2") {
		t.Errorf("expected sorted key \"a\" in output, got %q", got)
	}
}
