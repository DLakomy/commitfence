package cli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var update = flag.Bool("update", false, "rewrite the help golden in testdata/")

// TestCommandLine captures the public command tree so changes to commands,
// flags, and help text are reviewed together in one golden.
func TestCommandLine(t *testing.T) {
	var all bytes.Buffer
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		args := append(strings.Fields(command.CommandPath())[1:], "--help")
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("%v: status %d, stderr %q; want 0 and empty stderr", args, code, stderr.String())
		}
		fmt.Fprintf(&all, "=== %s\n%s", command.CommandPath(), stdout.String())
		// Shell-specific help belongs to Cobra; completion help already lists
		// the supported shells.
		if command.Name() == "completion" {
			return
		}
		for _, child := range command.Commands() {
			if !child.Hidden {
				visit(child)
			}
		}
	}
	var code int
	visit(newRoot(io.Discard, io.Discard, &code))
	golden := filepath.Join("testdata", "commandline.golden")
	if *update {
		if err := os.WriteFile(golden, all.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v; regenerate with go test ./internal/cli -update", err)
	}
	if !bytes.Equal(all.Bytes(), want) {
		t.Errorf("help differs from %s\ngot:\n%s\nwant:\n%s", golden, all.Bytes(), want)
	}
}
