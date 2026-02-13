package filter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// cargoValueFlags are cargo flags that consume the next argument as a value.
var cargoValueFlags = map[string]bool{
	"--manifest-path": true,
	"--color":         true,
}

// ---------------------------------------------------------------------------
// CargoTestStrategy
// ---------------------------------------------------------------------------

// CargoTestStrategy filters `cargo test` output to surface failures and summarize passes.
type CargoTestStrategy struct{}

func (s *CargoTestStrategy) Name() string { return "cargo-test" }

func (s *CargoTestStrategy) CanHandle(command string, args []string) bool {
	return command == "cargo" && isSubcommand(args, "test", cargoValueFlags)
}

// Package-level compiled regexes for CargoTestStrategy.
var (
	cargoTestRunningRe     = regexp.MustCompile(`^running \d+ tests?`)
	cargoTestResultRe      = regexp.MustCompile(`^test result:`)
	cargoTestFailedRe      = regexp.MustCompile(`^test .+ FAILED$`)
	cargoFailuresRe        = regexp.MustCompile(`^failures:`)
	cargoTestPassedCountRe = regexp.MustCompile(`(\d+) passed`)
)

func (s *CargoTestStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
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
	if len(lines) < 10 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	if exitCode == 0 {
		// Success: keep "running N tests" and "test result:" lines, append summary
		var kept []string
		totalTests := 0

		for _, line := range lines {
			if cargoTestRunningRe.MatchString(line) {
				kept = append(kept, line)
				continue
			}
			if cargoTestResultRe.MatchString(line) {
				kept = append(kept, line)
				// Extract the total from "test result: ok. N passed; ..."
				totalTests += countCargoTestsFromResult(line)
				continue
			}
		}

		kept = append(kept, fmt.Sprintf("all tests passed (%d total)", totalTests))

		filtered := strings.Join(kept, "\n")
		filtered = ensureTrailingNewline(filtered, hadTrailing)
		wasReduced := len(filtered) < len(cleaned)
		return Result{Filtered: filtered, WasReduced: wasReduced}
	}

	// Failure: keep failures: section, test result: lines, test ... FAILED lines, and "running N tests" headers
	var kept []string
	inFailuresSection := false

	for _, line := range lines {
		// Keep "running N tests" headers for multi-target context
		if cargoTestRunningRe.MatchString(line) {
			kept = append(kept, line)
			continue
		}

		// Start of failures: section
		if cargoFailuresRe.MatchString(line) {
			inFailuresSection = true
			kept = append(kept, line)
			continue
		}

		// Inside failures: section, keep until next test result:
		if inFailuresSection {
			if cargoTestResultRe.MatchString(line) {
				inFailuresSection = false
				kept = append(kept, line)
				continue
			}
			kept = append(kept, line)
			continue
		}

		// test result: lines (outside failures section)
		if cargoTestResultRe.MatchString(line) {
			kept = append(kept, line)
			continue
		}

		// test ... FAILED lines
		if cargoTestFailedRe.MatchString(line) {
			kept = append(kept, line)
			continue
		}
	}

	filtered := strings.Join(kept, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)
	wasReduced := len(filtered) < len(cleaned)
	return Result{Filtered: filtered, WasReduced: wasReduced}
}

// countCargoTestsFromResult extracts the passed count from a "test result: ok. N passed; ..." line.
func countCargoTestsFromResult(line string) int {
	m := cargoTestPassedCountRe.FindStringSubmatch(line)
	if len(m) > 1 {
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		return n
	}
	return 0
}

// ---------------------------------------------------------------------------
// CargoBuildStrategy
// ---------------------------------------------------------------------------

// CargoBuildStrategy filters `cargo build`, `cargo check`, and `cargo clippy` output.
type CargoBuildStrategy struct{}

func (s *CargoBuildStrategy) Name() string { return "cargo-build" }

func (s *CargoBuildStrategy) CanHandle(command string, args []string) bool {
	if command != "cargo" {
		return false
	}
	return isSubcommand(args, "build", cargoValueFlags) ||
		isSubcommand(args, "check", cargoValueFlags) ||
		isSubcommand(args, "clippy", cargoValueFlags)
}

// Package-level compiled regexes for CargoBuildStrategy.
var (
	cargoBuildErrorRe   = regexp.MustCompile(`^error\[|^error:`)
	cargoBuildWarningRe = regexp.MustCompile(`^warning\[|^warning:`)
	cargoBuildArrowRe   = regexp.MustCompile(`^\s*-->`)
	cargoBuildAbortRe   = regexp.MustCompile(`^aborting due to`)
	cargoBuildMoreRe    = regexp.MustCompile(`^For more information`)
	cargoBuildNoteRe    = regexp.MustCompile(`^= `)
	cargoBuildPipeRe    = regexp.MustCompile(`^\s*\d*\s*\|`)
)

func (s *CargoBuildStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
	filterName := s.Name()
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "coc: filter %s recovered from panic: %v\n", filterName, r)
			result = Result{Filtered: string(raw), WasReduced: false}
		}
	}()

	cleaned := StripANSIString(string(raw))

	// Success — pass through
	if exitCode == 0 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Failure — keep only error/warning lines and related context
	hadTrailing := endsWithNewline(cleaned)
	lines := strings.Split(cleaned, "\n")

	var kept []string
	totalNonEmpty := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		totalNonEmpty++

		if cargoBuildErrorRe.MatchString(line) ||
			cargoBuildWarningRe.MatchString(line) ||
			cargoBuildArrowRe.MatchString(line) ||
			cargoBuildAbortRe.MatchString(line) ||
			cargoBuildMoreRe.MatchString(line) ||
			cargoBuildNoteRe.MatchString(line) ||
			cargoBuildPipeRe.MatchString(line) {
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
