package filter

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// dockerValueFlags are docker global flags that consume the next argument as a value.
var dockerValueFlags = map[string]bool{
	"-H": true, "--host": true, "--config": true, "--context": true,
	"-l": true, "--log-level": true,
}

// dockerSubcmdValueFlags are subcommand-specific flags that consume a following value argument.
var dockerSubcmdValueFlags = map[string]map[string]bool{
	"buildx":  {"--builder": true, "--platform": true},
	"compose": {"-f": true, "--file": true, "-p": true, "--project-name": true, "--profile": true},
}

// dockerSubcommands returns the first two non-flag positional args from args,
// skipping flags and their values (identified by dockerValueFlags and subcommand-specific flags).
func dockerSubcommands(args []string, valueFlags map[string]bool) (string, string) {
	var positional []string
	skip := false
	subcmdFlags := map[string]bool{} // will be populated after first positional
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if valueFlags[a] || subcmdFlags[a] {
			skip = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		positional = append(positional, a)
		if len(positional) == 1 {
			// After first positional, check for subcommand-specific flags
			if flags, ok := dockerSubcmdValueFlags[a]; ok {
				subcmdFlags = flags
			}
		}
		if len(positional) == 2 {
			break
		}
	}
	first := ""
	second := ""
	if len(positional) > 0 {
		first = positional[0]
	}
	if len(positional) > 1 {
		second = positional[1]
	}
	return first, second
}

// ---------------------------------------------------------------------------
// DockerBuildStrategy
// ---------------------------------------------------------------------------

// DockerBuildStrategy filters `docker build`, `docker buildx build`,
// and `docker compose build` output.
type DockerBuildStrategy struct{}

func (s *DockerBuildStrategy) Name() string { return "docker-build" }

func (s *DockerBuildStrategy) CanHandle(command string, args []string) bool {
	if command != "docker" {
		return false
	}
	// Simple "docker build" (via isSubcommand)
	if isSubcommand(args, "build", dockerValueFlags) {
		return true
	}
	// Multi-word commands: "docker buildx build" or "docker compose build"
	first, second := dockerSubcommands(args, dockerValueFlags)
	if (first == "buildx" || first == "compose") && second == "build" {
		return true
	}
	return false
}

// Package-level compiled regexes for DockerBuildStrategy.
var (
	dockerLegacyHashRe      = regexp.MustCompile(`^\s*---> [0-9a-f]`)
	dockerRemoveContainerRe = regexp.MustCompile(`^Removing intermediate container`)
	dockerSendContextRe     = regexp.MustCompile(`^Sending build context`)
	dockerUsingCacheRe      = regexp.MustCompile(`---> Using cache`)
	dockerStepRe            = regexp.MustCompile(`^Step \d+/\d+`)
	dockerSuccessBuiltRe    = regexp.MustCompile(`^Successfully built`)
	dockerSuccessTaggedRe   = regexp.MustCompile(`^Successfully tagged`)
	dockerCopyRe            = regexp.MustCompile(`^COPY`)
	dockerRunRe             = regexp.MustCompile(`^RUN`)
	dockerFromRe            = regexp.MustCompile(`^FROM`)
	dockerBuildKitLineRe    = regexp.MustCompile(`^#\d+`)
	dockerBuildKitDoneRe    = regexp.MustCompile(`DONE`)
	dockerBuildKitErrorRe   = regexp.MustCompile(`ERROR`)
	dockerBuildKitCachedRe  = regexp.MustCompile(`CACHED`)
	dockerBuildKitSha256Re  = regexp.MustCompile(`^#\d+\s+sha256:`)
	dockerBuildKitTransfRe  = regexp.MustCompile(`\d+(\.\d+)?\s*(MB|KB|GB|B)\b`)
	dockerErrorLineRe       = regexp.MustCompile(`(?i)\b(error|failed)\b`)
	dockerArrowRe           = regexp.MustCompile(`^\s*-->`)
)

func (s *DockerBuildStrategy) Filter(raw []byte, command string, args []string, exitCode int) (result Result) {
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
	if len(lines) < 15 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	if exitCode == 0 {
		return s.filterSuccess(lines, cleaned, hadTrailing)
	}
	return s.filterFailure(lines, cleaned, hadTrailing)
}

