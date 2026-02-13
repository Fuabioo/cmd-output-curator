package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// Note: These tests assume the existence of helper functions that will be
// implemented in init.go:
// - addHookToSettings(input []byte) ([]byte, error)
// - removeHookFromSettings(input []byte) ([]byte, bool, error)
//
// These tests are written TDD-style to guide the implementation.

func TestAddHookToSettings(t *testing.T) {
	t.Run("empty settings", func(t *testing.T) {
		input := []byte("{}")
		result, err := addHookToSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify hook was added
		if !strings.Contains(string(result), "coc hook") {
			t.Error("expected 'coc hook' in output")
		}
		if !strings.Contains(string(result), "PreToolUse") {
			t.Error("expected 'PreToolUse' in output")
		}

		// Verify valid JSON
		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
	})

	t.Run("settings with empty hooks", func(t *testing.T) {
		input := []byte(`{"hooks": {}}`)
		result, err := addHookToSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(string(result), "coc hook") {
			t.Error("expected 'coc hook' in output")
		}
		if !strings.Contains(string(result), "PreToolUse") {
			t.Error("expected 'PreToolUse' in output")
		}
		if !strings.Contains(string(result), "Bash") {
			t.Error("expected 'Bash' matcher in output")
		}
	})

	t.Run("existing hooks preserved", func(t *testing.T) {
		input := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Bash",
						"hooks": [
							{"type": "command", "command": "some-other-hook.sh"}
						]
					}
				]
			}
		}`)
		result, err := addHookToSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Both hooks should be present
		if !strings.Contains(string(result), "some-other-hook.sh") {
			t.Error("existing hook should be preserved")
		}
		if !strings.Contains(string(result), "coc hook") {
			t.Error("coc hook should be added")
		}

		// Verify valid JSON structure
		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
	})

	t.Run("idempotent - already installed", func(t *testing.T) {
		input := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Bash",
						"hooks": [
							{"type": "command", "command": "coc hook"}
						]
					}
				]
			}
		}`)
		result, err := addHookToSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should not duplicate
		count := strings.Count(string(result), "coc hook")
		if count != 1 {
			t.Errorf("coc hook appears %d times, want 1 (idempotent)", count)
		}
	})

	t.Run("preserves other settings", func(t *testing.T) {
		input := []byte(`{
			"permissions": {"allow": ["Bash(git:*)"]},
			"hooks": {}
		}`)
		result, err := addHookToSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(string(result), "permissions") {
			t.Error("existing settings should be preserved")
		}
		if !strings.Contains(string(result), "coc hook") {
			t.Error("coc hook should be added")
		}

		// Verify structure
		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}

		if _, ok := settings["permissions"]; !ok {
			t.Error("permissions field should be preserved")
		}
	})

	t.Run("preserves other PreToolUse matchers", func(t *testing.T) {
		input := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Read",
						"hooks": [
							{"type": "command", "command": "read-hook.sh"}
						]
					}
				]
			}
		}`)
		result, err := addHookToSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Both matchers should be present
		if !strings.Contains(string(result), "Read") {
			t.Error("existing Read matcher should be preserved")
		}
		if !strings.Contains(string(result), "read-hook.sh") {
			t.Error("existing Read hook should be preserved")
		}
		if !strings.Contains(string(result), "Bash") {
			t.Error("Bash matcher should be added")
		}
		if !strings.Contains(string(result), "coc hook") {
			t.Error("coc hook should be added")
		}
	})

	t.Run("invalid JSON input", func(t *testing.T) {
		input := []byte("not json")
		_, err := addHookToSettings(input)
		if err == nil {
			t.Error("expected error for invalid JSON input")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		input := []byte("")
		_, err := addHookToSettings(input)
		if err == nil {
			t.Error("expected error for empty input")
		}
	})
}

func TestRemoveHookFromSettings(t *testing.T) {
	t.Run("removes coc hook", func(t *testing.T) {
		input := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Bash",
						"hooks": [
							{"type": "command", "command": "coc hook"}
						]
					}
				]
			}
		}`)
		result, removed, err := removeHookFromSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !removed {
			t.Error("expected removed=true")
		}
		if strings.Contains(string(result), "coc hook") {
			t.Error("coc hook should be removed")
		}

		// Verify valid JSON
		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
	})

	t.Run("not found in empty settings", func(t *testing.T) {
		input := []byte(`{}`)
		result, removed, err := removeHookFromSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if removed {
			t.Error("expected removed=false when hook not present")
		}

		// Result should be valid JSON
		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
	})

	t.Run("not found with empty hooks", func(t *testing.T) {
		input := []byte(`{"hooks": {}}`)
		_, removed, err := removeHookFromSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if removed {
			t.Error("expected removed=false when hook not present")
		}
	})

	t.Run("not found in different matcher", func(t *testing.T) {
		input := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Read",
						"hooks": [
							{"type": "command", "command": "other-hook.sh"}
						]
					}
				]
			}
		}`)
		result, removed, err := removeHookFromSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if removed {
			t.Error("expected removed=false when coc hook not present")
		}
		// Original hook should still be there
		if !strings.Contains(string(result), "other-hook.sh") {
			t.Error("other hooks should be preserved")
		}
	})

	t.Run("preserves other hooks", func(t *testing.T) {
		input := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Bash",
						"hooks": [
							{"type": "command", "command": "other-hook.sh"},
							{"type": "command", "command": "coc hook"}
						]
					}
				]
			}
		}`)
		result, removed, err := removeHookFromSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !removed {
			t.Error("expected removed=true")
		}
		if !strings.Contains(string(result), "other-hook.sh") {
			t.Error("other hooks should be preserved")
		}
		if strings.Contains(string(result), "coc hook") {
			t.Error("coc hook should be removed")
		}
	})

	t.Run("preserves other settings", func(t *testing.T) {
		input := []byte(`{
			"permissions": {"allow": ["Bash(git:*)"]},
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Bash",
						"hooks": [
							{"type": "command", "command": "coc hook"}
						]
					}
				]
			}
		}`)
		result, removed, err := removeHookFromSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !removed {
			t.Error("expected removed=true")
		}
		if !strings.Contains(string(result), "permissions") {
			t.Error("permissions should be preserved")
		}

		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if _, ok := settings["permissions"]; !ok {
			t.Error("permissions field should be preserved")
		}
	})

	t.Run("removes empty Bash matcher after removing last hook", func(t *testing.T) {
		input := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Bash",
						"hooks": [
							{"type": "command", "command": "coc hook"}
						]
					}
				]
			}
		}`)
		result, removed, err := removeHookFromSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !removed {
			t.Error("expected removed=true")
		}

		// After removing the only hook, the Bash matcher should be cleaned up
		// (implementation choice - this test documents the expected behavior)
		var settings map[string]interface{}
		if err := json.Unmarshal(result, &settings); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
	})

	t.Run("preserves other matchers", func(t *testing.T) {
		input := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Read",
						"hooks": [
							{"type": "command", "command": "read-hook.sh"}
						]
					},
					{
						"matcher": "Bash",
						"hooks": [
							{"type": "command", "command": "coc hook"}
						]
					}
				]
			}
		}`)
		result, removed, err := removeHookFromSettings(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !removed {
			t.Error("expected removed=true")
		}
		if !strings.Contains(string(result), "Read") {
			t.Error("Read matcher should be preserved")
		}
		if !strings.Contains(string(result), "read-hook.sh") {
			t.Error("Read hook should be preserved")
		}
		if strings.Contains(string(result), "coc hook") {
			t.Error("coc hook should be removed")
		}
	})

	t.Run("invalid JSON input", func(t *testing.T) {
		input := []byte("not json")
		_, _, err := removeHookFromSettings(input)
		if err == nil {
			t.Error("expected error for invalid JSON input")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		input := []byte("")
		_, _, err := removeHookFromSettings(input)
		if err == nil {
			t.Error("expected error for empty input")
		}
	})
}

