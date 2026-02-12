package filter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// goValueFlags are go global flags that consume the next argument as a value.
var goValueFlags = map[string]bool{
	"-C": true,
}

// ---------------------------------------------------------------------------
// GoTestStrategy
// ---------------------------------------------------------------------------

// GoTestStrategy filters `go test` output to surface failures and summarize passes.
type GoTestStrategy struct{}

func (s *GoTestStrategy) Name() string { return "go-test" }

func (s *GoTestStrategy) CanHandle(command string, args []string) bool {
	return command == "go" && isSubcommand(args, "test", goValueFlags)
}

// Package-level compiled regexes for GoTestStrategy.
var (
	goTestRunRe          = regexp.MustCompile(`^=== RUN\s+(\S+)`)
	goTestPassRe         = regexp.MustCompile(`^--- PASS:\s`)
	goTestFailRe         = regexp.MustCompile(`^--- FAIL:\s`)
	goTestPauseRe        = regexp.MustCompile(`^=== PAUSE\s`)
	goTestContRe         = regexp.MustCompile(`^=== CONT\s`)
	goTestStandaloneFail = regexp.MustCompile(`^FAIL$`)
)

func (s *GoTestStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
	filterName := s.Name()
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "coc: filter %s recovered from panic: %v\n", filterName, r)
			result = Result{Filtered: string(raw), WasReduced: false}
		}
	}()

	cleaned := StripANSIString(string(raw))
	hadTrailing := endsWithNewline(cleaned)

	lines := strings.Split(cleaned, "\n")

	// Small output — pass through
	pkgCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "ok  \t") || strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "?   \t") {
			pkgCount++
		}
	}
	if pkgCount <= 2 && len(lines) < 10 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Parse test output
	type testBlock struct {
		name   string
		lines  []string
		passed bool
		failed bool
	}

	var summaryLines []string   // ok/FAIL/? lines
	var failBlocks []*testBlock // blocks for failing tests
	var currentBlock *testBlock
	var orphanedLines []string // lines not associated with any test

	for _, line := range lines {
		// Package summary lines
		if strings.HasPrefix(line, "ok  \t") || strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "?   \t") {
			summaryLines = append(summaryLines, line)
			continue
		}

		// Standalone FAIL
		if goTestStandaloneFail.MatchString(line) {
			summaryLines = append(summaryLines, line)
			continue
		}

		// Test start
		if m := goTestRunRe.FindStringSubmatch(line); len(m) > 1 {
			// Save previous block if it was a failure
			if currentBlock != nil && currentBlock.failed {
				failBlocks = append(failBlocks, currentBlock)
			}
			currentBlock = &testBlock{name: m[1], lines: []string{line}}
			continue
		}

		// Skip PAUSE/CONT lines
		if goTestPauseRe.MatchString(line) || goTestContRe.MatchString(line) {
			continue
		}

		// Test pass
		if goTestPassRe.MatchString(line) {
			if currentBlock != nil {
				currentBlock.passed = true
				currentBlock = nil
			}
			continue
		}

		// Test fail
		if goTestFailRe.MatchString(line) {
			if currentBlock != nil {
				currentBlock.failed = true
				currentBlock.lines = append(currentBlock.lines, line)
				failBlocks = append(failBlocks, currentBlock)
				currentBlock = nil
			} else {
				// Fail line without a prior RUN — include as orphan
				orphanedLines = append(orphanedLines, line)
			}
			continue
		}

		// Normal output line — belongs to current test if any
		if currentBlock != nil {
			currentBlock.lines = append(currentBlock.lines, line)
		} else {
			// Orphaned line (compilation error, etc.)
			if strings.TrimSpace(line) != "" {
				orphanedLines = append(orphanedLines, line)
			}
		}
	}

	// In case the last block was a failure
	if currentBlock != nil && currentBlock.failed {
		failBlocks = append(failBlocks, currentBlock)
	}

	var out []string

	if exitCode == 0 {
		// Success: show only summary lines
		out = append(out, summaryLines...)
		passedPkgs := 0
		for _, line := range summaryLines {
			if strings.HasPrefix(line, "ok  \t") || strings.HasPrefix(line, "?   \t") {
				passedPkgs++
			}
		}
		out = append(out, fmt.Sprintf("all tests passed (%d packages)", passedPkgs))
	} else {
		// Failure: show failing test blocks, orphaned lines, and all summaries
		for _, block := range failBlocks {
			out = append(out, block.lines...)
		}
		if len(orphanedLines) > 0 {
			out = append(out, orphanedLines...)
		}
		out = append(out, summaryLines...)
	}

	filtered := strings.Join(out, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	wasReduced := len(filtered) < len(cleaned)
	return Result{Filtered: filtered, WasReduced: wasReduced}
}

// ---------------------------------------------------------------------------
// GoBuildStrategy
// ---------------------------------------------------------------------------

// GoBuildStrategy filters `go build`, `go vet`, and `go install` output.
type GoBuildStrategy struct{}

func (s *GoBuildStrategy) Name() string { return "go-build" }

func (s *GoBuildStrategy) CanHandle(command string, args []string) bool {
	if command != "go" {
		return false
	}
	return isSubcommand(args, "build", goValueFlags) ||
		isSubcommand(args, "vet", goValueFlags) ||
		isSubcommand(args, "install", goValueFlags)
}

// goBuildErrorRe matches compiler-style error lines: file.go:line:col: message
var goBuildErrorRe = regexp.MustCompile(`^\S+\.go:\d+:\d+:`)

func (s *GoBuildStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
	filterName := s.Name()
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "coc: filter %s recovered from panic: %v\n", filterName, r)
			result = Result{Filtered: string(raw), WasReduced: false}
		}
	}()

	cleaned := StripANSIString(string(raw))

	// Success — output is usually empty
	if exitCode == 0 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Failure — keep only error/warning lines and package headers
	hadTrailing := endsWithNewline(cleaned)
	lines := strings.Split(cleaned, "\n")

	var kept []string
	totalNonEmpty := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		totalNonEmpty++

		// Package headers
		if strings.HasPrefix(line, "# ") {
			kept = append(kept, line)
			continue
		}

		// Compiler error/warning lines
		if goBuildErrorRe.MatchString(line) {
			kept = append(kept, line)
			continue
		}
	}

	// If nothing was stripped, pass through as-is
	if len(kept) >= totalNonEmpty {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	filtered := strings.Join(kept, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	return Result{Filtered: filtered, WasReduced: true}
}
