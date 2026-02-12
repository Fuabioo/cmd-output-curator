package logpath

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{"simple command", "git", []string{"status"}, "git-status"},
		{"command with path", "/usr/bin/git", []string{"status"}, "git-status"},
		{"skip flags", "go", []string{"-v", "test", "./..."}, "go-test"},
		{"no args", "ls", nil, "ls"},
		{"only flags", "ls", []string{"-la"}, "ls"},
		{"spaces in arg", "echo", []string{"hello world"}, "echo-hello-world"},
		{"path in arg uses basename", "go", []string{"test", "./internal/..."}, "go-test"},
		{"docker compose", "docker", []string{"compose", "up"}, "docker-compose"},
		{"long slug truncation", "cmd", []string{strings.Repeat("a", 100)}, ""},
		{"special chars", "node", []string{"my script!@#.js"}, "node-my-script-.js"},
		{"path traversal in arg", "git", []string{"../../etc"}, "git-etc"},
		{"uppercase normalized", "Git", []string{"Status"}, "git-status"},
		{"empty command", "", nil, "."},
		{"dot args", "cmd", []string{"."}, "cmd-."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slug(tt.command, tt.args)

			// Always check: no path separators in slug
			if strings.Contains(got, "/") || strings.Contains(got, "\\") {
				t.Errorf("Slug(%q, %v) = %q, contains path separator", tt.command, tt.args, got)
			}

			// Always check: length <= 64
			if len(got) > 64 {
				t.Errorf("Slug(%q, %v) = %q, length %d > 64", tt.command, tt.args, got, len(got))
			}

			// Always check: no spaces
			if strings.Contains(got, " ") {
				t.Errorf("Slug(%q, %v) = %q, contains space", tt.command, tt.args, got)
			}

			// Check expected value where specified
			if tt.want != "" && got != tt.want {
				t.Errorf("Slug(%q, %v) = %q, want %q", tt.command, tt.args, got, tt.want)
			}
		})
	}
}

func TestSlugOnlyFirstNonFlagArg(t *testing.T) {
	// Slug should only include the command + first non-flag arg
	got := Slug("go", []string{"test", "./...", "-count=1"})
	if got != "go-test" {
		t.Errorf("Slug should only take first non-flag arg, got %q, want %q", got, "go-test")
	}
}

func TestSessionID(t *testing.T) {
	id := SessionID()

	// Format: YYYYMMDD-HHMMSS-XXXX
	pattern := `^\d{8}-\d{6}-[0-9a-f]{4}$`
	matched, err := regexp.MatchString(pattern, id)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Errorf("SessionID() = %q, does not match pattern %s", id, pattern)
	}

	// Two calls should produce different IDs (random suffix)
	id2 := SessionID()
	if id == id2 {
		// Could happen in same second with same random, but extremely unlikely
		t.Logf("Warning: two consecutive SessionID() calls returned same value: %s", id)
	}
}

func TestResolve(t *testing.T) {
	// Test with explicit flag dir
	path := Resolve("/custom/dir", "git", []string{"status"})
	if !strings.HasPrefix(path, "/custom/dir/git-status/") {
		t.Errorf("Resolve with flagDir: got %q, want prefix /custom/dir/git-status/", path)
	}
	if !strings.HasSuffix(path, ".log") {
		t.Errorf("Resolve: got %q, want .log suffix", path)
	}

	// Verify the session ID part of the filename
	// Path format: <dir>/<slug>/<sessionID>.log
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]
	sessionPart := strings.TrimSuffix(filename, ".log")
	pattern := `^\d{8}-\d{6}-[0-9a-f]{4}$`
	matched, err := regexp.MatchString(pattern, sessionPart)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Errorf("Resolve filename session part %q does not match pattern %s", sessionPart, pattern)
	}
}

func TestBaseDir(t *testing.T) {
	// Test flag override
	got := baseDir("/flag/dir")
	if got != "/flag/dir" {
		t.Errorf("baseDir with flag = %q, want /flag/dir", got)
	}

	// Test env override
	if err := os.Setenv("COC_LOG_DIR", "/env/dir"); err != nil {
		t.Fatalf("setenv error: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("COC_LOG_DIR"); err != nil {
			t.Fatalf("unsetenv error: %v", err)
		}
	}()
	got = baseDir("")
	if got != "/env/dir" {
		t.Errorf("baseDir with env = %q, want /env/dir", got)
	}

	// Test default (unset env)
	if err := os.Unsetenv("COC_LOG_DIR"); err != nil {
		t.Fatalf("unsetenv error: %v", err)
	}
	got = baseDir("")
	if got == "" {
		t.Error("baseDir default should not be empty")
	}
	// Default should end with /coc
	if !strings.HasSuffix(got, "/coc") {
		t.Errorf("baseDir default = %q, want suffix /coc", got)
	}
}

func TestBaseDirFlagTakesPriorityOverEnv(t *testing.T) {
	if err := os.Setenv("COC_LOG_DIR", "/env/dir"); err != nil {
		t.Fatalf("setenv error: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("COC_LOG_DIR"); err != nil {
			t.Fatalf("unsetenv error: %v", err)
		}
	}()

	got := baseDir("/flag/dir")
	if got != "/flag/dir" {
		t.Errorf("baseDir should prefer flag over env, got %q, want /flag/dir", got)
	}
}

func TestSanitizeSlugPart(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "Hello", "hello"},
		{"spaces to dash", "hello world", "hello-world"},
		{"multiple unsafe chars collapse", "a!!b", "a-b"},
		{"trim leading dash", "-hello", "hello"},
		{"trim trailing dash", "hello-", "hello"},
		{"dots preserved", "file.txt", "file.txt"},
		{"underscores preserved", "my_file", "my_file"},
		{"dashes preserved", "my-file", "my-file"},
		{"multiple dashes collapsed", "a---b", "a-b"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSlugPart(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeSlugPart(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
