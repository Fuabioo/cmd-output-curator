package filter

import (
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// GrepGroupStrategy
// ---------------------------------------------------------------------------

func TestGrepGroupStrategy_CanHandle(t *testing.T) {
	s := &GrepGroupStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"grep recursive", "grep", []string{"-rn", "pattern", "."}, true},
		{"grep basic", "grep", []string{"pattern", "file.txt"}, true},
		{"grep with flags", "grep", []string{"-i", "-n", "pattern"}, true},
		{"grep color", "grep", []string{"--color=auto", "pattern", "."}, true},
		{"rg pattern", "rg", []string{"pattern"}, true},
		{"rg with flags", "rg", []string{"-i", "--type", "go", "pattern"}, true},
		{"rg search", "rg", []string{"--no-heading", "pattern"}, true},
		{"not grep", "find", []string{"-name", "*.go"}, false},
		{"not rg", "ag", []string{"pattern"}, false},
		{"ack command", "ack", []string{"pattern"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.CanHandle(tc.command, tc.args)
			if got != tc.want {
				t.Errorf("CanHandle(%q, %v) = %v, want %v", tc.command, tc.args, got, tc.want)
			}
		})
	}
}

func TestGrepGroupStrategy_Name(t *testing.T) {
	s := &GrepGroupStrategy{}
	if got := s.Name(); got != "grep-group" {
		t.Errorf("Name() = %q, want %q", got, "grep-group")
	}
}

