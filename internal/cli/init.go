package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Install coc hook into Claude Code",
	Long:  "Installs a PreToolUse hook in Claude Code settings that transparently wraps supported commands with coc.",
	RunE:  runInit,
}

var uninstallFlag bool

func init() {
	initCmd.Flags().BoolVar(&uninstallFlag, "uninstall", false, "Remove the coc hook from Claude Code settings")
}

func runInit(_ *cobra.Command, _ []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to find home directory: %w", err)
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

	if uninstallFlag {
		return uninstallHook(settingsPath)
	}

	return installHook(settingsPath)
}

func installHook(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read settings.json: %w", err)
	}
	if os.IsNotExist(err) {
		data = []byte("{}")
	}

	result, err := addHookToSettings(data)
	if err != nil {
		return err
	}

	// Detect whether the hook was already present by normalizing the input
	// and comparing with the output. Both go through json.MarshalIndent so
	// the comparison is on canonical form.
	normalized, err := normalizeJSON(data)
	if err == nil && bytes.Equal(normalized, result) {
		fmt.Println("coc hook already installed in ~/.claude/settings.json")
		return nil
	}

	if err := writeSettings(settingsPath, result); err != nil {
		return err
	}

	fmt.Println("coc hook installed in ~/.claude/settings.json")
	return nil
}

func uninstallHook(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("coc hook not found, nothing to remove")
			return nil
		}
		return fmt.Errorf("failed to read settings.json: %w", err)
	}

	result, removed, err := removeHookFromSettings(data)
	if err != nil {
		return err
	}

	if !removed {
		fmt.Println("coc hook not found, nothing to remove")
		return nil
	}

	if err := writeSettings(settingsPath, result); err != nil {
		return err
	}

	fmt.Println("coc hook removed from ~/.claude/settings.json")
	return nil
}

// normalizeJSON re-serializes JSON through MarshalIndent to produce a
// canonical form that can be compared byte-for-byte with addHookToSettings output.
func normalizeJSON(data []byte) ([]byte, error) {
	var v map[string]interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ")
}

// writeSettings atomically writes data to path using a temp-file + rename.
func writeSettings(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Add trailing newline for consistency
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp settings: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to update settings: %w", err)
	}
	return nil
}

// addHookToSettings takes JSON bytes, adds the coc hook if not present, and returns updated JSON.
// This is a pure function for testing purposes.
func addHookToSettings(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(input, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Navigate to hooks.PreToolUse array
	hooksMap, hooksExists := settings["hooks"].(map[string]interface{})
	if !hooksExists {
		hooksMap = make(map[string]interface{})
		settings["hooks"] = hooksMap
	}

	preToolUse, preExists := hooksMap["PreToolUse"].([]interface{})
	if !preExists {
		preToolUse = []interface{}{}
	}

	// Check if coc hook already exists
	for _, hook := range preToolUse {
		hookMap, ok := hook.(map[string]interface{})
		if !ok {
			continue
		}
		if matcher, _ := hookMap["matcher"].(string); matcher == "Bash" {
			if hooksArr, ok := hookMap["hooks"].([]interface{}); ok {
				for _, h := range hooksArr {
					hMap, ok := h.(map[string]interface{})
					if !ok {
						continue
					}
					if hType, _ := hMap["type"].(string); hType == "command" {
						if cmd, _ := hMap["command"].(string); cmd == "coc hook" {
							// Already exists, return as-is
							return json.MarshalIndent(settings, "", "  ")
						}
					}
				}
			}
		}
	}

	// Add the coc hook
	cocHook := map[string]interface{}{
		"matcher": "Bash",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": "coc hook",
			},
		},
	}

	preToolUse = append(preToolUse, cocHook)
	hooksMap["PreToolUse"] = preToolUse

	return json.MarshalIndent(settings, "", "  ")
}

// removeHookFromSettings takes JSON bytes, removes the coc hook if present, and returns updated JSON.
// Returns (result, wasRemoved, error). This is a pure function for testing purposes.
func removeHookFromSettings(input []byte) ([]byte, bool, error) {
	if len(input) == 0 {
		return nil, false, fmt.Errorf("empty input")
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(input, &settings); err != nil {
		return nil, false, fmt.Errorf("failed to parse JSON: %w", err)
	}

	hooksMap, hooksExists := settings["hooks"].(map[string]interface{})
	if !hooksExists {
		result, err := json.MarshalIndent(settings, "", "  ")
		return result, false, err
	}

	preToolUse, preExists := hooksMap["PreToolUse"].([]interface{})
	if !preExists {
		result, err := json.MarshalIndent(settings, "", "  ")
		return result, false, err
	}

	// Find and remove coc hook entry
	var newPreToolUse []interface{}
	found := false

	for _, hook := range preToolUse {
		hookMap, ok := hook.(map[string]interface{})
		if !ok {
			newPreToolUse = append(newPreToolUse, hook)
			continue
		}

		if matcher, _ := hookMap["matcher"].(string); matcher == "Bash" {
			if hooksArr, ok := hookMap["hooks"].([]interface{}); ok {
				// Filter out "coc hook" from the hooks array
				var newHooksArr []interface{}
				for _, h := range hooksArr {
					hMap, ok := h.(map[string]interface{})
					if !ok {
						newHooksArr = append(newHooksArr, h)
						continue
					}
					if hType, _ := hMap["type"].(string); hType == "command" {
						if cmd, _ := hMap["command"].(string); cmd == "coc hook" {
							// Found coc hook, skip it
							found = true
							continue
						}
					}
					newHooksArr = append(newHooksArr, h)
				}

				// If there are remaining hooks, keep the matcher with updated hooks
				if len(newHooksArr) > 0 {
					updatedHook := make(map[string]interface{})
					for k, v := range hookMap {
						updatedHook[k] = v
					}
					updatedHook["hooks"] = newHooksArr
					newPreToolUse = append(newPreToolUse, updatedHook)
				}
				// If no hooks remain, don't add the matcher entry (it's removed)
				continue
			}
		}

		newPreToolUse = append(newPreToolUse, hook)
	}

	hooksMap["PreToolUse"] = newPreToolUse
	result, err := json.MarshalIndent(settings, "", "  ")
	return result, found, err
}
