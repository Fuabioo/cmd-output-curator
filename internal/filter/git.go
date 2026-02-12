package filter

import (
	"fmt"
	"os"
	"regexp"
	"slices"
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
// It understands that certain flags (like git's -c or go's -C) consume the next argument.
// The valueFlags parameter specifies which flags consume a following value argument.
func isSubcommand(args []string, subcmd string, valueFlags map[string]bool) bool {
	skip := false
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if valueFlags[a] {
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
	return command == "git" && isSubcommand(args, "status", gitValueFlags)
}

func (s *GitStatusStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
	filterName := s.Name()
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "coc: filter %s recovered from panic: %v\n", filterName, r)
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

		// Keep "On branch ..." or "HEAD detached ..." line
		if strings.HasPrefix(line, "On branch ") ||
			strings.HasPrefix(line, "HEAD detached at ") ||
			strings.HasPrefix(line, "HEAD detached from ") {
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
	return command == "git" && isSubcommand(args, "diff", gitValueFlags)
}

// indexLineRe matches "index <hash>..<hash>" lines in git diff output.
var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+`)

// binaryFileRe matches binary file diff lines like "Binary files a/foo.png and b/foo.png differ".
var binaryFileRe = regexp.MustCompile(`^Binary files .* differ$`)

// binaryFileNameRe extracts filenames from binary file diff lines (prefers b/ side).
var binaryFileNameRe = regexp.MustCompile(`^Binary files (?:a/\S+ and )?b/(\S+) differ$`)

// binaryFileNameFallbackRe extracts filename from the a/ side when b/ side is /dev/null.
var binaryFileNameFallbackRe = regexp.MustCompile(`^Binary files a/(\S+) and /dev/null differ$`)

func (s *GitDiffStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
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

	// Short diff — pass through unchanged
	if len(lines) < 20 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// First pass: collect file stats and filter lines
	type fileStat struct {
		name       string
		insertions int
		deletions  int
		binary     bool
	}
	var fileStats []fileStat
	var currentFile *fileStat
	var kept []string
	var lastMinusFile string

	for _, line := range lines {
		// Remove "diff --git a/... b/..." lines
		if strings.HasPrefix(line, "diff --git ") {
			continue
		}

		// Remove "index ..." lines
		if indexLineRe.MatchString(line) {
			continue
		}

		// Binary file diffs: "Binary files a/foo.png and b/foo.png differ"
		if binaryFileRe.MatchString(line) {
			name := ""
			if m := binaryFileNameRe.FindStringSubmatch(line); len(m) > 1 {
				name = m[1]
			} else if m := binaryFileNameFallbackRe.FindStringSubmatch(line); len(m) > 1 {
				name = m[1]
			}
			if name != "" {
				fs := fileStat{name: name, binary: true}
				fileStats = append(fileStats, fs)
			}
			kept = append(kept, line)
			continue
		}

		// Track the --- a/filename for use by +++ /dev/null
		if after, ok := strings.CutPrefix(line, "--- a/"); ok {
			lastMinusFile = after
			kept = append(kept, line)
			continue
		}

		// Track files from +++ b/ lines (normal case)
		if after, ok := strings.CutPrefix(line, "+++ b/"); ok {
			fs := fileStat{name: after}
			fileStats = append(fileStats, fs)
			currentFile = &fileStats[len(fileStats)-1]
			kept = append(kept, line)
			continue
		}

		// Handle +++ /dev/null (file deletion) — must come before generic "+" counting
		if strings.HasPrefix(line, "+++ ") {
			// This handles "+++ /dev/null" and any other non-"b/" +++ lines
			if lastMinusFile != "" {
				fs := fileStat{name: lastMinusFile}
				fileStats = append(fileStats, fs)
				currentFile = &fileStats[len(fileStats)-1]
			}
			kept = append(kept, line)
			continue
		}

		// Handle --- /dev/null and other non-"a/" --- lines
		if strings.HasPrefix(line, "--- ") {
			lastMinusFile = ""
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
		if fs.binary {
			header = append(header, fmt.Sprintf("  %s (binary)", fs.name))
		} else {
			header = append(header, fmt.Sprintf("  %s (+%d -%d)", fs.name, fs.insertions, fs.deletions))
		}
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
	return command == "git" && isSubcommand(args, "log", gitValueFlags)
}

// commitHashRe matches full commit hash lines like "commit abc123...".
var commitHashRe = regexp.MustCompile(`^commit ([0-9a-f]{40})`)

func (s *GitLogStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
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

	// Check if already --oneline format (no "commit " prefix lines)
	hasFullCommitLine := slices.ContainsFunc(lines, func(line string) bool {
		return commitHashRe.MatchString(line)
	})
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

		if after, ok := strings.CutPrefix(line, "Author:"); ok {
			// Extract just the name (before the email)
			authorField := strings.TrimSpace(after)
			if idx := strings.Index(authorField, " <"); idx >= 0 {
				authorField = authorField[:idx]
			}
			current.author = authorField
			continue
		}

		if after, ok := strings.CutPrefix(line, "Date:"); ok {
			current.date = strings.TrimSpace(after)
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
