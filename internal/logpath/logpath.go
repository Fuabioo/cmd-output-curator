package logpath

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Resolve returns the full log file path for a given command and args.
// Priority: flagDir > envDir > default (os.TempDir()/coc).
func Resolve(flagDir string, command string, args []string) string {
	dir := baseDir(flagDir)
	slug := Slug(command, args)
	sessionID := SessionID()
	return filepath.Join(dir, slug, sessionID+".log")
}

// baseDir determines the log directory from flag, env, or default.
func baseDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	if envDir := os.Getenv("COC_LOG_DIR"); envDir != "" {
		return envDir
	}
	return filepath.Join(os.TempDir(), "coc")
}

// slugUnsafe matches any character not in the safe set for slug parts.
var slugUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// slugDashes collapses consecutive dashes into a single dash.
var slugDashes = regexp.MustCompile(`-{2,}`)

// sanitizeSlugPart lowercases and sanitizes a single slug component.
func sanitizeSlugPart(s string) string {
	s = strings.ToLower(s)
	s = slugUnsafe.ReplaceAllString(s, "-")
	s = slugDashes.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// Slug generates a directory name from the command and its first non-flag subcommand.
// Examples: ("git", ["status"]) -> "git-status", ("go", ["test", "./..."]) -> "go-test"
func Slug(command string, args []string) string {
	base := filepath.Base(command)
	parts := []string{sanitizeSlugPart(base)}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		parts = append(parts, sanitizeSlugPart(filepath.Base(arg)))
		if len(parts) >= 2 {
			break
		}
	}
	slug := strings.Join(parts, "-")
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}

// SessionID generates a timestamp-based session ID with a random suffix.
// Format: YYYYMMDD-HHMMSS-XXXX where XXXX is 4 random hex chars.
func SessionID() string {
	now := time.Now()
	suffix, err := randomHex(2)
	if err != nil {
		suffix = "0000"
	}
	return fmt.Sprintf("%s-%s", now.Format("20060102-150405"), suffix)
}

// randomHex generates n random bytes and returns them as a hex string.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// CreateLogFile creates the log file at the given path, including parent directories.
func CreateLogFile(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating log directory %s: %w", dir, err)
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating log file %s: %w", path, err)
	}
	return f, nil
}
