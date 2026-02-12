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

	t.Run("default registry returns passthrough", func(t *testing.T) {
		r := DefaultRegistry()
		s := r.Find("anything", nil)
		if s.Name() != "passthrough" {
			t.Errorf("DefaultRegistry.Find = %q, want passthrough", s.Name())
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
