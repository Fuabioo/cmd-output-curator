package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// GenericErrorStrategy is a fallback filter that highlights errors/warnings when
// the exit code is non-zero. It should be registered last (before passthrough)
// so that specific strategies take priority.
type GenericErrorStrategy struct{}

func (s *GenericErrorStrategy) Name() string { return "generic-error" }

func (s *GenericErrorStrategy) CanHandle(_ string, _ []string) bool {
	return true
}

// genericErrorPatterns matches common error/warning patterns in log output.
var genericErrorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\berror\b`),
	regexp.MustCompile(`(?i)\bERRO\b`),
	regexp.MustCompile(`(?i)\bwarning\b`),
	regexp.MustCompile(`(?i)\bWARN\b`),
	regexp.MustCompile(`(?i)\bfatal\b`),
	regexp.MustCompile(`(?i)\bpanic\b`),
	regexp.MustCompile(`^[EW] `),
	regexp.MustCompile(`\S+:\d+:`), // filename:line: pattern
}

func (s *GenericErrorStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result = Result{Filtered: string(raw), WasReduced: false}
		}
	}()

	cleaned := StripANSIString(string(raw))

	// Exit code 0 — pass through unchanged
	if exitCode == 0 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	hadTrailing := endsWithNewline(cleaned)
	lines := strings.Split(cleaned, "\n")

	// Find matching lines
	matched := make([]bool, len(lines))
	matchCount := 0
	for i, line := range lines {
		for _, re := range genericErrorPatterns {
			if re.MatchString(line) {
				matched[i] = true
				matchCount++
				break
			}
		}
	}

	// If 30% or more of lines match, not worth reducing — pass through full
	nonEmpty := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		return Result{Filtered: cleaned, WasReduced: false}
	}
	matchRatio := float64(matchCount) / float64(nonEmpty)
	if matchRatio >= 0.3 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Build output with 1 line of context before and after each match
	included := make([]bool, len(lines))
	for i, isMatch := range matched {
		if !isMatch {
			continue
		}
		// Include the match itself
		included[i] = true
		// 1 line of context before
		if i > 0 {
			included[i-1] = true
		}
		// 1 line of context after
		if i+1 < len(lines) {
			included[i+1] = true
		}
	}

	var out []string
	for i, line := range lines {
		if included[i] {
			out = append(out, line)
		}
	}

	// If nothing was found, pass through full output (better than empty)
	if len(out) == 0 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	header := fmt.Sprintf("Showing errors/warnings from %d total lines:", len(lines))
	all := append([]string{header}, out...)

	filtered := strings.Join(all, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	return Result{Filtered: filtered, WasReduced: true}
}
