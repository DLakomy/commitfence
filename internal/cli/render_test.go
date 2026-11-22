package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"commitfence/internal/analysis"
)

func TestDiagnosticPresentation(t *testing.T) {
	base := t.TempDir()
	cwd := filepath.Join(base, "work")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(cwd, "source.sql")
	externalsPath := filepath.Join(base, "externals.json")
	diagnostic := analysis.Diagnostic{
		ID: "CF0001", Severity: analysis.SeverityWarning, Kind: analysis.KindMissingPermission,
		Location: analysis.Location{File: source, Line: 3, Column: 2}, Message: "transaction effect",
		Routine: "APP.RUN", Callee: "EXT.API.RUN",
		Origin: analysis.Location{File: externalsPath, Line: 2, Column: 1},
		Evidence: []analysis.Evidence{
			{Location: analysis.Location{File: source, Line: 3, Column: 2}, Routine: "APP.RUN", Callee: "EXT.API.RUN", Description: "call"},
			{Location: analysis.Location{File: externalsPath, Line: 2, Column: 1}, Routine: "EXT.API.RUN", Description: "external unsafe declaration"},
		},
	}
	var plain, styled, absolute bytes.Buffer
	newRenderer(&plain, cwd, "relative", "never").render(diagnostic)
	newRenderer(&styled, cwd, "relative", "always").render(diagnostic)
	newRenderer(&absolute, cwd, "absolute", "never").render(diagnostic)
	outside := filepath.Join("..", "externals.json")
	for _, text := range []string{
		"source.sql:3:2: warning CF0001", "  origin: " + outside + ":2:1",
		"  trace: source.sql:3:2: APP.RUN -> EXT.API.RUN: call",
		"  trace: " + outside + ":2:1: EXT.API.RUN: external unsafe declaration",
	} {
		if !strings.Contains(plain.String(), text) {
			t.Errorf("missing %q in %s", text, plain.String())
		}
	}
	if strings.Contains(plain.String(), base) || !strings.Contains(absolute.String(), source+":3:2") || !strings.Contains(absolute.String(), "origin: "+externalsPath+":2:1") || !strings.Contains(absolute.String(), "trace: "+externalsPath+":2:1") {
		t.Fatalf("path styles: relative %q, absolute %q", plain.String(), absolute.String())
	}
	strip := strings.NewReplacer("\x1b[33m", "", "\x1b[31m", "", "\x1b[0m", "")
	if !strings.Contains(styled.String(), "\x1b[33mwarning CF0001\x1b[0m") || strip.Replace(styled.String()) != plain.String() {
		t.Fatalf("color changed diagnostic text: plain %q, styled %q", plain.String(), styled.String())
	}
	if got := (renderer{cwd: "relative"}).location(diagnostic.Location); got != source+":3:2" {
		t.Fatalf("failed relative conversion should retain absolute path: %s", got)
	}
}

func TestColorPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, mode, noColor, term string
		terminal, want            bool
	}{
		{name: "terminal", mode: "auto", terminal: true, want: true},
		{name: "redirected", mode: "auto"},
		{name: "NO_COLOR", mode: "auto", terminal: true, noColor: "1"},
		{name: "dumb terminal", mode: "auto", terminal: true, term: "dumb"},
		{name: "force despite environment", mode: "always", noColor: "1", term: "dumb", want: true},
		{name: "disabled terminal", mode: "never", terminal: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := useColor(tc.mode, tc.terminal, tc.noColor, tc.term); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
	// An *os.File writer may be a redirected file, not a terminal. Exercise that
	// production wiring without depending on a terminal being available to tests.
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if newRenderer(file, "", "relative", "auto").color {
		t.Fatal("auto enabled color for redirected file stderr")
	}
}
