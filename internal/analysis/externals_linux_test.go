package analysis_test

import (
	"path/filepath"
	"syscall"
	"testing"

	"commitfence/internal/analysis"
)

func TestCheckNonregularExternalsFile(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "externals.fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, fifo} {
		got := analysis.Check(analysis.Options{Paths: []string{"testdata/externals/source.sql"}, ExternalsPath: path})
		if len(got.Errors) != 1 || got.Errors[0].ID != "CF1003" || got.Errors[0].Kind != analysis.KindConfiguration || got.Errors[0].Location.File != path || got.Errors[0].Message != "externals file must be a regular file" || len(got.Findings) == 0 {
			t.Fatalf("nonregular externals file %s: %+v", path, got)
		}
	}
}
