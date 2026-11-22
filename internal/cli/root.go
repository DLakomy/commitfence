// Package cli parses the command line, renders analysis reports, and returns
// an exit status without exiting the process.
package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Run executes args without the program name, writing help and completion to
// stdout and diagnostics to stderr. It returns 0 for a clean check, 1 for
// warnings, or 2 for invocation or analysis errors. Errors take precedence.
func Run(args []string, stdout, stderr io.Writer) int {
	var code int
	root := newRoot(stdout, stderr, &code)
	// A non-nil slice prevents Cobra from falling back to os.Args for nil input.
	root.SetArgs(append([]string{}, args...))
	if err := root.Execute(); err != nil {
		fmt.Fprintf(stderr, "commitfence: %v\nRun 'commitfence --help' for usage.\n", err)
		return 2
	}
	return code
}

func newRoot(stdout, stderr io.Writer, code *int) *cobra.Command {
	root := &cobra.Command{
		Use:           "commitfence",
		Short:         "Check local PL/SQL transaction behavior",
		SilenceErrors: true,
		SilenceUsage:  true,
		// A runnable root rejects missing commands, including --help=false,
		// instead of Cobra's default of printing help and succeeding.
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("expected check subcommand")
			}
			return fmt.Errorf("unknown command %q for commitfence", args[0])
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newCheckCommand(stderr, code))
	// Materialize built-ins so command-tree discovery sees the same public
	// interface that Execute exposes.
	root.InitDefaultHelpFlag()
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	return root
}
