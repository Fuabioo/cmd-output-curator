package filter

import (
	"strings"
	"testing"
)

func TestGenericErrorStrategy_CanHandle(t *testing.T) {
	s := &GenericErrorStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
	}{
		{"any command", "anything", nil},
		{"empty command", "", nil},
		{"with args", "foo", []string{"bar", "baz"}},
		{"known command", "git", []string{"status"}},
		{"cargo build", "cargo", []string{"build"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !s.CanHandle(tc.command, tc.args) {
				t.Errorf("CanHandle(%q, %v) = false, want true (always matches)", tc.command, tc.args)
			}
		})
	}
}

func TestGenericErrorStrategy_Name(t *testing.T) {
	s := &GenericErrorStrategy{}
	if got := s.Name(); got != "generic-error" {
		t.Errorf("Name() = %q, want %q", got, "generic-error")
	}
}

func TestGenericErrorStrategy_Filter_ExitZero(t *testing.T) {
	s := &GenericErrorStrategy{}

	input := "line 1\nline 2\nline 3\nsome output\n"
	result := s.Filter([]byte(input), "some-cmd", nil, 0)

	if result.WasReduced {
		t.Error("exit code 0 should not reduce output")
	}
	if result.Filtered != input {
		t.Errorf("exit code 0 should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
	}
}

func TestGenericErrorStrategy_Filter_WithErrors(t *testing.T) {
	s := &GenericErrorStrategy{}

	// Mix of normal output and a few error lines (< 30% ratio)
	input := "Starting process\n" +
		"Loading config\n" +
		"Connecting to database\n" +
		"Processing item 1\n" +
		"Processing item 2\n" +
		"Processing item 3\n" +
		"Processing item 4\n" +
		"Processing item 5\n" +
		"Error: connection refused\n" +
		"Processing item 6\n" +
		"Processing item 7\n" +
		"Processing item 8\n" +
		"Processing item 9\n" +
		"Processing item 10\n" +
		"Processing item 11\n" +
		"Processing item 12\n" +
		"Done\n"

	result := s.Filter([]byte(input), "some-cmd", nil, 1)

	if !result.WasReduced {
		t.Fatal("expected WasReduced=true when error lines are extracted")
	}

	// Error line should be preserved
	if !strings.Contains(result.Filtered, "Error: connection refused") {
		t.Error("error line should be preserved")
	}

	// Header should indicate how many total lines
	if !strings.Contains(result.Filtered, "Showing errors/warnings from") {
		t.Error("expected header line showing total line count")
	}

	// Context lines should be included (1 before, 1 after)
	if !strings.Contains(result.Filtered, "Processing item 5") {
		t.Error("expected 1 line of context before error")
	}
	if !strings.Contains(result.Filtered, "Processing item 6") {
		t.Error("expected 1 line of context after error")
	}

	// Distant normal lines should be stripped
	if strings.Contains(result.Filtered, "Starting process") {
		t.Error("distant normal lines should be stripped")
	}
	if strings.Contains(result.Filtered, "Processing item 12") {
		t.Error("distant normal lines should be stripped")
	}
}

func TestGenericErrorStrategy_Filter_MostlyErrors(t *testing.T) {
	s := &GenericErrorStrategy{}

	// > 30% of lines are error-matching, so not worth reducing
	input := "Error: first problem\n" +
		"Warning: something off\n" +
		"Error: second problem\n" +
		"normal line\n" +
		"Fatal: crash\n" +
		"another normal line\n"

	result := s.Filter([]byte(input), "some-cmd", nil, 1)

	// 4 out of 6 non-empty lines match = 66% > 30%, so pass through full
	if result.WasReduced {
		t.Error("mostly error lines (>30%) should not be reduced")
	}
	if result.Filtered != input {
		t.Errorf("mostly error output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
	}
}

func TestGenericErrorStrategy_Filter_NoPatterns(t *testing.T) {
	s := &GenericErrorStrategy{}

	// Exit code 1, but no recognizable error patterns in output
	input := "some output\n" +
		"more output\n" +
		"still more output\n" +
		"final output\n"

	result := s.Filter([]byte(input), "some-cmd", nil, 1)

	// No error patterns found → 0 matches → out will be empty → pass through full
	if result.WasReduced {
		t.Error("no recognizable error patterns should not reduce output")
	}
	if result.Filtered != input {
		t.Errorf("no-pattern output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
	}
}

func TestGenericErrorStrategy_Filter_EmptyInput(t *testing.T) {
	s := &GenericErrorStrategy{}

	result := s.Filter([]byte(""), "some-cmd", nil, 1)

	// Empty input with nonzero exit: nonEmpty==0, so pass through
	if result.WasReduced {
		t.Error("empty input should not be reduced")
	}
	if result.Filtered != "" {
		t.Errorf("empty input should return empty, got: %q", result.Filtered)
	}
}

func TestGenericErrorStrategy_Filter_WarningPattern(t *testing.T) {
	s := &GenericErrorStrategy{}

	// Test that "warning" pattern is recognized
	input := "line 1\n" +
		"line 2\n" +
		"line 3\n" +
		"line 4\n" +
		"line 5\n" +
		"line 6\n" +
		"line 7\n" +
		"line 8\n" +
		"line 9\n" +
		"line 10\n" +
		"warning: something deprecated\n" +
		"line 12\n" +
		"line 13\n" +
		"line 14\n" +
		"line 15\n"

	result := s.Filter([]byte(input), "some-cmd", nil, 1)

	if !result.WasReduced {
		t.Fatal("expected WasReduced=true when warning lines are extracted")
	}

	if !strings.Contains(result.Filtered, "warning: something deprecated") {
		t.Error("warning line should be preserved")
	}
}

func TestGenericErrorStrategy_Filter_FileLinePattern(t *testing.T) {
	s := &GenericErrorStrategy{}

	// Test the filename:line: pattern
	input := "line 1\n" +
		"line 2\n" +
		"line 3\n" +
		"line 4\n" +
		"line 5\n" +
		"line 6\n" +
		"line 7\n" +
		"line 8\n" +
		"line 9\n" +
		"line 10\n" +
		"main.go:42: something went wrong\n" +
		"line 12\n" +
		"line 13\n" +
		"line 14\n" +
		"line 15\n"

	result := s.Filter([]byte(input), "some-cmd", nil, 1)

	if !result.WasReduced {
		t.Fatal("expected WasReduced=true when file:line: pattern lines are extracted")
	}

	if !strings.Contains(result.Filtered, "main.go:42: something went wrong") {
		t.Error("file:line: pattern line should be preserved")
	}
}