func TestGrepGroupStrategy_Filter(t *testing.T) {
	s := &GrepGroupStrategy{}

	t.Run("many matches across files", func(t *testing.T) {
		input := "src/main.go:10:func main() {\n" +
			"src/main.go:15:    fmt.Println(\"hello\")\n" +
			"src/main.go:20:    fmt.Println(\"world\")\n" +
			"src/handler.go:5:func handleRequest() {\n" +
			"src/handler.go:10:    fmt.Println(\"request\")\n" +
			"src/handler.go:15:    fmt.Println(\"response\")\n" +
			"src/handler.go:20:    fmt.Println(\"done\")\n" +
			"src/handler.go:25:    fmt.Println(\"cleanup\")\n" +
			"src/handler.go:30:    fmt.Println(\"exit\")\n" +
			"src/handler.go:35:    fmt.Println(\"final\")\n" +
			"src/handler.go:40:    fmt.Println(\"really final\")\n" +
			"src/handler.go:45:    fmt.Println(\"ok\")\n" +
			"src/handler.go:50:    fmt.Println(\"last one\")\n" +
			"src/utils.go:3:func helper() {\n" +
			"src/utils.go:8:    fmt.Println(\"help\")\n" +
			"src/config.go:1:var config = \"test\"\n" +
			"src/config.go:5:var config2 = \"test2\"\n" +
			"src/config.go:10:var config3 = \"test3\"\n" +
			"src/config.go:15:var config4 = \"test4\"\n" +
			"src/config.go:20:var config5 = \"test5\"\n" +
			"src/config.go:25:var config6 = \"test6\"\n" +
			"src/config.go:30:var config7 = \"test7\"\n" +
			"src/config.go:35:var config8 = \"test8\"\n" +
			"src/config.go:40:var config9 = \"test9\"\n" +
			"src/config.go:45:var config10 = \"test10\"\n"

		result := s.Filter([]byte(input), "grep", []string{"-rn", "pattern", "."}, 0)

		// Should have file grouping headers
		if !strings.Contains(result.Filtered, "src/main.go") {
			t.Error("expected src/main.go group header")
		}
		if !strings.Contains(result.Filtered, "src/handler.go") {
			t.Error("expected src/handler.go group header")
		}
		if !strings.Contains(result.Filtered, "src/utils.go") {
			t.Error("expected src/utils.go group header")
		}
		if !strings.Contains(result.Filtered, "src/config.go") {
			t.Error("expected src/config.go group header")
		}

		// Should show match counts in headers
		if !strings.Contains(result.Filtered, "(3 matches)") {
			t.Error("expected (3 matches) for main.go")
		}
		if !strings.Contains(result.Filtered, "(10 matches)") {
			t.Error("expected (10 matches) for handler.go and config.go")
		}
		if !strings.Contains(result.Filtered, "(2 matches)") {
			t.Error("expected (2 matches) for utils.go")
		}

		// Files with few matches (<= 8) should have all lines preserved
		if !strings.Contains(result.Filtered, "src/main.go:10:func main()") {
			t.Error("main.go:10 should be preserved")
		}
		if !strings.Contains(result.Filtered, "src/main.go:15:") {
			t.Error("main.go:15 should be preserved")
		}
		if !strings.Contains(result.Filtered, "src/main.go:20:") {
			t.Error("main.go:20 should be preserved")
		}

		// Files with many matches (> 8) should be truncated
		if !strings.Contains(result.Filtered, "src/handler.go:5:func handleRequest()") {
			t.Error("handler.go:5 (first line) should be preserved")
		}
		if !strings.Contains(result.Filtered, "src/handler.go:10:") {
			t.Error("handler.go:10 (second line) should be preserved")
		}
		if !strings.Contains(result.Filtered, "src/handler.go:15:") {
			t.Error("handler.go:15 (third line) should be preserved")
		}
		if !strings.Contains(result.Filtered, "... 4 more") {
			t.Error("expected '... 4 more' truncation indicator for handler.go")
		}
		if !strings.Contains(result.Filtered, "src/handler.go:40:") {
			t.Error("handler.go:40 (third-to-last) should be preserved")
		}
		if !strings.Contains(result.Filtered, "src/handler.go:45:") {
			t.Error("handler.go:45 (second-to-last) should be preserved")
		}
		if !strings.Contains(result.Filtered, "src/handler.go:50:") {
			t.Error("handler.go:50 (last) should be preserved")
		}

		// Middle lines of truncated files should NOT appear
		if strings.Contains(result.Filtered, "src/handler.go:20:") {
			t.Error("handler.go:20 (middle line) should be truncated")
		}
		if strings.Contains(result.Filtered, "src/handler.go:25:") {
			t.Error("handler.go:25 (middle line) should be truncated")
		}
		if strings.Contains(result.Filtered, "src/handler.go:30:") {
			t.Error("handler.go:30 (middle line) should be truncated")
		}
		if strings.Contains(result.Filtered, "src/handler.go:35:") {
			t.Error("handler.go:35 (middle line) should be truncated")
		}

		// Summary footer should show total matches and file count
		if !strings.Contains(result.Filtered, "25 matches") {
			t.Error("expected summary with '25 matches'")
		}
		if !strings.Contains(result.Filtered, "4 files") {
			t.Error("expected summary with '4 files'")
		}

		if !result.WasReduced {
			t.Error("expected WasReduced=true since output was truncated")
		}
	})

	t.Run("grep no matches", func(t *testing.T) {
		input := ""

		result := s.Filter([]byte(input), "grep", []string{"pattern", "file.txt"}, 1)

		if result.WasReduced {
			t.Error("expected WasReduced=false for no matches (exit 1)")
		}
		if result.Filtered != input {
			t.Errorf("empty output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("grep error", func(t *testing.T) {
		input := "grep: invalid option -- 'z'\nUsage: grep [OPTION]... PATTERN [FILE]...\nTry 'grep --help' for more information.\n"

		result := s.Filter([]byte(input), "grep", []string{"-z", "pattern"}, 2)

		if result.WasReduced {
			t.Error("expected WasReduced=false for error output (exit 2)")
		}
		if result.Filtered != input {
			t.Errorf("error output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("small output", func(t *testing.T) {
		input := "file.txt:1:first line\n" +
			"file.txt:2:second line\n" +
			"file.txt:3:third line\n" +
			"other.txt:5:match here\n" +
			"other.txt:10:another match\n"

		result := s.Filter([]byte(input), "grep", []string{"pattern", "*.txt"}, 0)

		if result.WasReduced {
			t.Error("small output (< 10 lines) should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("small output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := s.Filter([]byte(""), "grep", []string{"pattern"}, 0)

		if result.WasReduced {
			t.Error("empty input should not be reduced")
		}
		if result.Filtered != "" {
			t.Errorf("empty input should produce empty output, got: %q", result.Filtered)
		}
	})

	t.Run("separator lines", func(t *testing.T) {
		input := "file.txt:1:first match\n" +
			"file.txt:2:second match\n" +
			"--\n" +
			"file.txt:10:third match\n" +
			"file.txt:11:fourth match\n" +
			"--\n" +
			"other.txt:5:another match\n" +
			"other.txt:6:yet another\n" +
			"--\n" +
			"other.txt:20:final match\n"

		result := s.Filter([]byte(input), "grep", []string{"-C", "1", "pattern", "*.txt"}, 0)

		// Should group by filename, stripping separators
		if !strings.Contains(result.Filtered, "file.txt") {
			t.Error("expected file.txt group")
		}
		if !strings.Contains(result.Filtered, "other.txt") {
			t.Error("expected other.txt group")
		}

		// Separator lines should be stripped
		separatorCount := strings.Count(result.Filtered, "--")
		if separatorCount > 0 {
			t.Errorf("expected separator lines to be stripped, found %d", separatorCount)
		}

		// Content should be preserved
		if !strings.Contains(result.Filtered, ":1:first match") {
			t.Error("match content should be preserved")
		}
		if !strings.Contains(result.Filtered, ":20:final match") {
			t.Error("match content should be preserved")
		}
	})

	t.Run("rg output without line numbers", func(t *testing.T) {
		input := "src/main.go:func main() {\n" +
			"src/main.go:    fmt.Println(\"hello\")\n" +
			"src/main.go:    return\n" +
			"src/utils.go:func helper() {\n" +
			"src/utils.go:    // helper code\n" +
			"src/utils.go:    return\n" +
			"README.md:# Project\n" +
			"README.md:Some documentation\n"

		result := s.Filter([]byte(input), "rg", []string{"pattern"}, 0)

		// Should still group by filename
		if !strings.Contains(result.Filtered, "src/main.go") {
			t.Error("expected src/main.go group")
		}
		if !strings.Contains(result.Filtered, "src/utils.go") {
			t.Error("expected src/utils.go group")
		}
		if !strings.Contains(result.Filtered, "README.md") {
			t.Error("expected README.md group")
		}

		// Content should be preserved
		if !strings.Contains(result.Filtered, "func main()") {
			t.Error("match content should be preserved")
		}
		if !strings.Contains(result.Filtered, "func helper()") {
			t.Error("match content should be preserved")
		}
	})

	t.Run("binary file matches", func(t *testing.T) {
		input := "src/main.go:10:func main() {\n" +
			"src/main.go:15:    fmt.Println(\"hello\")\n" +
			"Binary file vendor/lib.a matches\n" +
			"Binary file build/app matches\n" +
			"src/utils.go:3:func helper() {\n" +
			"src/utils.go:8:    return\n" +
			"Binary file data/image.png matches\n" +
			"README.md:1:# Project\n" +
			"README.md:5:## Overview\n"

		result := s.Filter([]byte(input), "grep", []string{"-r", "pattern", "."}, 0)

		// Binary file lines should be preserved
		if !strings.Contains(result.Filtered, "Binary file vendor/lib.a matches") {
			t.Error("binary file line should be preserved")
		}
		if !strings.Contains(result.Filtered, "Binary file build/app matches") {
			t.Error("binary file line should be preserved")
		}
		if !strings.Contains(result.Filtered, "Binary file data/image.png matches") {
			t.Error("binary file line should be preserved")
		}

		// Regular file groups should still be present
		if !strings.Contains(result.Filtered, "src/main.go") {
			t.Error("expected src/main.go group")
		}
		if !strings.Contains(result.Filtered, "src/utils.go") {
			t.Error("expected src/utils.go group")
		}
		if !strings.Contains(result.Filtered, "README.md") {
			t.Error("expected README.md group")
		}

		// Content should be preserved
		if !strings.Contains(result.Filtered, "func main()") {
			t.Error("match content should be preserved")
		}
	})

	t.Run("all files need truncation", func(t *testing.T) {
		// Create input with 3 files, each having 12 matches (all > 8)
		var lines []string
		for i := 1; i <= 12; i++ {
			lines = append(lines, "file1.go:"+strconv.Itoa(i*5)+":line "+strconv.Itoa(i))
		}
		for i := 1; i <= 12; i++ {
			lines = append(lines, "file2.go:"+strconv.Itoa(i*5)+":line "+strconv.Itoa(i))
		}
		for i := 1; i <= 12; i++ {
			lines = append(lines, "file3.go:"+strconv.Itoa(i*5)+":line "+strconv.Itoa(i))
		}
		input := strings.Join(lines, "\n") + "\n"

		result := s.Filter([]byte(input), "grep", []string{"-rn", "pattern", "."}, 0)

		// All three files should be truncated
		truncationCount := strings.Count(result.Filtered, "... ")
		if truncationCount != 3 {
			t.Errorf("expected 3 truncated file groups, got %d", truncationCount)
		}

		// Summary should show correct totals
		if !strings.Contains(result.Filtered, "36 matches") {
			t.Error("expected summary with '36 matches'")
		}
		if !strings.Contains(result.Filtered, "3 files") {
			t.Error("expected summary with '3 files'")
		}

		if !result.WasReduced {
			t.Error("expected WasReduced=true")
		}
	})

	t.Run("mixed truncation", func(t *testing.T) {
		// file1: 2 matches (no truncation)
		// file2: 15 matches (truncation: first 3 + last 3, with "... 9 more")
		// file3: 5 matches (no truncation)
		var lines []string
		lines = append(lines, "file1.txt:1:match 1")
		lines = append(lines, "file1.txt:2:match 2")

		for i := 1; i <= 15; i++ {
			lines = append(lines, "file2.txt:"+strconv.Itoa(i*10)+":match "+strconv.Itoa(i))
		}

		lines = append(lines, "file3.txt:1:match 1")
		lines = append(lines, "file3.txt:2:match 2")
		lines = append(lines, "file3.txt:3:match 3")
		lines = append(lines, "file3.txt:4:match 4")
		lines = append(lines, "file3.txt:5:match 5")

		input := strings.Join(lines, "\n") + "\n"

		result := s.Filter([]byte(input), "grep", []string{"-rn", "pattern", "."}, 0)

		// file1: all 2 matches visible
		if !strings.Contains(result.Filtered, "file1.txt:1:match 1") {
			t.Error("file1.txt:1 should be visible")
		}
		if !strings.Contains(result.Filtered, "file1.txt:2:match 2") {
			t.Error("file1.txt:2 should be visible")
		}

		// file2: first 3 + last 3, middle truncated
		if !strings.Contains(result.Filtered, "file2.txt:10:match 1") {
			t.Error("file2.txt first match should be visible")
		}
		if !strings.Contains(result.Filtered, "file2.txt:20:match 2") {
			t.Error("file2.txt second match should be visible")
		}
		if !strings.Contains(result.Filtered, "file2.txt:30:match 3") {
			t.Error("file2.txt third match should be visible")
		}
		if !strings.Contains(result.Filtered, "... 9 more") {
			t.Error("expected '... 9 more' for file2.txt")
		}
		if !strings.Contains(result.Filtered, "file2.txt:130:match 13") {
			t.Error("file2.txt third-to-last match should be visible")
		}
		if !strings.Contains(result.Filtered, "file2.txt:140:match 14") {
			t.Error("file2.txt second-to-last match should be visible")
		}
		if !strings.Contains(result.Filtered, "file2.txt:150:match 15") {
			t.Error("file2.txt last match should be visible")
		}

		// Middle matches should NOT be visible
		if strings.Contains(result.Filtered, "file2.txt:50:match 5") {
			t.Error("file2.txt middle match should be truncated")
		}

		// file3: all 5 matches visible
		if !strings.Contains(result.Filtered, "file3.txt:5:match 5") {
			t.Error("file3.txt:5 should be visible")
		}

		// Summary
		if !strings.Contains(result.Filtered, "22 matches") {
			t.Error("expected summary with '22 matches'")
		}
		if !strings.Contains(result.Filtered, "3 files") {
			t.Error("expected summary with '3 files'")
		}

		if !result.WasReduced {
			t.Error("expected WasReduced=true")
		}
	})
}
