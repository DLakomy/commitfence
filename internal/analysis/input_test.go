package analysis_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"commitfence/internal/analysis"
)

func TestCheckDirectoryExtensions(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.sql", "b.pks", "c.PKB", "d.pls", "e.TPS", "f.tpb", "g.unit", "h.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("-- source\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name       string
		extensions []string
		want       []string
	}{
		{"defaults", nil, []string{"a.sql", "b.pks", "c.PKB", "d.pls", "e.TPS", "f.tpb"}},
		{"replacement and normalization", []string{".UNIT", "unit", "SQL", ".sql"}, []string{"a.sql", "g.unit"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := analysis.Check(analysis.Options{Paths: []string{root}, Extensions: tc.extensions})
			var names []string
			for _, file := range report.Files {
				if !filepath.IsAbs(file) {
					t.Errorf("report lost absolute path: %s", file)
				}
				names = append(names, filepath.Base(file))
			}
			if !reflect.DeepEqual(names, tc.want) || len(report.Errors) != 0 || len(report.Findings) != 0 {
				t.Fatalf("got %v, report %+v; want %v", names, report, tc.want)
			}
		})
	}
	report := analysis.Check(analysis.Options{Paths: []string{root}, Extensions: []string{}})
	if len(report.Errors) != 1 || len(report.Files) != 0 {
		t.Fatalf("explicit empty extensions must not fall back to defaults: %+v", report)
	}
}