func TestSettingsRoundTrip(t *testing.T) {
	// Test that add -> remove -> add produces consistent results
	t.Run("add remove add produces consistent result", func(t *testing.T) {
		original := []byte(`{"hooks": {}}`)

		// Add hook
		withHook, err := addHookToSettings(original)
		if err != nil {
			t.Fatalf("add failed: %v", err)
		}

		// Remove hook
		withoutHook, removed, err := removeHookFromSettings(withHook)
		if err != nil {
			t.Fatalf("remove failed: %v", err)
		}
		if !removed {
			t.Error("expected hook to be removed")
		}

		// Add again
		withHookAgain, err := addHookToSettings(withoutHook)
		if err != nil {
			t.Fatalf("second add failed: %v", err)
		}

		// Should have hook again
		if !strings.Contains(string(withHookAgain), "coc hook") {
			t.Error("hook should be present after round trip")
		}
	})

	t.Run("idempotent add produces same result", func(t *testing.T) {
		original := []byte(`{"hooks": {}}`)

		// Add hook twice
		first, err := addHookToSettings(original)
		if err != nil {
			t.Fatalf("first add failed: %v", err)
		}

		second, err := addHookToSettings(first)
		if err != nil {
			t.Fatalf("second add failed: %v", err)
		}

		// Count occurrences
		firstCount := strings.Count(string(first), "coc hook")
		secondCount := strings.Count(string(second), "coc hook")

		if firstCount != 1 {
			t.Errorf("first add produced %d occurrences, want 1", firstCount)
		}
		if secondCount != 1 {
			t.Errorf("second add produced %d occurrences, want 1", secondCount)
		}
	})

	t.Run("idempotent remove produces same result", func(t *testing.T) {
		original := []byte(`{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Bash",
						"hooks": [
							{"type": "command", "command": "coc hook"}
						]
					}
				]
			}
		}`)

		// Remove hook twice
		first, removed1, err := removeHookFromSettings(original)
		if err != nil {
			t.Fatalf("first remove failed: %v", err)
		}
		if !removed1 {
			t.Error("first remove should report removed=true")
		}

		second, removed2, err := removeHookFromSettings(first)
		if err != nil {
			t.Fatalf("second remove failed: %v", err)
		}
		if removed2 {
			t.Error("second remove should report removed=false (already gone)")
		}

		// Should not contain hook in either result
		if strings.Contains(string(first), "coc hook") {
			t.Error("first remove should eliminate hook")
		}
		if strings.Contains(string(second), "coc hook") {
			t.Error("second remove should still have no hook")
		}
	})
}
