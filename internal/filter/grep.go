package filter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// GrepGroupStrategy
// ---------------------------------------------------------------------------

// GrepGroupStrategy filters grep and rg (ripgrep) output by grouping matches
// by file and providing a summary.
type GrepGroupStrategy struct{}

func (s *GrepGroupStrategy) Name() string { return "grep-group" }

func (s *GrepGroupStrategy) CanHandle(command string, args []string) bool {
	return command == "grep" || command == "rg"
}

// Package-level compiled regexes for GrepGroupStrategy.
var (
	// grepFileLineRe matches grep/rg output lines: filename:linenum:content or filename:content.
	// Limitation: filenames containing colons will be misparsed (the lazy quantifier stops at
	// the first colon). This is a known ambiguity in grep's output format itself.
	grepFileLineRe = regexp.MustCompile(`^(.+?):(\d+:)?(.*)$`)
	// grepBinaryFileRe matches "Binary file X matches" notices
	grepBinaryFileRe = regexp.MustCompile(`^Binary file .+ matches`)
)

const (
	grepMaxLinesPerFile = 8
	grepHeadTail        = 3
)

type fileGroup struct {
	name  string
	lines []string // the full original lines (filename:linenum:content)
}

func (s *GrepGroupStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
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

	// Exit code 1 means "no matches" for grep/rg — pass through
	// Exit code >= 2 means actual error — pass through
	if exitCode != 0 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Parse lines into file groups and binary notices
	groups, binaryNotices := s.parseGroups(lines)

	// If no groups were parsed (all lines are special/binary/separator), pass through
	if len(groups) == 0 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	// Build filtered output
	var output []string
	totalMatches := 0

	for _, grp := range groups {
		matchCount := len(grp.lines)
		totalMatches += matchCount

		// File header with proper pluralization
		matchWord := "matches"
		if matchCount == 1 {
			matchWord = "match"
		}
		output = append(output, fmt.Sprintf("%s (%d %s):", grp.name, matchCount, matchWord))

		// Show matches (truncate if needed)
		if matchCount <= grepMaxLinesPerFile {
			// Show all
			for _, line := range grp.lines {
				output = append(output, "  "+line)
			}
		} else {
			// Show first 3 and last 3
			for i := range grepHeadTail {
				output = append(output, "  "+grp.lines[i])
			}
			omitted := matchCount - (grepHeadTail * 2)
			output = append(output, fmt.Sprintf("  ... %d more", omitted))
			for i := matchCount - grepHeadTail; i < matchCount; i++ {
				output = append(output, "  "+grp.lines[i])
			}
		}
	}

	// Render binary file notices after file groups
	output = append(output, binaryNotices...)

	// Summary footer with proper pluralization
	fileCount := len(groups)
	output = append(output, "")

	matchWord := "matches"
	if totalMatches == 1 {
		matchWord = "match"
	}
	fileWord := "files"
	if fileCount == 1 {
		fileWord = "file"
	}
	output = append(output, fmt.Sprintf("%d %s across %d %s", totalMatches, matchWord, fileCount, fileWord))

	filtered := strings.Join(output, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	// Fix: WasReduced should only be true if output was actually reduced
	wasReduced := len(filtered) < len(cleaned)
	return Result{Filtered: filtered, WasReduced: wasReduced}
}

// parseGroups parses grep/rg output lines into file groups and binary notices.
// Lines that don't match the expected pattern are skipped.
// Binary file notices are returned separately and not counted as file groups.
func (s *GrepGroupStrategy) parseGroups(lines []string) ([]fileGroup, []string) {
	var groups []fileGroup
	var binaryNotices []string
	groupIndex := map[string]int{} // filename -> index in groups

	for _, line := range lines {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Skip separator lines
		if line == "--" {
			continue
		}

		// Binary file notices are tracked separately
		if grepBinaryFileRe.MatchString(line) {
			binaryNotices = append(binaryNotices, line)
			continue
		}

		// Try to parse as filename:linenum:content or filename:content
		matches := grepFileLineRe.FindStringSubmatch(line)
		if matches == nil {
			// Doesn't match expected format — skip
			continue
		}

		filename := matches[1]

		// Check if we've seen this file before
		if idx, ok := groupIndex[filename]; ok {
			// Append to existing group
			groups[idx].lines = append(groups[idx].lines, line)
		} else {
			// Create new group
			groupIndex[filename] = len(groups)
			groups = append(groups, fileGroup{
				name:  filename,
				lines: []string{line},
			})
		}
	}

	return groups, binaryNotices
}
