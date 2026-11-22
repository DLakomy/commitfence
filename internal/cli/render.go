package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/term"

	"commitfence/internal/analysis"
)

type renderer struct {
	out       io.Writer
	cwd       string
	pathStyle string
	color     bool
}

func newRenderer(out io.Writer, cwd, pathStyle, mode string) renderer {
	if pathStyle != "absolute" {
		if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
			cwd = resolved
		}
	}
	terminal := false
	if file, ok := out.(interface{ Fd() uintptr }); ok {
		terminal = term.IsTerminal(int(file.Fd()))
	}
	return renderer{out: out, cwd: cwd, pathStyle: pathStyle,
		color: useColor(mode, terminal, os.Getenv("NO_COLOR"), os.Getenv("TERM"))}
}

func useColor(mode string, terminal bool, noColor, terminalType string) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	default:
		return terminal && noColor == "" && terminalType != "dumb"
	}
}

func (r renderer) location(loc analysis.Location) string {
	if loc.File == "" {
		return "commitfence"
	}
	file := loc.File
	if r.pathStyle != "absolute" {
		if relative, err := filepath.Rel(r.cwd, resolveDiagnosticPath(file)); err == nil {
			file = relative
		}
	}
	if loc.Line == 0 {
		return file
	}
	return fmt.Sprintf("%s:%d:%d", file, loc.Line, loc.Column)
}

// Resolve the existing directory prefix so even a missing file can be
// displayed relative to the resolved working directory.
func resolveDiagnosticPath(path string) string {
	dir, suffix := filepath.Dir(path), filepath.Base(path)
	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, suffix)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path
		}
		suffix = filepath.Join(filepath.Base(dir), suffix)
		dir = parent
	}
}

func (r renderer) render(d analysis.Diagnostic) {
	out := r.out
	label := fmt.Sprintf("%s %s", d.Severity, d.ID)
	if r.color {
		if d.Severity == analysis.SeverityError {
			label = "\x1b[31m" + label + "\x1b[0m"
		} else if d.Severity == analysis.SeverityWarning {
			label = "\x1b[33m" + label + "\x1b[0m"
		}
	}
	fmt.Fprintf(out, "%s: %s [%s]: %s", r.location(d.Location), label, d.Kind, d.Message)
	if d.Routine != "" {
		fmt.Fprintf(out, " (routine %s)", d.Routine)
	}
	if d.Callee != "" {
		fmt.Fprintf(out, " (callee %s)", d.Callee)
	}
	if d.UnresolvedText != "" {
		fmt.Fprintf(out, " (unresolved %q)", d.UnresolvedText)
	}
	fmt.Fprintln(out)
	if d.Origin.File != "" {
		fmt.Fprintf(out, "  origin: %s\n", r.location(d.Origin))
	}
	for _, step := range d.Evidence {
		fmt.Fprintf(out, "  trace: %s: %s", r.location(step.Location), step.Routine)
		if step.Callee != "" {
			fmt.Fprintf(out, " -> %s", step.Callee)
		}
		fmt.Fprintf(out, ": %s\n", step.Description)
	}
}
