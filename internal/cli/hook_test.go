package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fuabioo/coc/internal/filter"
)

func TestContainsShellOps(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Should detect shell ops
		{"pipe", "git diff | head", true},
		{"and chain", "git status && echo done", true},
		{"or chain", "git log || true", true},
		{"semicolon", "git status; echo done", true},
		{"command substitution dollar", "echo $(git status)", true},
		{"command substitution backtick", "echo `git status`", true},
		{"multiple ops", "git diff | grep foo && echo found", true},

		// Should NOT detect shell ops
		{"simple command", "git status", false},
		{"command with flags", "git diff --cached", false},
		{"command with args", "go test ./...", false},
		{"empty", "", false},
		{"whitespace only", "   ", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := containsShellOps(tc.command)
			if got != tc.want {
				t.Errorf("containsShellOps(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestExtractFirstWord(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"single word", "git", "git"},
		{"command with args", "git status", "git"},
		{"command with flags", "git diff --cached", "git"},
		{"leading spaces", "  git status", "git"},
		{"trailing spaces", "git status  ", "git"},
		{"multiple spaces", "git   status", "git"},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractFirstWord(tc.command)
			if got != tc.want {
				t.Errorf("extractFirstWord(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}

func TestIsSupportedCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Supported commands (from cocSupportedCommands)
		{"git", "git", true},
		{"go", "go", true},
		{"cargo", "cargo", true},
		{"docker", "docker", true},
		{"grep", "grep", true},
		{"rg", "rg", true},
		{"npm", "npm", true},
		{"pip", "pip", true},
		{"pip3", "pip3", true},
		{"yarn", "yarn", true},

		// Not supported
		{"echo", "echo", false},
		{"ls", "ls", false},
		{"curl", "curl", false},
		{"make", "make", false},
		{"python", "python", false},
		{"coc", "coc", false},
		{"empty", "", false},
		{"partial match", "gitk", false},
		{"case sensitive", "GIT", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isSupportedCommand(tc.command)
			if got != tc.want {
				t.Errorf("isSupportedCommand(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestHookInputParsing(t *testing.T) {
	t.Run("valid bash tool input", func(t *testing.T) {
		input := `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
		var parsed hookInput
		err := json.Unmarshal([]byte(input), &parsed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed.ToolName != "Bash" {
			t.Errorf("ToolName = %q, want %q", parsed.ToolName, "Bash")
		}
		if parsed.ToolInput.Command != "git status" {
			t.Errorf("Command = %q, want %q", parsed.ToolInput.Command, "git status")
		}
	})

	t.Run("non-bash tool", func(t *testing.T) {
		input := `{"tool_name":"Read","tool_input":{"file_path":"/tmp/test"}}`
		var parsed hookInput
		err := json.Unmarshal([]byte(input), &parsed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed.ToolName != "Read" {
			t.Errorf("ToolName = %q, want %q", parsed.ToolName, "Read")
		}
		// Command field should be empty for non-Bash tools
		if parsed.ToolInput.Command != "" {
			t.Errorf("Command should be empty for non-Bash tool, got %q", parsed.ToolInput.Command)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		var parsed hookInput
		err := json.Unmarshal([]byte(""), &parsed)
		if err == nil {
			t.Error("expected error for empty input")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		var parsed hookInput
		err := json.Unmarshal([]byte("not json"), &parsed)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("missing command field", func(t *testing.T) {
		input := `{"tool_name":"Bash","tool_input":{}}`
		var parsed hookInput
		err := json.Unmarshal([]byte(input), &parsed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed.ToolInput.Command != "" {
			t.Errorf("missing command should be empty string, got %q", parsed.ToolInput.Command)
		}
	})
}

func TestHookOutputGeneration(t *testing.T) {
	t.Run("valid output structure", func(t *testing.T) {
		var output hookOutput
		output.HookSpecificOutput.HookEventName = "PreToolUse"
		output.HookSpecificOutput.PermissionDecision = "allow"
		output.HookSpecificOutput.UpdatedInput.Command = "coc git status"

		outputBytes, err := json.Marshal(output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should be valid JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal(outputBytes, &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}

		// Check structure
		hookOutput, ok := parsed["hookSpecificOutput"].(map[string]interface{})
		if !ok {
			t.Fatal("missing hookSpecificOutput")
		}
		if hookOutput["hookEventName"] != "PreToolUse" {
			t.Errorf("hookEventName = %v, want PreToolUse", hookOutput["hookEventName"])
		}
		if hookOutput["permissionDecision"] != "allow" {
			t.Errorf("permissionDecision = %v, want allow", hookOutput["permissionDecision"])
		}
		updatedInput, ok := hookOutput["updatedInput"].(map[string]interface{})
		if !ok {
			t.Fatal("missing updatedInput")
		}
		if updatedInput["command"] != "coc git status" {
			t.Errorf("command = %v, want 'coc git status'", updatedInput["command"])
		}
	})

	t.Run("empty command", func(t *testing.T) {
		var output hookOutput
		output.HookSpecificOutput.HookEventName = "PreToolUse"
		output.HookSpecificOutput.PermissionDecision = "allow"
		output.HookSpecificOutput.UpdatedInput.Command = ""

		_, err := json.Marshal(output)
		if err != nil {
			t.Fatalf("marshaling should succeed even with empty command: %v", err)
		}
	})
}

func TestShouldWrapCommandIntegration(t *testing.T) {
	// This is an integration test that combines all the helper functions
	// to determine if a command should be wrapped with coc.
	shouldWrap := func(command string) bool {
		trimmed := strings.TrimSpace(command)
		if trimmed == "" {
			return false
		}
		if containsShellOps(trimmed) {
			return false
		}
		firstWord := extractFirstWord(trimmed)
		if firstWord == "" {
			return false
		}
		if firstWord == "coc" {
			return false
		}
		return isSupportedCommand(firstWord)
	}

	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Should wrap
		{"git status", "git status", true},
		{"git diff --cached", "git diff --cached", true},
		{"go test ./...", "go test ./...", true},
		{"go build", "go build", true},
		{"cargo test", "cargo test", true},
		{"cargo build --release", "cargo build --release", true},
		{"docker build .", "docker build .", true},
		{"docker pull alpine", "docker pull alpine", true},
		{"grep -rn pattern .", "grep -rn pattern .", true},
		{"rg pattern", "rg pattern", true},
		{"npm install", "npm install", true},
		{"pip install requests", "pip install requests", true},
		{"pip3 install flask", "pip3 install flask", true},
		{"yarn add lodash", "yarn add lodash", true},
		{"git with leading space", "  git status", true},
		{"git bare", "git", true},

		// Should NOT wrap - already has coc prefix
		{"coc git status", "coc git status", false},
		{"coc go test", "coc go test", false},

		// Should NOT wrap - not a known command
		{"echo hello", "echo hello", false},
		{"ls -la", "ls -la", false},
		{"curl https://example.com", "curl https://example.com", false},
		{"make build", "make build", false},
		{"python script.py", "python script.py", false},

		// Should NOT wrap - pipelines and chains
		{"git diff | head", "git diff | head", false},
		{"git status && echo done", "git status && echo done", false},
		{"git log || true", "git log || true", false},
		{"git status; echo done", "git status; echo done", false},
		{"echo $(git status)", "echo $(git status)", false},
		{"echo `git status`", "echo `git status`", false},

		// Should NOT wrap - empty/whitespace
		{"empty", "", false},
		{"whitespace only", "   ", false},

		// Edge cases
		{"cocaine app", "cocaine start", false}, // "cocaine" is not "coc "
		{"coc standalone", "coc", false},        // just "coc" alone, no command
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldWrap(tc.command)
			if got != tc.want {
				t.Errorf("shouldWrap(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

// TestCocSupportedCommandsMatchRegistry verifies that every command listed in
// cocSupportedCommands has at least one non-passthrough strategy in the default
// filter registry. This catches drift between the hook's supported commands list
// and the filter strategies that actually handle those commands.
func TestCocSupportedCommandsMatchRegistry(t *testing.T) {
	registry := filter.DefaultRegistry()
	for _, cmd := range cocSupportedCommands {
		strategy := registry.Find(cmd, nil)
		if strategy.Name() == "passthrough" {
			t.Errorf("cocSupportedCommands contains %q but no filter strategy handles it (got passthrough)", cmd)
		}
	}
}
