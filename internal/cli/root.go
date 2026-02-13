package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Fuabioo/coc/internal/executor"
	"github.com/Fuabioo/coc/internal/filter"
)

// Version and Commit are set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
)

// exitError carries an exit code through Cobra's error handling.
type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:                "coc [flags] <command> [args...]",
		Short:              "CMD Output Curator -- curate CLI output for AI agents",
		Long:               "coc proxies CLI commands, tees output to log files, and filters stdout for reduced token consumption by AI agents.",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,
		RunE:               runRoot,
	}

	// Add subcommands (these have normal flag parsing)
	root.AddCommand(hookCmd)
	root.AddCommand(initCmd)

	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		printError("%v", err)
		return 1
	}
	return 0
}

// runRoot handles the main proxy logic.
func runRoot(cmd *cobra.Command, _ []string) error {
	// Local flag state — not package-level, so tests can call runRoot safely
	var (
		flagVerbose  int
		flagLogDir   string
		flagNoFilter bool
		flagNoLog    bool
	)

	args := os.Args[1:]
	var proxiedArgs []string

	i := 0
	for i < len(args) {
		switch {
		case args[i] == "-v" || args[i] == "--verbose":
			flagVerbose++
			i++
		case args[i] == "-vv":
			flagVerbose += 2
			i++
		case args[i] == "-vvv":
			flagVerbose += 3
			i++
		case strings.HasPrefix(args[i], "--log-dir="):
			flagLogDir = strings.TrimPrefix(args[i], "--log-dir=")
			i++
		case args[i] == "--log-dir" && i+1 < len(args):
			flagLogDir = args[i+1]
			i += 2
		case args[i] == "--no-filter":
			flagNoFilter = true
			i++
		case args[i] == "--no-log":
			flagNoLog = true
			flagNoFilter = true
			i++
		case args[i] == "-h" || args[i] == "--help":
			return cmd.Help()
		case args[i] == "--version":
			fmt.Printf("coc %s (%s)\n", Version, Commit)
			return nil
		default:
			proxiedArgs = args[i:]
			i = len(args)
		}
	}

	if len(proxiedArgs) == 0 {
		return cmd.Help()
	}

	cfg := executor.Config{
		Command:  proxiedArgs[0],
		Args:     proxiedArgs[1:],
		LogDir:   flagLogDir,
		NoFilter: flagNoFilter,
		NoLog:    flagNoLog,
		Verbose:  flagVerbose > 0,
		Registry: filter.DefaultRegistry(),
	}

	result := executor.Run(cfg)
	if result.ExitCode != 0 {
		return &exitError{code: result.ExitCode}
	}
	return nil
}
