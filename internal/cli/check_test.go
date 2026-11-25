package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const committingSource = "CREATE OR REPLACE PROCEDURE app.run IS BEGIN COMMIT; END run;\n/\n"

func writeCLIFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func checkCLI(t *testing.T, want int, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run(append([]string{"check"}, args...), &stdout, &stderr); code != want || stdout.Len() != 0 {
		t.Fatalf("check %v: status %d, stdout %q, stderr %q; want status %d and empty stdout", args, code, stdout.String(), stderr.String(), want)
	}
	return stderr.String()
}

func TestConfigDiscoveryAndDefaultPaths(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, ".commitfence.json"), "invalid parent config")
	child := filepath.Join(root, "child")
	writeCLIFile(t, filepath.Join(child, "local.sql"), committingSource)
	writeCLIFile(t, filepath.Join(child, "selected", "inner.sql"), committingSource)
	t.Chdir(child)

	// A parent config must not prevent the current-directory fallback scan.
	output := checkCLI(t, 1)
	if !strings.Contains(output, "local.sql:") || !strings.Contains(output, "selected/inner.sql:") || strings.Contains(output, "\x1b[") {
		t.Fatalf("default recursive scan: %s", output)
	}
	writeCLIFile(t, ".commitfence.json", `{"version":0,"paths":["selected"]}`)
	output = checkCLI(t, 1)
	if !strings.Contains(output, "selected/inner.sql:") || strings.Contains(output, "local.sql:") {
		t.Fatalf("discovered config paths: %s", output)
	}

	// Explicit config replaces discovery. With paths omitted, its directory must
	// not become the fallback scan root.
	writeCLIFile(t, ".commitfence.json", "invalid local config")
	writeCLIFile(t, "config/settings.json", `{"version":0}`)
	output = checkCLI(t, 1, "--config", "config/settings.json")
	if !strings.Contains(output, "local.sql:") || !strings.Contains(output, "selected/inner.sql:") {
		t.Fatalf("explicit config without paths: %s", output)
	}
}

func TestConfigAndCLIOverrides(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCLIFile(t, "project/src/default.sql", committingSource)
	writeCLIFile(t, "project/src/custom.unit", committingSource)
	writeCLIFile(t, "override.txt", committingSource)
	writeCLIFile(t, "project/settings.json", `{
  "version":0, "paths":["src"], "extensions":[".UNIT","unit"],
  "path_style":"absolute", "color":"always"
}`)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	output := checkCLI(t, 1, "--config", "project/settings.json")
	if !strings.Contains(output, filepath.Join(cwd, "project/src/custom.unit")+":") || !strings.Contains(output, "\x1b[33mwarning CF0001\x1b[0m") || strings.Contains(output, "default.sql:") {
		t.Fatalf("configured selection and presentation: %s", output)
	}
	output = checkCLI(t, 1, "--config", "project/settings.json", "--extensions=sql", "--path-style=relative", "--color=never")
	if !strings.Contains(output, "project/src/default.sql:") || strings.Contains(output, "custom.unit:") || strings.Contains(output, cwd) || strings.Contains(output, "\x1b[") {
		t.Fatalf("CLI setting overrides: %s", output)
	}
	output = checkCLI(t, 1, "override.txt", "--config", "project/settings.json", "--color=never", "--path-style=relative")
	if !strings.Contains(output, "override.txt:") || strings.Contains(output, "project/src/") {
		t.Fatalf("positional replacement and explicit-file bypass: %s", output)
	}
}

func TestCheckRelativePathsFromSymlinkedWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	writeCLIFile(t, filepath.Join(project, "src", "source.sql"), committingSource)
	writeCLIFile(t, filepath.Join(project, ".commitfence.json"), `{"version":0,"paths":["src"]}`)
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(project, alias); err != nil {
		t.Fatal(err)
	}
	t.Chdir(alias)

	missing := filepath.Join("missing", "externals.json")
	output := checkCLI(t, 2, "--externals", filepath.Join(alias, missing))
	if !strings.HasPrefix(output, filepath.Join("src", "source.sql")+":") ||
		!strings.Contains(output, "\n"+missing+":1:1: error CF1003") {
		t.Fatalf("expected project-relative source and externals diagnostics: %s", output)
	}
	output = checkCLI(t, 2, "--config", filepath.Join(alias, "missing", "settings.json"))
	if !strings.HasPrefix(output, filepath.Join("missing", "settings.json")+": error CF1003") {
		t.Fatalf("expected project-relative settings diagnostic: %s", output)
	}
}

