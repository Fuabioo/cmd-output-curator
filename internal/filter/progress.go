package filter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// progressCommands maps command names to subcommands that produce progress output.
var progressCommands = map[string][]string{
	"npm":    {"install", "ci", "update"},
	"yarn":   {"install", "add"},
	"pip":    {"install"},
	"pip3":   {"install"},
	"docker": {"pull", "push"},
}

// progressValueFlags maps command names to flags that consume a following value argument.
var progressValueFlags = map[string]map[string]bool{
	"docker": {"--host": true, "-H": true, "--config": true, "--context": true, "-l": true, "--log-level": true},
	"npm":    {"--prefix": true, "--registry": true, "--cache": true},
	"pip":    {"--target": true, "-t": true, "--prefix": true, "--root": true, "-i": true, "--index-url": true},
	"pip3":   {"--target": true, "-t": true, "--prefix": true, "--root": true, "-i": true, "--index-url": true},
	"yarn":   {"--cwd": true, "--modules-folder": true, "--cache-folder": true},
}

// ---------------------------------------------------------------------------
// ProgressStripStrategy
// ---------------------------------------------------------------------------

// ProgressStripStrategy strips progress bar / spinner output from package
// managers and download commands.
type ProgressStripStrategy struct{}

func (s *ProgressStripStrategy) Name() string { return "progress-strip" }

func (s *ProgressStripStrategy) CanHandle(command string, args []string) bool {
	subs, ok := progressCommands[command]
	if !ok {
		return false
	}
	vf := progressValueFlags[command]
	for _, sub := range subs {
		if isSubcommand(args, sub, vf) {
			return true
		}
	}
	return false
}

// Package-level compiled regexes for ProgressStripStrategy.
var (
	progressBarRe         = regexp.MustCompile(`\[#+[=> ]*\]`)
	progressPercentRe     = regexp.MustCompile(`\d+%`)
	progressSpeedRe       = regexp.MustCompile(`\d+(\.\d+)?\s*(MB|KB|GB|B)/s`)
	progressETARe         = regexp.MustCompile(`(?i)\beta\b`)
	dockerLayerProgressRe = regexp.MustCompile(`^[a-f0-9]+: (Downloading|Extracting|Pulling fs layer|Waiting|Verifying)`)
	dockerLayerCompleteRe = regexp.MustCompile(`^[a-f0-9]+: (Pull complete|Already exists)`)
	npmWarnRe             = regexp.MustCompile(`^npm WARN`)
	npmErrRe              = regexp.MustCompile(`^npm ERR!`)
	npmAddedRe            = regexp.MustCompile(`^added \d+ packages`)
	npmSpinnerRe          = regexp.MustCompile(`^\s*[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏|/\\-]`)
)

func (s *ProgressStripStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
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

	var kept []string
	var prevLine string
	crCleaned := false

	for _, line := range lines {
		// Carriage return cleanup: keep only content after the last \r
		if idx := strings.LastIndex(line, "\r"); idx >= 0 {
			line = line[idx+1:]
			crCleaned = true
		}

		// Docker pull layer progress — drop progress lines, keep complete/exists
		if dockerLayerProgressRe.MatchString(line) {
			continue
		}
		if dockerLayerCompleteRe.MatchString(line) {
			if line != prevLine {
				kept = append(kept, line)
				prevLine = line
			}
			continue
		}

		// npm spinner/whitespace-only lines — drop
		if npmSpinnerRe.MatchString(line) {
			continue
		}

		// npm meaningful lines — always keep
		if npmWarnRe.MatchString(line) || npmErrRe.MatchString(line) || npmAddedRe.MatchString(line) {
			if line != prevLine {
				kept = append(kept, line)
				prevLine = line
			}
			continue
		}

		// Strip progress bar patterns
		if progressBarRe.MatchString(line) {
			continue
		}
		if progressPercentRe.MatchString(line) && progressSpeedRe.MatchString(line) {
			continue
		}
		if progressPercentRe.MatchString(line) && progressETARe.MatchString(line) {
			continue
		}

		// Deduplicate consecutive identical lines
		if line == prevLine {
			continue
		}

		kept = append(kept, line)
		prevLine = line
	}

	linesRemoved := len(lines) - len(kept)
	if linesRemoved <= 0 && !crCleaned {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// If only CR cleanup happened (no lines removed), rebuild from kept lines
	if linesRemoved <= 0 {
		filtered := strings.Join(kept, "\n")
		filtered = ensureTrailingNewline(filtered, hadTrailing)
		wasReduced := len(filtered) < len(cleaned)
		return Result{Filtered: filtered, WasReduced: wasReduced}
	}

	// Prepend header indicating stripped progress
	header := fmt.Sprintf("Progress output stripped (%d lines removed):", linesRemoved)
	out := append([]string{header}, kept...)

	filtered := strings.Join(out, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	return Result{Filtered: filtered, WasReduced: true}
}
