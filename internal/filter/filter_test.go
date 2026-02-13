package filter

import "testing"

func TestPassthroughStrategy(t *testing.T) {
	p := &PassthroughStrategy{}

	t.Run("name", func(t *testing.T) {
		if got := p.Name(); got != "passthrough" {
			t.Errorf("Name() = %q, want passthrough", got)
		}
	})

	t.Run("can handle anything", func(t *testing.T) {
		if !p.CanHandle("anything", nil) {
			t.Error("CanHandle should return true for any command")
		}
	})

	t.Run("can handle empty command", func(t *testing.T) {
		if !p.CanHandle("", nil) {
			t.Error("CanHandle should return true for empty command")
		}
	})

	t.Run("filter returns unchanged", func(t *testing.T) {
		raw := []byte("hello world\n")
		result := p.Filter(raw, "echo", []string{"hello", "world"}, 0)
		if result.Filtered != "hello world\n" {
			t.Errorf("Filter() = %q, want %q", result.Filtered, "hello world\n")
		}
		if result.WasReduced {
			t.Error("Passthrough should not reduce output")
		}
	})

	t.Run("filter empty input", func(t *testing.T) {
		result := p.Filter(nil, "cmd", nil, 0)
		if result.Filtered != "" {
			t.Errorf("Filter(nil) = %q, want empty", result.Filtered)
		}
		if result.WasReduced {
			t.Error("Passthrough should not reduce empty output")
		}
	})

	t.Run("filter preserves exit code context", func(t *testing.T) {
		raw := []byte("error output")
		result := p.Filter(raw, "cmd", nil, 1)
		if result.Filtered != "error output" {
			t.Errorf("Filter with nonzero exit = %q, want %q", result.Filtered, "error output")
		}
		if result.WasReduced {
			t.Error("Passthrough should not reduce output regardless of exit code")
		}
	})

	t.Run("filter multiline", func(t *testing.T) {
		raw := []byte("line1\nline2\nline3\n")
		result := p.Filter(raw, "cmd", nil, 0)
		if result.Filtered != "line1\nline2\nline3\n" {
			t.Errorf("Filter multiline = %q, want %q", result.Filtered, "line1\nline2\nline3\n")
		}
	})
}

func TestRegistry(t *testing.T) {
	t.Run("empty registry returns passthrough", func(t *testing.T) {
		r := NewRegistry()
		s := r.Find("git", []string{"status"})
		if s.Name() != "passthrough" {
			t.Errorf("Find on empty registry = %q, want passthrough", s.Name())
		}
	})

	t.Run("default registry returns generic-error for unknown commands", func(t *testing.T) {
		r := DefaultRegistry()
		s := r.Find("anything", nil)
		if s.Name() != "generic-error" {
			t.Errorf("DefaultRegistry.Find = %q, want generic-error", s.Name())
		}
	})

	t.Run("first match wins", func(t *testing.T) {
		mock1 := &mockStrategy{name: "mock1", canHandle: true}
		mock2 := &mockStrategy{name: "mock2", canHandle: true}
		r := NewRegistry(mock1, mock2)
		s := r.Find("cmd", nil)
		if s.Name() != "mock1" {
			t.Errorf("Find should return first match, got %q", s.Name())
		}
	})

	t.Run("skips non-matching", func(t *testing.T) {
		mock1 := &mockStrategy{name: "no-match", canHandle: false}
		mock2 := &mockStrategy{name: "match", canHandle: true}
		r := NewRegistry(mock1, mock2)
		s := r.Find("cmd", nil)
		if s.Name() != "match" {
			t.Errorf("Find should skip non-matching, got %q", s.Name())
		}
	})

	t.Run("falls back to passthrough when none match", func(t *testing.T) {
		mock := &mockStrategy{name: "no-match", canHandle: false}
		r := NewRegistry(mock)
		s := r.Find("cmd", nil)
		if s.Name() != "passthrough" {
			t.Errorf("Find should fall back to passthrough, got %q", s.Name())
		}
	})

	t.Run("many strategies none match", func(t *testing.T) {
		mocks := make([]Strategy, 10)
		for i := range mocks {
			mocks[i] = &mockStrategy{name: "nope", canHandle: false}
		}
		r := NewRegistry(mocks...)
		s := r.Find("cmd", nil)
		if s.Name() != "passthrough" {
			t.Errorf("Find with many non-matching should fall back to passthrough, got %q", s.Name())
		}
	})

	t.Run("registry passes command and args to CanHandle", func(t *testing.T) {
		m := &recordingStrategy{}
		r := NewRegistry(m)
		r.Find("git", []string{"status", "-s"})
		if m.lastCommand != "git" {
			t.Errorf("CanHandle received command %q, want %q", m.lastCommand, "git")
		}
		if len(m.lastArgs) != 2 || m.lastArgs[0] != "status" || m.lastArgs[1] != "-s" {
			t.Errorf("CanHandle received args %v, want [status -s]", m.lastArgs)
		}
	})
}

