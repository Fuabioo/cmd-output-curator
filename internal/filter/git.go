package filter

import (
	"fmt"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Shared helpers for git strategies
// ---------------------------------------------------------------------------

// gitValueFlags are git global flags that consume the next argument as a value.
var gitValueFlags = map[string]bool{
	"-c": true, "-C": true, "--git-dir": true, "--work-tree": true,
}

// isSubcommand finds the first non-flag argument in args and checks if it matches subcmd.
// It understands that certain flags (like git's -c) consume the next argument.
func isSubcommand(args []string, subcmd string) bool {
	skip := false
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if gitValueFlags[a] {
			skip = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == subcmd
	}
	return false
}

// endsWithNewline reports whether s ends with a newline character.
func endsWithNewline(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '\n'
}

// ensureTrailingNewline appends a newline if the original had one and result doesn't.
func ensureTrailingNewline(result string, hadTrailing bool) string {
	if hadTrailing && !endsWithNewline(result) {
		return result + "\n"
	}
	return result
}

// ---------------------------------------------------------------------------
// GitStatusStrategy
// ---------------------------------------------------------------------------

// GitStatusStrategy filters `git status` output into a compact summary.
type GitStatusStrategy struct{}

func (s *GitStatusStrategy) Name() string { return "git-status" }

func (s *GitStatusStrategy) CanHandle(command string, args []string) bool {
	return command == "git" && isSubcommand(args, "status")
}

func (s *GitStatusStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result = Result{Filtered: string(raw), WasReduced: false}
		}
	}()

	cleaned := StripANSIString(string(raw))
	hadTrailing := endsWithNewline(cleaned)

	// Clean tree — pass through unchanged
	if strings.Contains(cleaned, "nothing to commit, working tree clean") {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	lines := strings.Split(cleaned, "\n")

	// Small output — don't bother filtering
	if len(lines) < 5 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Status marker replacements (verbose → short form)
	statusReplacements := []struct {
		from string
		to   string
	}{
		{"modified:", "M"},
		{"new file:", "A"},
		{"deleted:", "D"},
		{"renamed:", "R"},
		{"copied:", "C"},
		{"typechange:", "T"},
	}

	var out []string
	staged := 0
	unstaged := 0
	untracked := 0

	section := "" // track which section we're in

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Keep "On branch ..." line
		if strings.HasPrefix(line, "On branch ") {
			out = append(out, line)
			continue
		}

		// Section headers
		if strings.HasPrefix(line, "Changes to be committed:") {
			section = "staged"
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(line, "Changes not staged for commit:") {
			section = "unstaged"
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(line, "Untracked files:") {
			section = "untracked"
			out = append(out, line)
			continue
		}

		// Skip hint lines (lines starting with `  (use "git`)
		if strings.HasPrefix(line, `  (use "git`) {
			continue
		}

		// File listing lines (start with tab)
		if strings.HasPrefix(line, "\t") {
			converted := line
			for _, rep := range statusReplacements {
				if strings.Contains(converted, rep.from) {
					converted = strings.Replace(converted, rep.from, rep.to, 1)
					break
				}
			}
			out = append(out, converted)

			switch section {
			case "staged":
				staged++
			case "unstaged":
				unstaged++
			case "untracked":
				untracked++
			}
			continue
		}

		// Keep empty lines between sections for readability
		if trimmed == "" {
			out = append(out, line)
			continue
		}

		// Drop everything else (other hint lines, etc.)
	}

	// Add summary line
	summary := fmt.Sprintf("%d staged, %d unstaged, %d untracked", staged, unstaged, untracked)
	out = append(out, summary)

	filtered := strings.Join(out, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	return Result{Filtered: filtered, WasReduced: true}
}

// ---------------------------------------------------------------------------
// GitDiffStrategy
// ---------------------------------------------------------------------------

// GitDiffStrategy filters `git diff` output by removing noise and adding a file summary.
type GitDiffStrategy struct{}

func (s *GitDiffStrategy) Name() string { return "git-diff" }

func (s *GitDiffStrategy) CanHandle(command string, args []string) bool {
	return command == "git" && isSubcommand(args, "diff")
}

// indexLineRe matches "index <hash>..<hash>" lines in git diff output.
var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+`)

func (s *GitDiffStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result = Result{Filtered: string(raw), WasReduced: false}
		}
	}()

	cleaned := StripANSIString(string(raw))
	hadTrailing := endsWithNewline(cleaned)

	lines := strings.Split(cleaned, "\n")

	// Short diff — pass through unchanged
	if len(lines) < 20 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// First pass: collect file stats and filter lines
	type fileStat struct {
		name       string
		insertions int
		deletions  int
	}
	var fileStats []fileStat
	var currentFile *fileStat
	var kept []string

	for _, line := range lines {
		// Remove "diff --git a/... b/..." lines
		if strings.HasPrefix(line, "diff --git ") {
			continue
		}

		// Remove "index ..." lines
		if indexLineRe.MatchString(line) {
			continue
		}

		// Track files from --- / +++ lines
		if strings.HasPrefix(line, "+++ b/") {
			name := strings.TrimPrefix(line, "+++ b/")
			fs := fileStat{name: name}
			fileStats = append(fileStats, fs)
			currentFile = &fileStats[len(fileStats)-1]
			kept = append(kept, line)
			continue
		}

		if strings.HasPrefix(line, "--- ") {
			kept = append(kept, line)
			continue
		}

		// Hunk headers
		if strings.HasPrefix(line, "@@ ") {
			kept = append(kept, line)
			continue
		}

		// Addition/deletion/context lines
		if strings.HasPrefix(line, "+") {
			if currentFile != nil {
				currentFile.insertions++
			}
			kept = append(kept, line)
			continue
		}
		if strings.HasPrefix(line, "-") {
			if currentFile != nil {
				currentFile.deletions++
			}
			kept = append(kept, line)
			continue
		}

		// Context lines (start with space) and empty lines
		kept = append(kept, line)
	}

	// Build file summary header
	var header []string
	header = append(header, "Files changed:")
	for _, fs := range fileStats {
		header = append(header, fmt.Sprintf("  %s (+%d -%d)", fs.name, fs.insertions, fs.deletions))
	}
	header = append(header, "")

	all := append(header, kept...)
	filtered := strings.Join(all, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	return Result{Filtered: filtered, WasReduced: true}
}

// ---------------------------------------------------------------------------
// GitLogStrategy
// ---------------------------------------------------------------------------

// GitLogStrategy condenses verbose `git log` output into a one-line-per-commit format.
type GitLogStrategy struct{}

func (s *GitLogStrategy) Name() string { return "git-log" }

func (s *GitLogStrategy) CanHandle(command string, args []string) bool {
	return command == "git" && isSubcommand(args, "log")
}

// commitHashRe matches full commit hash lines like "commit abc123...".
var commitHashRe = regexp.MustCompile(`^commit ([0-9a-f]{40})`)

func (s *GitLogStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
	defer func() {
		if r := recover(); r != nil {
			result = Result{Filtered: string(raw), WasReduced: false}
		}
	}()

	cleaned := StripANSIString(string(raw))
	hadTrailing := endsWithNewline(cleaned)

	lines := strings.Split(cleaned, "\n")

	// Check if already --oneline format (no "commit " prefix lines)
	hasFullCommitLine := false
	for _, line := range lines {
		if commitHashRe.MatchString(line) {
			hasFullCommitLine = true
			break
		}
	}
	if !hasFullCommitLine {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Parse commits
	type commitInfo struct {
		shortHash string
		author    string
		date      string
		message   string
	}

	var commits []commitInfo
	var current *commitInfo

	for _, line := range lines {
		if m := commitHashRe.FindStringSubmatch(line); len(m) > 1 {
			if current != nil {
				commits = append(commits, *current)
			}
			current = &commitInfo{shortHash: m[1][:7]}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "Author:") {
			// Extract just the name (before the email)
			authorField := strings.TrimPrefix(line, "Author:")
			authorField = strings.TrimSpace(authorField)
			if idx := strings.Index(authorField, " <"); idx >= 0 {
				authorField = authorField[:idx]
			}
			current.author = authorField
			continue
		}

		if strings.HasPrefix(line, "Date:") {
			current.date = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
			continue
		}

		// Commit message lines are indented with 4 spaces
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && current.message == "" {
			current.message = trimmed
		}
	}
	// Don't forget the last commit
	if current != nil {
		commits = append(commits, *current)
	}

	// Few commits — pass through unchanged
	if len(commits) <= 5 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Build compact output
	var out []string
	for _, c := range commits {
		out = append(out, fmt.Sprintf("%s %s %s: %s", c.shortHash, c.date, c.author, c.message))
	}

	filtered := strings.Join(out, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	return Result{Filtered: filtered, WasReduced: true}
}
