package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndToEndProject(t *testing.T) {
	fixture, err := filepath.Abs("../../e2e")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                     string
		args                     []string
		annotateEntry            bool
		status                   int
		missing, unknown, errors int
		contains                 []string
	}{
		{
			name: "config discovery", status: 1, missing: 2,
			contains: []string{
				"src/import_api.pkb:4:5: warning CF0001 [missing-permission]",
				"src/import_api.pkb:5:5: warning CF0001 [missing-permission]",
				"(routine APP.IMPORT_API.RUN_IMPORT) (callee APP.CUSTOMER_SERVICE.SAVE_CUSTOMER)",
				"(routine APP.IMPORT_API.RUN_IMPORT) (callee EXT.CUSTOMER_API.FLUSH_BATCH)",
				"origin: src/customer_service.pkb:6:5",
				"trace: src/import_api.pkb:4:5: APP.IMPORT_API.RUN_IMPORT -> APP.CUSTOMER_SERVICE.SAVE_CUSTOMER: call",
				"trace: src/customer_service.pkb:6:5: APP.CUSTOMER_SERVICE.SAVE_CUSTOMER: COMMIT",
				"origin: externals.json:8:5",
				"trace: src/import_api.pkb:5:5: APP.IMPORT_API.RUN_IMPORT -> EXT.CUSTOMER_API.FLUSH_BATCH: call",
				"trace: externals.json:8:5: EXT.CUSTOMER_API.FLUSH_BATCH: external unsafe declaration",
			},
		},
		{
			name: "entry specification grants permission", annotateEntry: true, status: 0,
		},
		{
			name: "external declarations disabled", args: []string{"--externals="},
			status: 1, missing: 1, unknown: 3,
			contains: []string{
				"src/customer_service.pkb:5:20: warning CF0002 [unknown-effect]",
				"src/import_api.pkb:4:5: warning CF0002 [unknown-effect]",
				"src/import_api.pkb:5:5: warning CF0002 [unknown-effect]",
				"APP.CUSTOMER_SERVICE.SAVE_CUSTOMER -> EXT.CUSTOMER_API.FIND_CUSTOMER: unresolved call",
				"APP.IMPORT_API.RUN_IMPORT -> EXT.CUSTOMER_API.FLUSH_BATCH: unresolved call",
			},
		},
		{
			name: "missing externals file preserves source findings", args: []string{"--externals=missing.json"},
			status: 2, missing: 1, unknown: 3, errors: 1,
			contains: []string{
				"missing.json:1:1: error CF1003 [configuration]: inspect externals file:",
				"trace: src/customer_service.pkb:6:5: APP.CUSTOMER_SERVICE.SAVE_CUSTOMER: COMMIT",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := t.TempDir()
			if err := os.CopyFS(project, os.DirFS(fixture)); err != nil {
				t.Fatal(err)
			}
			t.Chdir(project)
			if tc.annotateEntry {
				path := filepath.Join("src", "import_api.pks")
				spec, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				annotated := strings.Replace(string(spec), "  PROCEDURE run_import;", "  --@commitfence\n  PROCEDURE run_import;", 1)
				writeCLIFile(t, path, annotated)
			}

			output := checkCLI(t, tc.status, tc.args...)
			if strings.Count(output, "warning CF0001 [missing-permission]") != tc.missing ||
				strings.Count(output, "warning CF0002 [unknown-effect]") != tc.unknown ||
				strings.Count(output, ": warning ") != tc.missing+tc.unknown ||
				strings.Count(output, ": error ") != tc.errors {
				t.Fatalf("unexpected diagnostics: %s", output)
			}
			if tc.status == 0 && output != "" {
				t.Fatalf("clean project produced output: %s", output)
			}
			if strings.Contains(output, project) || strings.Contains(output, "\x1b[") {
				t.Fatalf("expected plain diagnostics with relative paths: %s", output)
			}
			for _, fragment := range tc.contains {
				if !strings.Contains(output, fragment) {
					t.Errorf("missing %q in diagnostics:\n%s", fragment, output)
				}
			}
		})
	}
}