func (s *DockerBuildStrategy) filterSuccess(lines []string, cleaned string, hadTrailing bool) Result {
	var kept []string

	for _, line := range lines {
		// Strip legacy builder noise
		if dockerLegacyHashRe.MatchString(line) {
			continue
		}
		if dockerRemoveContainerRe.MatchString(line) {
			continue
		}
		if dockerSendContextRe.MatchString(line) {
			continue
		}
		if dockerUsingCacheRe.MatchString(line) {
			continue
		}

		// Keep Dockerfile instruction lines
		if dockerStepRe.MatchString(line) ||
			dockerSuccessBuiltRe.MatchString(line) ||
			dockerSuccessTaggedRe.MatchString(line) ||
			dockerCopyRe.MatchString(line) ||
			dockerRunRe.MatchString(line) ||
			dockerFromRe.MatchString(line) {
			kept = append(kept, line)
			continue
		}

		// BuildKit output
		if dockerBuildKitLineRe.MatchString(line) {
			// Strip sha256 hash lines
			if dockerBuildKitSha256Re.MatchString(line) {
				continue
			}
			// Strip transfer byte count lines (lines that are just transfer info)
			if dockerBuildKitTransfRe.MatchString(line) &&
				!dockerBuildKitDoneRe.MatchString(line) &&
				!dockerBuildKitErrorRe.MatchString(line) &&
				!dockerBuildKitCachedRe.MatchString(line) {
				continue
			}
			// Keep lines with DONE, ERROR, CACHED
			if dockerBuildKitDoneRe.MatchString(line) ||
				dockerBuildKitErrorRe.MatchString(line) ||
				dockerBuildKitCachedRe.MatchString(line) {
				kept = append(kept, line)
				continue
			}
			// Other BuildKit lines — strip
			continue
		}

		// Keep everything else that wasn't explicitly stripped
		kept = append(kept, line)
	}

	// If nothing was stripped, passthrough
	if len(kept) >= len(lines) {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	filtered := strings.Join(kept, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	wasReduced := len(filtered) < len(cleaned)
	return Result{Filtered: filtered, WasReduced: wasReduced}
}

func (s *DockerBuildStrategy) filterFailure(lines []string, cleaned string, hadTrailing bool) Result {
	// Collect pattern-matched lines
	patternKept := make(map[int]bool)
	for i, line := range lines {
		if dockerErrorLineRe.MatchString(line) {
			patternKept[i] = true
			continue
		}
		// BuildKit #N ERROR lines
		if dockerBuildKitLineRe.MatchString(line) && dockerBuildKitErrorRe.MatchString(line) {
			patternKept[i] = true
			continue
		}
		// Dockerfile pointer lines
		if dockerArrowRe.MatchString(line) {
			patternKept[i] = true
			continue
		}
	}

	// Collect last 10 non-empty lines
	var nonEmptyIndices []int
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyIndices = append(nonEmptyIndices, i)
		}
	}
	lastNStart := 0
	if len(nonEmptyIndices) > 10 {
		lastNStart = len(nonEmptyIndices) - 10
	}
	lastNSet := make(map[int]bool)
	for _, idx := range nonEmptyIndices[lastNStart:] {
		lastNSet[idx] = true
	}

	// Merge both sets (deduplicate: each line appears once)
	included := make(map[int]bool)
	for idx := range patternKept {
		included[idx] = true
	}
	for idx := range lastNSet {
		included[idx] = true
	}

	// Build output preserving order
	var kept []string
	for i := range lines {
		if included[i] {
			kept = append(kept, lines[i])
		}
	}

	// If nothing was stripped, passthrough
	if len(kept) >= len(lines) {
		return Result{Filtered: cleaned, WasReduced: false}
	}
	if len(kept) == 0 {
		return Result{Filtered: cleaned, WasReduced: false}
	}

	filtered := strings.Join(kept, "\n")
	filtered = ensureTrailingNewline(filtered, hadTrailing)

	wasReduced := len(filtered) < len(cleaned)
	return Result{Filtered: filtered, WasReduced: wasReduced}
}