func TestRegistryPriority(t *testing.T) {
	r := DefaultRegistry()

	tests := []struct {
		name         string
		command      string
		args         []string
		wantStrategy string
	}{
		// Git strategies
		{"git status", "git", []string{"status"}, "git-status"},
		{"git status short", "git", []string{"status", "-s"}, "git-status"},
		{"git diff", "git", []string{"diff"}, "git-diff"},
		{"git diff cached", "git", []string{"diff", "--cached"}, "git-diff"},
		{"git log", "git", []string{"log"}, "git-log"},
		{"git log oneline", "git", []string{"log", "--oneline"}, "git-log"},
		// Go strategies
		{"go test", "go", []string{"test"}, "go-test"},
		{"go test all", "go", []string{"test", "./..."}, "go-test"},
		{"go build", "go", []string{"build"}, "go-build"},
		{"go build all", "go", []string{"build", "./..."}, "go-build"},
		{"go vet", "go", []string{"vet"}, "go-build"},
		{"go vet all", "go", []string{"vet", "./..."}, "go-build"},
		{"go install", "go", []string{"install"}, "go-build"},
		// Cargo strategies
		{"cargo test", "cargo", []string{"test"}, "cargo-test"},
		{"cargo test all", "cargo", []string{"test", "--all"}, "cargo-test"},
		{"cargo build", "cargo", []string{"build"}, "cargo-build"},
		{"cargo check", "cargo", []string{"check"}, "cargo-build"},
		{"cargo clippy", "cargo", []string{"clippy"}, "cargo-build"},
		// Docker strategies
		{"docker build", "docker", []string{"build", "."}, "docker-build"},
		{"docker compose build", "docker", []string{"compose", "build"}, "docker-build"},
		// Grep/rg strategies
		{"grep pattern", "grep", []string{"-rn", "pattern", "."}, "grep-group"},
		{"rg pattern", "rg", []string{"pattern"}, "grep-group"},
		// Progress strip strategies
		{"npm install", "npm", []string{"install"}, "progress-strip"},
		{"docker pull", "docker", []string{"pull", "alpine"}, "progress-strip"},
		{"pip install", "pip", []string{"install", "requests"}, "progress-strip"},
		// Unknown commands should get generic-error (last registered strategy that matches anything)
		{"unknown command", "unknown", nil, "generic-error"},
		{"npm test", "npm", []string{"test"}, "generic-error"},
		// Git subcommands without specific strategies should get generic-error
		{"git commit", "git", []string{"commit"}, "generic-error"},
		{"git push", "git", []string{"push"}, "generic-error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := r.Find(tc.command, tc.args)
			if s.Name() != tc.wantStrategy {
				t.Errorf("Find(%q, %v) = %q, want %q", tc.command, tc.args, s.Name(), tc.wantStrategy)
			}
		})
	}
}

// mockStrategy is a test helper implementing Strategy.
type mockStrategy struct {
	name      string
	canHandle bool
}

func (m *mockStrategy) Name() string                        { return m.name }
func (m *mockStrategy) CanHandle(_ string, _ []string) bool { return m.canHandle }
func (m *mockStrategy) Filter(raw []byte, _ string, _ []string, _ int) Result {
	return Result{Filtered: string(raw), WasReduced: false}
}

// recordingStrategy records what was passed to CanHandle.
type recordingStrategy struct {
	lastCommand string
	lastArgs    []string
}

func (r *recordingStrategy) Name() string { return "recording" }
func (r *recordingStrategy) CanHandle(command string, args []string) bool {
	r.lastCommand = command
	r.lastArgs = args
	return true
}
func (r *recordingStrategy) Filter(raw []byte, _ string, _ []string, _ int) Result {
	return Result{Filtered: string(raw), WasReduced: false}
}
