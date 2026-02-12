package cli

import (
	"fmt"
	"os"
)

// printError prints a formatted error message to stderr.
func printError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "coc: "+format+"\n", args...)
}
