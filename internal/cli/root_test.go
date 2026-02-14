package cli

import (
	"testing"
)

// TestRootFind_ArbitraryCommands verifies that arbitrary commands like
// "git status" are accepted by the root command and not rejected as
// "unknown command" by Cobra's legacyArgs validator.
//
// This is a regression test for the bug where cobra.Command.Find calls
// legacyArgs when Args==nil, which rejects any positional args on a
// root command that has subcommands.
func TestRootFind_ArbitraryCommands(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string // expected resolved command name
	}{
		{
			name:     "git status resolves to root",
			args:     []string{"git", "status"},
			wantName: "coc",
		},
		{
			name:     "go test resolves to root",
			args:     []string{"go", "test", "./..."},
			wantName: "coc",
		},
		{
			name:     "hook subcommand still resolves",
			args:     []string{"hook"},
			wantName: "hook",
		},
		{
			name:     "init subcommand still resolves",
			args:     []string{"init"},
			wantName: "init",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd()
			found, _, err := cmd.Find(tt.args)
			if err != nil {
				t.Fatalf("Find(%v) returned error: %v", tt.args, err)
			}
			if found.Name() != tt.wantName {
				t.Errorf("Find(%v) resolved to %q, want %q", tt.args, found.Name(), tt.wantName)
			}
		})
	}
}