func TestConfiguredExternals(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCLIFile(t, "project/source.sql", `CREATE OR REPLACE PROCEDURE app.run IS
  value NUMBER;
BEGIN
  value := ext.api.lookup(42);
END run;
/
`)
	writeCLIFile(t, "project/lookup-behavior.json", `{"version":0,"safe":["ext.api.lookup"]}`)
	writeCLIFile(t, "project/settings.json", `{"version":0,"paths":["source.sql"],"externals":"lookup-behavior.json"}`)
	if output := checkCLI(t, 0, "--config", "project/settings.json"); output != "" {
		t.Fatalf("config-relative externals file: %s", output)
	}
	if output := checkCLI(t, 1, "--config", "project/settings.json", "--externals="); !strings.Contains(output, "unknown-effect") {
		t.Fatalf("empty override did not disable declarations: %s", output)
	}
	writeCLIFile(t, "override.json", `{"version":0,"unsafe":["ext.api.lookup"]}`)
	output := checkCLI(t, 1, "--config", "project/settings.json", "--externals", "override.json")
	if !strings.Contains(output, "missing-permission") || !strings.Contains(output, "origin: override.json:") {
		t.Fatalf("working-directory-relative externals override: %s", output)
	}
}

func TestInvalidConfigurationStopsAnalysis(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCLIFile(t, "source.sql", committingSource)
	for _, tc := range []struct{ name, document string }{
		{"malformed", "{"},
		{"missing version", `{}`},
		{"null version", `{"version":null}`},
		{"unsupported version", `{"version":2}`},
		{"unknown setting", `{"version":0,"other":true}`},
		{"removed warning policy", `{"version":0,"fail_on_warning":false}`},
		{"wrong type", `{"version":0,"paths":"source.sql"}`},
		{"empty paths", `{"version":0,"paths":[]}`},
		{"empty source path", `{"version":0,"paths":[""]}`},
		{"empty extensions", `{"version":0,"extensions":[]}`},
		{"invalid extension", `{"version":0,"extensions":["*.sql"]}`},
		{"path style", `{"version":0,"path_style":"other"}`},
		{"color", `{"version":0,"color":"other"}`},
		{"null field", `{"version":0,"paths":null}`},
		{"duplicate setting", `{"version":0,"color":"auto","color":"never"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeCLIFile(t, ".commitfence.json", tc.document)
			output := checkCLI(t, 2)
			if !strings.HasPrefix(output, ".commitfence.json: error CF1003 [configuration]:") || strings.Contains(output, "CF0001") {
				t.Fatalf("invalid config must stop before analysis: %s", output)
			}
		})
	}
	output := checkCLI(t, 2, "--path-style=absolute", "--color=always")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output, filepath.Join(cwd, ".commitfence.json")+": \x1b[31merror CF1003\x1b[0m") {
		t.Fatalf("config error presentation overrides: %s", output)
	}
	for _, args := range [][]string{{"--help"}, {"completion", "bash"}} {
		if args[0] == "--help" {
			args = append([]string{"check"}, args...)
		}
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.Len() == 0 || strings.Contains(stdout.String(), "\x1b[") {
			t.Fatalf("%v with invalid config: status %d, stdout %q, stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestCheckErrorsAndExitStatus(t *testing.T) {
	t.Chdir(t.TempDir())
	writeCLIFile(t, "warning.sql", committingSource)
	writeCLIFile(t, "clean.sql", "-- no transaction effects\n")
	if output := checkCLI(t, 0, "clean.sql"); output != "" {
		t.Fatalf("clean output: %s", output)
	}
	checkCLI(t, 1, "warning.sql")
	output := checkCLI(t, 2, "warning.sql", "missing.sql", "--externals", "missing.json")
	if !strings.Contains(output, "CF0001") || !strings.Contains(output, "missing.sql: error CF1002") || !strings.Contains(output, "missing.json:1:1: error CF1003") {
		t.Fatalf("errors must take precedence while preserving independent findings: %s", output)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, cwd) {
		t.Fatalf("filesystem messages repeated absolute paths: %s", output)
	}
	output = checkCLI(t, 2, "warning.sql", "")
	if !strings.Contains(output, "CF0001") || !strings.Contains(output, "empty path") {
		t.Fatalf("an empty CLI path must not stop independent inputs: %s", output)
	}
	for _, args := range [][]string{
		{"--config", "missing.json"}, {"--config", "."}, {"--config="},
		{"--color=other"}, {"--path-style=other"}, {"--extensions="},
		{"--fail-on-warning"}, {""},
	} {
		checkCLI(t, 2, args...)
	}
}
