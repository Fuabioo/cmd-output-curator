package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// cocSupportedCommands lists base commands that coc has filters for.
// This list must be kept in sync with filter.DefaultRegistry() capabilities.
var cocSupportedCommands = []string{
	"git", "go", "cargo", "docker", "grep", "rg", "npm", "pip", "pip3", "yarn",
}

// hookInput represents the JSON structure Claude Code sends to PreToolUse hooks.
type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// hookOutput represents the JSON structure we return to Claude Code.
type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName      string `json:"hookEventName"`
		PermissionDecision string `json:"permissionDecision"`
		UpdatedInput       struct {
			Command string `json:"command"`
		} `json:"updatedInput"`
	} `json:"hookSpecificOutput"`
}

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Claude Code PreToolUse hook handler",
	Long:  "Reads Claude Code hook input from stdin and rewrites supported commands to use coc.",
	RunE:  runHook,
}

// runHook implements the Claude Code PreToolUse hook contract.
//
// DESIGN: This function silently returns nil on ALL errors. This is intentional.
// Claude Code hooks that exit non-zero or produce unexpected output break ALL
// subsequent tool invocations in the session. The hook must be invisible when
// it cannot help — a broken hook is worse than no hook.
//
// To debug hook behavior, run manually:
//
//	echo '{"tool_name":"Bash","tool_input":{"command":"git status"}}' | coc hook
func runHook(_ *cobra.Command, _ []string) error {
	// Read hook input from stdin
	inputBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		// Hook contract: exit 0 even on errors to prevent hook failures
		return nil
	}

	// Parse hook input
	var input hookInput
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		// Invalid JSON, exit silently
		return nil
	}

	// Only handle Bash tool
	if input.ToolName != "Bash" {
		return nil
	}

	command := strings.TrimSpace(input.ToolInput.Command)
	if command == "" {
		return nil
	}

	// Don't wrap shell pipelines or chains — coc can't handle them
	if containsShellOps(command) {
		return nil
	}

	// Extract first word
	firstWord := extractFirstWord(command)
	if firstWord == "" {
		return nil
	}

	// Don't double-wrap if already coc-prefixed
	if firstWord == "coc" {
		return nil
	}

	// Check if it's a supported command
	if !isSupportedCommand(firstWord) {
		return nil
	}

	// Rewrite the command
	var output hookOutput
	output.HookSpecificOutput.HookEventName = "PreToolUse"
	output.HookSpecificOutput.PermissionDecision = "allow"
	output.HookSpecificOutput.UpdatedInput.Command = "coc " + command

	// Write the rewrite JSON to stdout
	outputBytes, err := json.Marshal(output)
	if err != nil {
		// Marshaling should never fail, but exit silently if it does
		return nil
	}

	if _, err := os.Stdout.Write(outputBytes); err != nil {
		// Can't write output, exit silently
		return nil
	}

	return nil
}

// containsShellOps checks if the command contains shell operators that would
// prevent coc from wrapping it (pipes, chains, subshells, etc.).
//
// NOTE: This uses naive string matching and may produce false positives for
// operators inside quoted strings (e.g., git log --grep="|pattern"). This is
// acceptable for the Claude Code use case where the Bash tool rarely sends
// quoted shell operators in flag values. A false positive simply means the
// command runs unwrapped (no filtering), which is safe.
func containsShellOps(cmd string) bool {
	return strings.Contains(cmd, "|") ||
		strings.Contains(cmd, "&&") ||
		strings.Contains(cmd, "||") ||
		strings.Contains(cmd, ";") ||
		strings.Contains(cmd, "$(") ||
		strings.Contains(cmd, "`")
}

// extractFirstWord returns the first whitespace-separated word from the command.
func extractFirstWord(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// isSupportedCommand checks if the command is in the list of coc-supported commands.
func isSupportedCommand(cmd string) bool {
	for _, supported := range cocSupportedCommands {
		if cmd == supported {
			return true
		}
	}
	return false
}
