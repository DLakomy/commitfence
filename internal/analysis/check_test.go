package analysis_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/DLakomy/commitfence/internal/analysis"
)

// These report combinations drive the CLI's clean, warning, and error statuses.
// Keep verification in process against the same real-file acceptance fixtures.
func TestCheckReportSeverities(t *testing.T) {
	for _, tc := range []struct {
		name             string
		options          analysis.Options
		warnings, errors bool
	}{
		{"clean", analysis.Options{Paths: []string{"testdata/sources"}}, false, false},
		{"warnings only", analysis.Options{Paths: []string{"testdata/graph"}}, true, false},
		{"errors only", analysis.Options{Paths: []string{"testdata/sources/missing.sql"}}, false, true},
		{"warnings and input errors", analysis.Options{Paths: []string{"testdata/graph", "testdata/sources/missing.sql"}}, true, true},
		{"warnings and declaration errors", analysis.Options{Paths: []string{"testdata/externals/source.sql"}, ExternalsPath: "testdata/externals/valid.json"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := analysis.Check(tc.options)
			if (len(report.Findings) > 0) != tc.warnings || (len(report.Errors) > 0) != tc.errors {
				t.Fatalf("unexpected severity combination: %+v", report)
			}
			// Compare complete mixed reports, including errors and evidence,
			// when paths are reversed and repeated.
			reordered := tc.options
			reordered.Paths = nil
			for _, path := range slices.Backward(tc.options.Paths) {
				reordered.Paths = append(reordered.Paths, path)
			}
			reordered.Paths = append(reordered.Paths, tc.options.Paths...)
			if !reflect.DeepEqual(report, analysis.Check(reordered)) {
				t.Fatal("input order or duplicates changed report")
			}
			for _, d := range report.Findings {
				if d.Severity != analysis.SeverityWarning {
					t.Fatalf("unexpected finding severity: %+v", d)
				}
			}
			for _, e := range report.Errors {
				if e.Severity != analysis.SeverityError {
					t.Fatalf("unexpected error severity: %+v", e)
				}
			}
		})
	}
}

func TestCheckInputs(t *testing.T) {
	root, err := filepath.Abs("testdata/sources")
	if err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(root, "a.sql")
	b := filepath.Join(root, "nested", "b.PKB")
	missing := filepath.Join(root, "missing.sql")
	for _, tc := range []struct {
		name   string
		paths  []string
		files  []string
		errors int
	}{
		{"no inputs", nil, []string{}, 1},
		{"empty path", []string{""}, []string{}, 1},
		{"readable file", []string{a}, []string{a}, 0},
		{"recursive directory", []string{root}, []string{a, b}, 0},
		{"overlapping paths", []string{b, root, a, root}, []string{a, b}, 0},
		{"explicit extension", []string{filepath.Join(root, "ignored.txt")}, []string{filepath.Join(root, "ignored.txt")}, 0},
		{"invalid with valid", []string{missing, a, missing}, []string{a}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := analysis.Check(analysis.Options{Paths: tc.paths})
			if !reflect.DeepEqual(got.Files, tc.files) || len(got.Errors) != tc.errors || len(got.Findings) != 0 {
				t.Fatalf("unexpected report: %+v", got)
			}
			for _, e := range got.Errors {
				if e.ID == "" || e.Severity != analysis.SeverityError || e.Kind != analysis.KindInput || e.Message == "" {
					t.Fatalf("incomplete error: %+v", e)
				}
			}
		})
	}
	first := analysis.Check(analysis.Options{Paths: []string{missing, b, root, filepath.Join(root, "another-missing")}})
	second := analysis.Check(analysis.Options{Paths: []string{filepath.Join(root, "another-missing"), root, b, missing}})
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("argument order changed report: %+v versus %+v", first, second)
	}
	if first.Errors[0].Location.File != filepath.Join(root, "another-missing") || first.Errors[0].Location.Line != 0 {
		t.Fatalf("unexpected location: %+v", first.Errors[0])
	}
}

func TestCheckFilesystemAliasesAndInvalidFile(t *testing.T) {
	root := canonicalTempDir(t)
	original := filepath.Join(root, "a.sql")
	if err := os.WriteFile(original, []byte("-- readable\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(original, filepath.Join(root, "b.sql")); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(original, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(root, "broken.sql")); err != nil {
		t.Fatal(err)
	}
	got := analysis.Check(analysis.Options{Paths: []string{root, link}})
	if !reflect.DeepEqual(got.Files, []string{original}) || len(got.Errors) != 0 {
		t.Fatalf("unexpected aliases: %+v", got)
	}
	// A process-independent invalid-file case: a device is never a regular source file.
	got = analysis.Check(analysis.Options{Paths: []string{os.DevNull, original}})
	if len(got.Errors) != 1 || got.Errors[0].ID != "CF1002" || !reflect.DeepEqual(got.Files, []string{original}) {
		t.Fatalf("unexpected invalid file report: %+v", got)
	}
	empty := analysis.Check(analysis.Options{Paths: []string{t.TempDir()}})
	if len(empty.Errors) != 0 || len(empty.Files) != 0 {
		t.Fatalf("unexpected empty directory report: %+v", empty)
	}
}

func TestCheckSymlinkParent(t *testing.T) {
	root := canonicalTempDir(t)
	target := filepath.Join(root, "sources")
	if err := os.MkdirAll(filepath.Join(target, "subdir"), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(target, "a.sql")
	if err := os.WriteFile(source, nil, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(target, "subdir"), link); err != nil {
		t.Fatal(err)
	}
	// Do not use filepath.Join: it would remove the very component under test.
	got := analysis.Check(analysis.Options{Paths: []string{link + "/../a.sql"}})
	if len(got.Errors) != 0 || !reflect.DeepEqual(got.Files, []string{source}) {
		t.Fatalf("unexpected report: %+v", got)
	}
}

func TestCheckUnreadableSource(t *testing.T) {
	root := canonicalTempDir(t)
	bad := filepath.Join(root, "a.sql")
	good := filepath.Join(root, "z.sql")
	if err := os.WriteFile(bad, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bad, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0600) })
	if _, err := os.ReadFile(bad); err == nil {
		t.Skip("process privileges bypass file read permissions")
	}
	got := analysis.Check(analysis.Options{Paths: []string{root}})
	if !reflect.DeepEqual(got.Files, []string{good}) || len(got.Errors) != 1 {
		t.Fatalf("unexpected report: %+v", got)
	}
	e := got.Errors[0]
	if e.ID != "CF1002" || e.Kind != analysis.KindInput || e.Severity != analysis.SeverityError || e.Location.File != bad || !strings.HasPrefix(e.Message, "read source:") {
		t.Fatalf("unexpected read error: %+v", e)
	}
}

func TestCheckMissingExternalsFile(t *testing.T) {
	source, err := filepath.Abs("testdata/sources/a.sql")
	if err != nil {
		t.Fatal(err)
	}
	externalsPath := filepath.Join(t.TempDir(), "externals.json")
	got := analysis.Check(analysis.Options{Paths: []string{source}, ExternalsPath: externalsPath})
	if !reflect.DeepEqual(got.Files, []string{source}) || len(got.Errors) != 1 {
		t.Fatalf("unexpected report: %+v", got)
	}
	e := got.Errors[0]
	if e.ID != "CF1003" || e.Severity != analysis.SeverityError || e.Kind != analysis.KindConfiguration || e.Location.File != externalsPath {
		t.Fatalf("unexpected configuration error: %+v", e)
	}
}

func TestCheckSourceLimits(t *testing.T) {
	for _, aggregate := range []bool{false, true} {
		t.Run(fmt.Sprint("aggregate=", aggregate), func(t *testing.T) {
			root := canonicalTempDir(t)
			count := 1
			size := analysis.MaxSourceBytes + 1
			if aggregate {
				count = int(analysis.MaxTotalSourceBytes/analysis.MaxSourceBytes) + 1
				size = analysis.MaxSourceBytes
			}
			for i := 0; i < count; i++ {
				f, err := os.Create(filepath.Join(root, fmt.Sprintf("%02d.sql", i)))
				if err != nil {
					t.Fatal(err)
				}
				err = f.Truncate(size)
				closeErr := f.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			}
			good := filepath.Join(root, "z.sql")
			if err := os.WriteFile(good, nil, 0600); err != nil {
				t.Fatal(err)
			}
			got := analysis.Check(analysis.Options{Paths: []string{root}})
			if len(got.Errors) != 1 || got.Errors[0].ID != "CF1002" || !strings.Contains(got.Errors[0].Message, "source byte limit exceeded") || got.Files[len(got.Files)-1] != good {
				t.Fatalf("unexpected limit report: %+v", got)
			}
		})
	}
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckExtraction(t *testing.T) {
	for _, tc := range []struct {
		file           string
		routines       []string
		effects        []string
		errorKinds     []analysis.Kind
		positions      [][2]int
		errorPositions [][2]int
	}{
		{"effects.sql", []string{"APP.DIRECT", "APP.DIRECT", "APP.DIRECT", "APP.DIRECT", "APP.PROSE", "APP.LITERAL_MARKER", "APP.BLOCK_MARKER"}, []string{"COMMIT", "ROLLBACK", "ROLLBACK", "ROLLBACK", "COMMIT", "ROLLBACK", "COMMIT"}, nil, [][2]int{{3, 3}, {4, 17}, {4, 32}, {9, 5}, {31, 3}, {37, 3}, {45, 3}}, nil},
		{"groups.sql", []string{"APP.API.PERMITTED.HELPER", "APP.API.UNTOUCHED", "APP.OUTER_R", "OTHER.PERMITTED", `"app"."case"`}, []string{"COMMIT", "ROLLBACK", "COMMIT", "COMMIT", "COMMIT"}, nil, [][2]int{{13, 7}, {24, 5}, {34, 5}, {42, 3}, {53, 3}}, nil},
		{"boundaries.sql", []string{"APP.UNNAMED", "APP.RECOVERED", "APP.UNATTACHED"}, []string{"COMMIT", "ROLLBACK", "COMMIT"}, []analysis.Kind{analysis.KindInvalidAnnotation, analysis.KindIncompleteAnalysis, analysis.KindIncompleteAnalysis, analysis.KindIncompleteAnalysis, analysis.KindInvalidAnnotation, analysis.KindInvalidAnnotation}, [][2]int{{8, 3}, {18, 3}, {33, 3}}, [][2]int{{1, 1}, {11, 1}, {21, 1}, {22, 3}, {29, 1}, {36, 1}}},
		{"isolation.sql", []string{"APP.OUTER_R.LOGGER", "APP.OUTER_R.LOGGER.ORDINARY_HELPER"}, []string{"COMMIT", "COMMIT"}, nil, [][2]int{{6, 7}, {6, 7}}, nil},
	} {
		t.Run(tc.file, func(t *testing.T) {
			path, err := filepath.Abs(filepath.Join("testdata/extraction", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			got := analysis.Check(analysis.Options{Paths: []string{path}})
			if len(got.Findings) != len(tc.routines) || len(got.Errors) != len(tc.errorKinds) {
				t.Fatalf("unexpected report: %+v", got)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for i, d := range got.Findings {
				if d.Location.Line != tc.positions[i][0] || d.Location.Column != tc.positions[i][1] {
					t.Fatalf("finding %d location: %+v, want %v", i, d.Location, tc.positions[i])
				}
				if d.ID != "CF0001" || d.Kind != analysis.KindMissingPermission || d.Severity != analysis.SeverityWarning || d.Routine != tc.routines[i] || d.Location != d.Origin || len(d.Evidence) != 1 || d.Evidence[0].Routine != d.Routine || d.Evidence[0].Location != d.Location || d.Evidence[0].Description != tc.effects[i] {
					t.Fatalf("finding %d: %+v", i, d)
				}
				assertSourceLocation(t, content, path, d.Location, tc.effects[i])
			}
			for i, e := range got.Errors {
				if e.Location.Line != tc.errorPositions[i][0] || e.Location.Column != tc.errorPositions[i][1] {
					t.Fatalf("error %d location: %+v, want %v", i, e.Location, tc.errorPositions[i])
				}
				if e.Kind != tc.errorKinds[i] || e.Severity != analysis.SeverityError || e.Message == "" {
					t.Fatalf("error %d: %+v", i, e)
				}
				prefix := "CREATE"
				if e.Kind == analysis.KindInvalidAnnotation {
					prefix = "--@commitfence"
					if e.ID != "CF1005" {
						t.Fatal(e)
					}
				} else {
					prefix = "PROCEDURE"
					if e.ID != "CF1004" {
						t.Fatal(e)
					}
					if strings.HasPrefix(string(content[e.Location.Offset:]), "CREATE") {
						prefix = "CREATE"
					}
				}
				assertSourceLocation(t, content, path, e.Location, prefix)
			}
		})
	}
}

func assertSourceLocation(t *testing.T, content []byte, path string, loc analysis.Location, prefix string) {
	t.Helper()
	if loc.File != path || loc.Offset < 0 || loc.Offset >= len(content) || !strings.HasPrefix(string(content[loc.Offset:]), prefix) {
		t.Fatalf("bad location: %+v, want %q", loc, prefix)
	}
	before := string(content[:loc.Offset])
	line := strings.Count(before, "\n") + 1
	column := loc.Offset - strings.LastIndex(before, "\n")
	if loc.Line != line || loc.Column != column {
		t.Fatalf("bad line/column: %+v, want %d:%d", loc, line, column)
	}
}

func TestCheckUnnamedEndings(t *testing.T) {
	path, _ := filepath.Abs("testdata/extraction/unnamed.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	report := analysis.Check(analysis.Options{Paths: []string{path}})
	// Exact sites also protect clean siblings, overload permission, and the
	// autonomous logger's enclosing routine and caller from leaked effects.
	want := []struct {
		line, column, origin, steps int
		routine, callee, text       string
	}{
		{10, 40, 10, 1, "APP.UNNAMED_API.LEAF", "", "COMMIT"},
		{11, 44, 10, 2, "APP.UNNAMED_API.MIDDLE", "APP.UNNAMED_API.LEAF", "leaf"},
		{12, 29, 10, 3, "APP.UNNAMED_API.DRIVER", "APP.UNNAMED_API.MIDDLE", "middle"},
		{18, 3, 10, 4, "APP.UNNAMED_TOP", "APP.UNNAMED_API.DRIVER", "app.unnamed_api.driver"},
		{25, 31, 25, 1, "APP.UNNAMED_HOST.LOGGER", "", "COMMIT"},
		{25, 31, 25, 1, "APP.UNNAMED_HOST.LOGGER.HELPER", "", "COMMIT"},
		{27, 5, 25, 2, "APP.UNNAMED_HOST.LOGGER", "APP.UNNAMED_HOST.LOGGER.HELPER", "helper"},
		{28, 5, 28, 1, "APP.UNNAMED_HOST.LOGGER", "EXECUTE IMMEDIATE", "EXECUTE IMMEDIATE"},
		{45, 5, 45, 1, "APP.UNNAMED_BLOCKS", "", "ROLLBACK"},
		{47, 3, 47, 1, "APP.UNNAMED_BLOCKS", "", "COMMIT"},
		{50, 31, 50, 1, "APP.UNNAMED_BLOCKS", "", "ROLLBACK"},
		{50, 31, 50, 1, "APP.UNNAMED_BLOCKS.LOCAL_HELPER", "", "ROLLBACK"},
		{50, 31, 50, 1, "APP.UNNAMED_BLOCKS.LOCAL_HELPER.LEAF", "", "ROLLBACK"},
		{62, 5, 50, 2, "APP.UNNAMED_BLOCKS", "APP.UNNAMED_BLOCKS.LOCAL_HELPER", "local_helper"},
		{66, 3, 66, 1, "APP.UNNAMED_BLOCKS", "", "ROLLBACK"},
	}
	if len(report.Errors) != 0 || len(report.Findings) != len(want) {
		t.Fatalf("unexpected report: %+v", report)
	}
	for i, site := range want {
		d := report.Findings[i]
		kind, id := analysis.KindMissingPermission, "CF0001"
		if site.callee == "EXECUTE IMMEDIATE" {
			kind, id = analysis.KindUnknownEffect, "CF0002"
		}
		if d.ID != id || d.Kind != kind || d.Severity != analysis.SeverityWarning || d.Routine != site.routine || d.Callee != site.callee || d.Location.Line != site.line || d.Location.Column != site.column || d.Origin.Line != site.origin || len(d.Evidence) != site.steps {
			t.Fatalf("finding %d: %+v, want %+v", i, d, site)
		}
		if d.Evidence[0].Location != d.Location || d.Evidence[0].Routine != d.Routine || d.Evidence[len(d.Evidence)-1].Location != d.Origin {
			t.Fatalf("finding %d lost evidence locations: %+v", i, d)
		}
		assertSourceLocation(t, content, path, d.Location, site.text)
		last := d.Evidence[len(d.Evidence)-1]
		prefix := last.Description
		if kind == analysis.KindUnknownEffect {
			prefix = "EXECUTE IMMEDIATE"
		}
		assertSourceLocation(t, content, path, d.Origin, prefix)
	}
	chain := report.Findings[3].Evidence
	for i, name := range []string{"APP.UNNAMED_TOP", "APP.UNNAMED_API.DRIVER", "APP.UNNAMED_API.MIDDLE", "APP.UNNAMED_API.LEAF"} {
		if chain[i].Routine != name || i+1 < len(chain) && chain[i].Callee != chain[i+1].Routine {
			t.Fatalf("incorrect transitive evidence: %+v", chain)
		}
	}
}

func TestCheckUnnamedMalformedRecovery(t *testing.T) {
	a := "testdata/extraction/unnamed_malformed.sql"
	b := "testdata/extraction/unnamed.sql"
	report := analysis.Check(analysis.Options{Paths: []string{a}})
	want := []struct {
		line, column int
		routine      string
	}{
		{1, 1, "APP.MISSING_END"},
		{5, 1, "APP.WRONG_END"},
		{6, 1, "APP.WRONG_CONTROL"},
		{11, 1, "APP.AMBIGUOUS_OWNER"},
		{12, 3, "APP.AMBIGUOUS_OWNER.AMBIGUOUS_OWNER"},
		{16, 1, "APP.MISSING_BEFORE_CREATE"},
		{19, 1, "APP.NO_BEGIN"},
		{23, 1, ""},
		{24, 1, "APP.WRONG_CASE"},
		{25, 1, "APP.MISSING_AT_EOF"},
	}
	if len(report.Errors) != len(want) || len(report.Findings) != 3 {
		t.Fatalf("unexpected recovery report: %+v", report)
	}
	path, _ := filepath.Abs(a)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, site := range want {
		e := report.Errors[i]
		if e.ID != "CF1004" || e.Kind != analysis.KindIncompleteAnalysis || e.Severity != analysis.SeverityError || e.Routine != site.routine || e.Location.Line != site.line || e.Location.Column != site.column {
			t.Fatalf("error %d: %+v, want %+v", i, e, site)
		}
		prefix := "CREATE"
		if site.column == 3 {
			prefix = "PROCEDURE"
		} else if site.routine == "" {
			prefix = "END"
		}
		assertSourceLocation(t, content, path, e.Location, prefix)
	}
	for i, name := range []string{"APP.AFTER_SLASH", "APP.AFTER_CREATE", "APP.GOOD_LAST"} {
		if d := report.Findings[i]; d.Kind != analysis.KindMissingPermission || d.Routine != name || d.Location.Line != []int{3, 17, 21}[i] {
			t.Fatalf("incorrect recovered finding: %+v", d)
		}
	}
	first := analysis.Check(analysis.Options{Paths: []string{a, b}})
	second := analysis.Check(analysis.Options{Paths: []string{b, a, b}})
	if !reflect.DeepEqual(first, second) || len(first.Errors) != len(want) || len(first.Findings) != 18 {
		t.Fatalf("independent input/order changed recovery: %+v", first)
	}
}

func TestCheckExtractionOrderAndIndependentInputs(t *testing.T) {
	a := "testdata/extraction/boundaries.sql"
	b := "testdata/extraction/effects.sql"
	c := "testdata/extraction/groups.sql"
	first := analysis.Check(analysis.Options{Paths: []string{a, b, c}})
	second := analysis.Check(analysis.Options{Paths: []string{c, b, a, b}})
	if !reflect.DeepEqual(first, second) || len(first.Findings) != 15 || len(first.Errors) != 6 {
		t.Fatalf("unexpected combined results: %+v versus %+v", first, second)
	}
}

func TestCheckQuotedIdentityAndBytePositions(t *testing.T) {
	for _, tc := range []struct {
		file, routine string
		line, column  int
	}{
		{"quoted.sql", `"Owner"."pkg"."END fake; PROCEDURE bogus"`, 9, 5},
		{"positions.sql", "APP.WINDOWS", 4, 15},
	} {
		t.Run(tc.file, func(t *testing.T) {
			path, _ := filepath.Abs(filepath.Join("testdata/extraction", tc.file))
			report := analysis.Check(analysis.Options{Paths: []string{path}})
			if len(report.Errors) != 0 || len(report.Findings) != 1 {
				t.Fatalf("unexpected report: %+v", report)
			}
			d := report.Findings[0]
			if d.Routine != tc.routine || d.Location.Line != tc.line || d.Location.Column != tc.column {
				t.Fatalf("unexpected finding: %+v", d)
			}
		})
	}
}

func TestCheckMalformedHeaderAndExtraBoundary(t *testing.T) {
	report := analysis.Check(analysis.Options{Paths: []string{"testdata/extraction/ambiguous.sql"}})
	if len(report.Errors) != 2 || len(report.Findings) != 1 || report.Findings[0].Routine != "APP.GOOD" {
		t.Fatalf("unexpected report: %+v", report)
	}
	for i, e := range report.Errors {
		if e.ID != "CF1004" || e.Kind != analysis.KindIncompleteAnalysis || e.Location.Line != []int{1, 6}[i] {
			t.Fatalf("unexpected error: %+v", e)
		}
	}
}

func TestCheckLexicalPermissionScope(t *testing.T) {
	report := analysis.Check(analysis.Options{Paths: []string{"testdata/extraction/scopes.sql"}})
	if len(report.Errors) != 0 || len(report.Findings) != 2 || report.Findings[0].Routine != "APP.WORKER" || report.Findings[1].Routine != "APP.WORKER.HELPER" {
		t.Fatalf("unrelated declaration granted permission: %+v", report)
	}
}

func TestCheckStatementContextsAndLabeledBlocks(t *testing.T) {
	report := analysis.Check(analysis.Options{Paths: []string{"testdata/extraction/statements.sql"}})
	var effects []analysis.Diagnostic
	var unknowns []analysis.Diagnostic
	for _, d := range report.Findings {
		if d.Kind == analysis.KindUnknownEffect {
			unknowns = append(unknowns, d)
		} else {
			effects = append(effects, d)
		}
	}
	if len(unknowns) != 4 {
		t.Fatalf("expected four unresolved identifier calls: %+v", unknowns)
	}
	for i, d := range unknowns {
		if d.ID != "CF0002" || d.Location.Line != 6+i || d.UnresolvedText == "" {
			t.Fatalf("unexpected unknown call: %+v", d)
		}
	}
	report.Findings = effects
	want := [][2]int{{3, 20}, {4, 22}, {5, 8}, {13, 3}, {16, 27}, {18, 3}}
	if len(report.Findings) != len(want) || len(report.Errors) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	for i, d := range report.Findings {
		if d.ID != "CF0001" || d.Routine != "APP.WORKER" || d.Location.Line != want[i][0] || d.Location.Column != want[i][1] {
			t.Fatalf("finding %d: %+v, want position %v", i, d, want[i])
		}
	}
	e := report.Errors[0]
	if e.ID != "CF1004" || e.Routine != "APP.BAD" || e.Location.Line != 21 || e.Location.Column != 1 {
		t.Fatalf("unexpected boundary error: %+v", e)
	}
}

func TestCheckPermissionAcrossFiles(t *testing.T) {
	spec := "testdata/extraction/split.pks"
	body := "testdata/extraction/split.pkb"
	first := analysis.Check(analysis.Options{Paths: []string{spec, body}})
	second := analysis.Check(analysis.Options{Paths: []string{body, spec}})
	if !reflect.DeepEqual(first, second) || len(first.Errors) != 0 || len(first.Findings) != 1 {
		t.Fatalf("unexpected split reports: %+v versus %+v", first, second)
	}
	d := first.Findings[0]
	path, _ := filepath.Abs(body)
	if d.ID != "CF0001" || d.Routine != "APP.SPLIT.OTHER_MEMBER" || d.Location.File != path || d.Location.Line != 8 || d.Location.Column != 5 {
		t.Fatalf("unexpected finding: %+v", d)
	}
	alone := analysis.Check(analysis.Options{Paths: []string{body}})
	if len(alone.Errors) != 0 || len(alone.Findings) != 2 || alone.Findings[0].Routine != "APP.SPLIT.PERMITTED" {
		t.Fatalf("body without permission: %+v", alone)
	}
}

func TestCheckAutonomousDescendantIntervals(t *testing.T) {
	report := analysis.Check(analysis.Options{Paths: []string{"testdata/extraction/isolation_intervals.sql"}})
	want := []struct {
		routine      string
		line, column int
	}{
		{"APP.HOST.FIRST_AUTO", 11, 7},
		{"APP.HOST.FIRST_AUTO.ORDINARY", 11, 7},
		{"APP.HOST", 22, 3},
	}
	if len(report.Errors) != 0 || len(report.Findings) != len(want) {
		t.Fatalf("unexpected report: %+v", report)
	}
	for i, d := range report.Findings {
		if d.Routine != want[i].routine || d.Location.Line != want[i].line || d.Location.Column != want[i].column {
			t.Fatalf("finding %d: %+v, want %+v", i, d, want[i])
		}
	}
}

func TestCheckGraphBehaviors(t *testing.T) {
	// Exact sites also assert absence of findings in clean recursion, isolated
	// callers, permitted overloads, and safe built-ins. Both diamond branches
	// warn, but their caller receives one finding with a representative path.
	want := []struct {
		file            string
		line, column    int
		routine, callee string
		unknown         bool
	}{
		{"api.pkb", 7, 30, "APP.API.CYCLE_A", "APP.API.CYCLE_B", false},
		{"api.pkb", 8, 30, "APP.API.CYCLE_B", "APP.API.CYCLE_A", false},
		{"api.pkb", 8, 39, "APP.API.CYCLE_B", "", false},
		{"api.pkb", 9, 40, "APP.API.LEAF", "", false},
		{"api.pkb", 10, 32, "APP.API.LEFT_PATH", "APP.API.LEAF", false},
		{"api.pkb", 11, 33, "APP.API.RIGHT_PATH", "APP.API.LEAF", false},
		{"api.pkb", 12, 30, "APP.API.DIAMOND", "APP.API.LEFT_PATH", false},
		{"api.pkb", 12, 41, "APP.API.DIAMOND", "APP.API.RIGHT_PATH", false},
		{"api.pkb", 16, 10, "APP.API.EXPRESSIONS", "APP.API.OVERLOADED", false},
		{"api.pkb", 17, 12, "APP.API.EXPRESSIONS", "APP.API.LEAF", false},
		{"api.pkb", 17, 50, "APP.API.EXPRESSIONS", "APP.API.LEAF", false},
		{"api.pkb", 18, 40, "APP.API.EXPRESSIONS", "APP.API.LEAF", false},
		{"api.pkb", 19, 32, "APP.API.EXPRESSIONS", "APP.API.LEAF", false},
		{"api.pkb", 20, 39, "APP.API.EXPRESSIONS", "APP.API.LEAF", false},
		{"api.pkb", 21, 5, "APP.API.EXPRESSIONS", "APP.API.ADVERTISED", false},
		{"callers.sql", 1, 35, "APP.TOP", "APP.API.DIAMOND", false},
		{"callers.sql", 3, 44, "OTHER.ADVERTISED", "", false},
		{"callers.sql", 5, 41, "APP.UNRELATED", "ADVERTISED", true},
		{"callers.sql", 7, 47, "APP.UNKNOWN_WRAPPER", "APP.API.MISSING_BODY", true},
		{"callers.sql", 10, 49, "APP.PERMITTED_WRAPPER", "APP.UNKNOWN_WRAPPER", true},
		{"callers.sql", 12, 49, "APP.CAPABILITY_CALLER", "APP.API.ADVERTISED", false},
		{"callers.sql", 14, 46, "APP.UNKNOWN_CALLER", "APP.PERMITTED_WRAPPER", false},
		{"callers.sql", 14, 46, "APP.UNKNOWN_CALLER", "APP.PERMITTED_WRAPPER", true},
		{"scopes.sql", 6, 29, "APP.HOST", "APP.HOST.HELPER", false},
		{"scopes.sql", 6, 29, "APP.HOST.WORKER", "APP.HOST.HELPER", false},
		{"scopes.sql", 7, 29, "APP.HOST", "", false},
		{"scopes.sql", 7, 29, "APP.HOST.HELPER", "", false},
		{"scopes.sql", 9, 3, "APP.HOST", "APP.HOST.WORKER", false},
		{"scopes.sql", 16, 5, "APP.ISOLATED.LOGGER", "APP.HOST", false},
		{"scopes.sql", 17, 5, "APP.ISOLATED.LOGGER", "UNKNOWN_EXT", true},
		{"scopes.sql", 27, 47, "APP.MIXED", "", false},
		{"scopes.sql", 29, 44, "APP.MIXED_CALLER", "APP.MIXED", false},
		{"scopes.sql", 31, 54, `"app"."Count"`, "", false},
		{"scopes.sql", 33, 50, "APP.COUNT", "", false},
		{"scopes.sql", 38, 8, "APP.QUOTED_CALLS", `"app"."Count"`, false},
		{"scopes.sql", 39, 8, "APP.QUOTED_CALLS", `"app"."count"`, true},
		{"scopes.sql", 40, 8, "APP.QUOTED_CALLS", "APP.COUNT", false},
	}
	scenarios := []struct {
		name     string
		routines []string
	}{
		{"effectful recursion", []string{"APP.API.CYCLE_A", "APP.API.CYCLE_B"}},
		{"clean recursion", []string{"APP.API.CLEAN_A", "APP.API.CLEAN_B"}},
		{"transitive and repeated paths", []string{"APP.API.LEAF", "APP.API.LEFT_PATH", "APP.API.RIGHT_PATH", "APP.API.DIAMOND", "APP.TOP"}},
		{"specification and overload permission", []string{"APP.API.ADVERTISED", "APP.API.OVERLOADED", "APP.CAPABILITY_CALLER"}},
		{"expression and SQL calls", []string{"APP.API.EXPRESSIONS"}},
		{"unrelated names", []string{"OTHER.ADVERTISED", "APP.UNRELATED"}},
		{"unknown propagation through permission", []string{"APP.UNKNOWN_WRAPPER", "APP.PERMITTED_WRAPPER", "APP.UNKNOWN_CALLER"}},
		{"nested lexical resolution", []string{"APP.HOST", "APP.HOST.WORKER", "APP.HOST.HELPER"}},
		{"autonomous isolation", []string{"APP.ISOLATED", "APP.ISOLATED.LOGGER", "APP.ISOLATED_CALLER"}},
		{"mixed autonomous overloads", []string{"APP.MIXED", "APP.MIXED_CALLER"}},
		{"quoted identities and source before builtins", []string{`"app"."Count"`, "APP.COUNT", "APP.QUOTED_CALLS"}},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			report := analysis.Check(analysis.Options{Paths: []string{"testdata/graph"}})
			if len(report.Errors) != 0 {
				t.Fatalf("analysis errors: %+v", report.Errors)
			}
			selected := map[string]bool{}
			for _, r := range scenario.routines {
				selected[r] = true
			}
			got := report
			got.Findings = nil
			for _, d := range report.Findings {
				if selected[d.Routine] {
					got.Findings = append(got.Findings, d)
				}
			}
			expected := want
			want := expected[:0:0]
			for _, w := range expected {
				if selected[w.routine] {
					want = append(want, w)
				}
			}
			if len(got.Findings) != len(want) {
				t.Fatalf("got %d findings, want %d: %+v", len(got.Findings), len(want), got.Findings)
			}
			for i, w := range want {
				d := got.Findings[i]
				id, kind := "CF0001", analysis.KindMissingPermission
				if w.unknown {
					id, kind = "CF0002", analysis.KindUnknownEffect
				}
				if filepath.Base(d.Location.File) != w.file || d.Location.Line != w.line || d.Location.Column != w.column || d.Routine != w.routine || d.Callee != w.callee || d.ID != id || d.Kind != kind || d.Severity != analysis.SeverityWarning {
					t.Fatalf("finding %d: %+v, want %+v", i, d, w)
				}
				if len(d.Evidence) == 0 || d.Evidence[0].Location != d.Location || d.Evidence[0].Routine != d.Routine || d.Evidence[len(d.Evidence)-1].Location != d.Origin {
					t.Fatalf("invalid evidence: %+v", d)
				}
				for _, e := range d.Evidence {
					content, err := os.ReadFile(e.Location.File)
					if err != nil {
						t.Fatal(err)
					}
					assertSourceLocation(t, content, e.Location.File, e.Location, "")
				}
			}

		})
	}
}

func graphFinding(t *testing.T, routine string, line int, kind analysis.Kind) analysis.Diagnostic {
	t.Helper()
	report := analysis.Check(analysis.Options{Paths: []string{"testdata/graph"}})
	if len(report.Errors) != 0 {
		t.Fatalf("analysis errors: %+v", report.Errors)
	}
	for _, d := range report.Findings {
		if d.Routine == routine && d.Location.Line == line && d.Kind == kind {
			return d
		}
	}
	t.Fatalf("missing finding for %s:%d (%s)", routine, line, kind)
	return analysis.Diagnostic{}
}

func TestCheckGraphEvidence(t *testing.T) {
	t.Run("representative diamond path", func(t *testing.T) {
		top := graphFinding(t, "APP.TOP", 1, analysis.KindMissingPermission)
		var chain []string
		for _, e := range top.Evidence {
			chain = append(chain, e.Routine)
		}
		if !reflect.DeepEqual(chain, []string{"APP.TOP", "APP.API.DIAMOND", "APP.API.LEFT_PATH", "APP.API.LEAF"}) || filepath.Base(top.Origin.File) != "api.pkb" || top.Origin.Line != 9 {
			t.Fatalf("unexpected representative path: %+v", top)
		}
	})
	t.Run("specification marker origin", func(t *testing.T) {
		d := graphFinding(t, "APP.CAPABILITY_CALLER", 12, analysis.KindMissingPermission)
		if filepath.Base(d.Origin.File) != "api.pks" || d.Origin.Line != 2 || len(d.Evidence) != 2 || d.Evidence[1].Description != "transaction capability" {
			t.Fatalf("lost specification marker: %+v", d)
		}
	})
	t.Run("unknown chain", func(t *testing.T) {
		d := graphFinding(t, "APP.UNKNOWN_CALLER", 14, analysis.KindUnknownEffect)
		if len(d.Evidence) != 4 || filepath.Base(d.Origin.File) != "api.pks" || d.Origin.Line != 6 {
			t.Fatalf("unknown propagation: %+v", d)
		}
	})
	t.Run("unresolved spelling", func(t *testing.T) {
		a := graphFinding(t, "APP.UNRELATED", 5, analysis.KindUnknownEffect)
		b := graphFinding(t, "APP.QUOTED_CALLS", 39, analysis.KindUnknownEffect)
		if a.UnresolvedText != "advertised" || b.UnresolvedText != `"app"."count"` {
			t.Fatalf("lost unresolved text: %+v %+v", a, b)
		}
	})
}

func TestCheckGraphOrdering(t *testing.T) {
	got := analysis.Check(analysis.Options{Paths: []string{"testdata/graph"}})
	reordered := analysis.Check(analysis.Options{Paths: []string{"testdata/graph/scopes.sql", "testdata/graph/callers.sql", "testdata/graph/api.pks", "testdata/graph/api.pkb", "testdata/graph"}})
	if !reflect.DeepEqual(got, reordered) {
		t.Fatalf("input order changed report")
	}
}

func TestCheckResolutionWithoutSchemaDefaults(t *testing.T) {
	got := analysis.Check(analysis.Options{Paths: []string{"testdata/resolution"}})
	want := []struct {
		routine, callee string
		line            int
		kind            analysis.Kind
	}{
		{"CALLER", "ADVERTISED", 6, analysis.KindUnknownEffect},
		{"CALLER", "BARE.ADVERTISED", 6, analysis.KindMissingPermission},
		{"APP.CALLER", "ADVERTISED", 8, analysis.KindUnknownEffect},
		{"APP.CALLER", "BARE.ADVERTISED", 8, analysis.KindMissingPermission},
		{"APP.CALLER", "OTHER.ADVERTISED", 8, analysis.KindUnknownEffect},
		{"APP.OUTER_R", "", 11, analysis.KindMissingPermission},
		{"APP.OUTER_R.HELPER", "", 11, analysis.KindMissingPermission},
		{"APP.OUTER_R", "APP.OUTER_R.HELPER", 13, analysis.KindMissingPermission},
		{"APP.QUOTED", "EXT.UNKNOWN", 20, analysis.KindUnknownEffect},
	}
	if len(got.Errors) != 0 || len(got.Findings) != len(want) {
		t.Fatalf("unexpected report: %+v", got)
	}
	for i, w := range want {
		d := got.Findings[i]
		if d.Routine != w.routine || d.Callee != w.callee || d.Location.Line != w.line || d.Kind != w.kind {
			t.Fatalf("finding %d: %+v, want %+v", i, d, w)
		}
	}
	if got.Findings[1].Origin.Line != 2 || got.Findings[3].Origin.Line != 2 || got.Findings[8].UnresolvedText != "ext . unknown" {
		t.Fatalf("lost marker origin or original text: %+v", got.Findings)
	}
}

func TestCheckForwardDeclarationIsolation(t *testing.T) {
	got := analysis.Check(analysis.Options{Paths: []string{"testdata/forward_isolation"}})
	if len(got.Errors) != 0 || len(got.Findings) != 1 {
		t.Fatalf("unexpected isolation report: %+v", got)
	}
	d := got.Findings[0]
	if d.Kind != analysis.KindMissingPermission || d.Routine != "APP.MIXED_CALLER" || d.Callee != "APP.MIXED_PARENT" || d.Location.Line != 28 || d.Origin.Line != 16 {
		t.Fatalf("ordinary overload lost its capability: %+v", d)
	}
}

func TestCheckLongCallerBeforeCalleeChain(t *testing.T) {
	const count = 96
	var source strings.Builder
	for i := range count {
		fmt.Fprintf(&source, "CREATE PROCEDURE app.r%03d IS BEGIN ", i)
		if i+1 < count {
			fmt.Fprintf(&source, "app.r%03d; ", i+1)
		} else {
			source.WriteString("COMMIT; missing_ext(); ")
		}
		fmt.Fprintf(&source, "END r%03d;\n/\n", i)
	}
	path := filepath.Join(t.TempDir(), "chain.sql")
	if err := os.WriteFile(path, []byte(source.String()), 0600); err != nil {
		t.Fatal(err)
	}
	report := analysis.Check(analysis.Options{Paths: []string{path}})
	if len(report.Errors) != 0 || len(report.Findings) != 2*count {
		t.Fatalf("unexpected chain report: %d findings, errors %+v", len(report.Findings), report.Errors)
	}
	for _, kind := range []analysis.Kind{analysis.KindMissingPermission, analysis.KindUnknownEffect} {
		t.Run(string(kind), func(t *testing.T) {
			var found bool
			for _, d := range report.Findings {
				if d.Routine != "APP.R000" || d.Kind != kind {
					continue
				}
				found = true
				if len(d.Evidence) != count || d.Origin.Line != 2*count-1 {
					t.Fatalf("incomplete chain: %+v", d)
				}
				for i, e := range d.Evidence {
					if e.Routine != fmt.Sprintf("APP.R%03d", i) {
						t.Fatalf("step %d: %+v", i, e)
					}
				}
			}
			if !found {
				t.Fatal("missing first caller finding")
			}
		})
	}
}

func TestCheckUnknownBehavior(t *testing.T) {
	report := analysis.Check(analysis.Options{Paths: []string{"testdata/unknown"}})
	if len(report.Errors) != 0 {
		t.Fatalf("errors: %+v", report.Errors)
	}
	want := []struct {
		routine string
		line    int
		kind    analysis.Kind
		callee  string
	}{
		{"APP.NATIVE", 3, analysis.KindUnknownEffect, "EXECUTE IMMEDIATE"},
		{"APP.NATIVE", 4, analysis.KindUnknownEffect, "EXECUTE IMMEDIATE"},
		{"APP.NATIVE", 5, analysis.KindUnknownEffect, "EXECUTE IMMEDIATE"},
		{"APP.NATIVE", 6, analysis.KindUnknownEffect, "OPEN FOR"},
		{"APP.NATIVE", 7, analysis.KindUnknownEffect, "OPEN FOR"},
		{"APP.DBMS_DYNAMIC", 12, analysis.KindUnknownEffect, "DBMS_SQL.PARSE"},
		{"APP.DBMS_DYNAMIC", 13, analysis.KindUnknownEffect, "DBMS_SQL.EXECUTE"},
		{"APP.DBMS_DYNAMIC", 14, analysis.KindUnknownEffect, "DBMS_SQL.EXECUTE_AND_FETCH"},
		{"APP.MIXED", 22, analysis.KindMissingPermission, ""},
		{"APP.MIXED", 23, analysis.KindUnknownEffect, "APP.API.UNDESCRIBED"},
		{"APP.MIXED", 24, analysis.KindUnknownEffect, "EXT.MISSING"},
		{"APP.WRAPPER", 28, analysis.KindUnknownEffect, "APP.MIXED"},
		{"APP.TOP", 30, analysis.KindMissingPermission, "APP.WRAPPER"},
		{"APP.TOP", 30, analysis.KindUnknownEffect, "APP.WRAPPER"},
		{"APP.CYCLE_A", 32, analysis.KindMissingPermission, "APP.CYCLE_B"},
		{"APP.CYCLE_A", 32, analysis.KindUnknownEffect, "APP.CYCLE_B"},
		{"APP.CYCLE_B", 34, analysis.KindMissingPermission, "APP.CYCLE_A"},
		{"APP.CYCLE_B", 34, analysis.KindUnknownEffect, "APP.CYCLE_A"},
		{"APP.CYCLE_B", 34, analysis.KindMissingPermission, "APP.NATIVE"},
		{"APP.CYCLE_B", 34, analysis.KindUnknownEffect, "APP.NATIVE"},
		{"APP.BUILTINS", 37, analysis.KindUnknownEffect, "ABS"},
		{"APP.BUILTINS", 39, analysis.KindUnknownEffect, "DBMS_RANDOM.VALUE"},
		{"APP.BUILTINS", 40, analysis.KindUnknownEffect, "DBMS_OUTPUT.UNDESCRIBED"},
		{"ABS", 43, analysis.KindUnknownEffect, "MISSING_DEPENDENCY"},
		{"APP.ISOLATED", 47, analysis.KindUnknownEffect, "EXECUTE IMMEDIATE"},
		{"APP.NESTED", 52, analysis.KindUnknownEffect, "OPEN FOR"},
		{"APP.NESTED.HELPER", 52, analysis.KindUnknownEffect, "OPEN FOR"},
	}
	content, err := os.ReadFile("testdata/unknown/routines.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name        string
		first, last int
	}{
		{"native dynamic execution", 3, 8},
		{"DBMS_SQL execution", 12, 14},
		{"known effects", 22, 22},
		{"declaration-only call", 23, 23},
		{"unresolved call", 24, 24},
		{"marked wrapper propagation", 28, 30},
		{"recursive propagation", 32, 34},
		{"builtins and source shadowing", 37, 43},
		{"autonomous isolation", 47, 49},
		{"nested routine propagation", 52, 53},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var findings []analysis.Diagnostic
			for _, d := range report.Findings {
				if d.Location.Line >= scenario.first && d.Location.Line <= scenario.last {
					findings = append(findings, d)
				}
			}
			expected := want[:0:0]
			for _, w := range want {
				if w.line >= scenario.first && w.line <= scenario.last {
					expected = append(expected, w)
				}
			}
			if len(findings) != len(expected) {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), len(expected), findings)
			}
			for i, w := range expected {
				d := findings[i]
				if d.Routine != w.routine || d.Location.Line != w.line || d.Kind != w.kind || d.Callee != w.callee {
					t.Fatalf("finding %d: %+v, want %+v", i, d, w)
				}
				id := "CF0002"
				if w.kind == analysis.KindMissingPermission {
					id = "CF0001"
				}
				if d.ID != id || d.Severity != analysis.SeverityWarning || len(d.Evidence) == 0 || d.Evidence[0].Location != d.Location || d.Evidence[len(d.Evidence)-1].Location != d.Origin {
					t.Fatalf("invalid diagnostic: %+v", d)
				}
				assertSourceLocation(t, content, d.Location.File, d.Location, "")
				if (d.Routine == "APP.NATIVE" || d.Routine == "APP.DBMS_DYNAMIC") && d.Evidence[0].Description != "dynamic execution" {
					t.Fatalf("lost dynamic classification: %+v", d)
				}
				if d.Routine == "APP.TOP" && d.Kind == analysis.KindUnknownEffect && (len(d.Evidence) != 3 || d.Origin.Line != 24) {
					t.Fatalf("lost unknown chain: %+v", d)
				}
				if d.Routine == "APP.MIXED" && d.Location.Line == 24 && d.UnresolvedText != "Ext . Missing" {
					t.Fatalf("lost spelling: %+v", d)
				}
			}
		})
	}
}

func TestCheckUnknownOrdering(t *testing.T) {
	report := analysis.Check(analysis.Options{Paths: []string{"testdata/unknown"}})
	reordered := analysis.Check(analysis.Options{Paths: []string{"testdata/unknown/routines.sql", "testdata/unknown"}})
	if !reflect.DeepEqual(report, reordered) {
		t.Fatal("unstable ordering")
	}
}

func TestCheckSourceOverridesDynamicBuiltin(t *testing.T) {
	report := analysis.Check(analysis.Options{Paths: []string{"testdata/builtin_shadow"}})
	if len(report.Errors) != 0 || len(report.Findings) != 0 {
		t.Fatalf("source classification lost: %+v", report)
	}
}

func TestCheckAutonomousContexts(t *testing.T) {
	const fixture = "testdata/autonomous/routines.sql"
	report := analysis.Check(analysis.Options{Paths: []string{fixture}})
	want := []struct {
		routine, callee     string
		line, origin, steps int
		unknown             bool
	}{
		{"APP.AUDIT", "EXT.AUDIT_UNKNOWN", 5, 5, 1, true},
		{"APP.CONTEXT", "", 12, 12, 1, false},
		{"APP.CONTEXT.ORDINARY", "", 12, 12, 1, false},
		{"APP.CONTEXT", "EXT.ORDINARY_UNKNOWN", 13, 13, 1, true},
		{"APP.CONTEXT.ORDINARY", "EXT.ORDINARY_UNKNOWN", 13, 13, 1, true},
		{"APP.CONTEXT.ISOLATED", "EXT.ISOLATED_UNKNOWN", 19, 19, 1, true},
		{"APP.CONTEXT", "APP.CONTEXT.ORDINARY", 22, 12, 2, false},
		{"APP.CONTEXT", "APP.CONTEXT.ORDINARY", 22, 13, 2, true},
		{"APP.HOST", "", 33, 33, 1, false},
		{"APP.HOST", "EXT.HOST_UNKNOWN", 34, 34, 1, true},
		{"APP.CAPABILITY.ISOLATED", "EXT.CAPABILITY_UNKNOWN", 47, 47, 1, true},
		{"APP.MIXED", "EXT.AUTONOMOUS_UNKNOWN", 55, 55, 1, true},
		{"APP.MIXED", "", 60, 60, 1, false},
		{"APP.MIXED", "EXT.MIXED_UNKNOWN", 61, 61, 1, true},
		{"APP.CALLER", "APP.HOST", 68, 33, 2, false},
		{"APP.CALLER", "APP.HOST", 68, 34, 2, true},
		{"APP.CALLER", "APP.MIXED", 70, 60, 2, false},
		{"APP.CALLER", "APP.MIXED", 70, 61, 2, true},
	}
	content, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name        string
		first, last int
	}{
		{"internal diagnostics", 1, 7},
		{"nested helper propagation and isolation", 8, 25},
		{"nested pragma ownership", 26, 37},
		{"declared autonomous capability", 38, 50},
		{"mixed overload internal diagnostics", 51, 63},
		{"autonomous caller isolation", 64, 67},
		{"ordinary caller propagation", 68, 68},
		{"declared capability caller isolation", 69, 69},
		{"mixed overload outward aggregation", 70, 72},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if len(report.Errors) != 0 {
				t.Fatalf("analysis errors: %+v", report.Errors)
			}
			var findings []analysis.Diagnostic
			for _, d := range report.Findings {
				if d.Location.Line >= scenario.first && d.Location.Line <= scenario.last {
					findings = append(findings, d)
				}
			}
			expected := want[:0:0]
			for _, w := range want {
				if w.line >= scenario.first && w.line <= scenario.last {
					expected = append(expected, w)
				}
			}
			if len(findings) != len(expected) {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), len(expected), findings)
			}
			for i, w := range expected {
				d := findings[i]
				kind, id := analysis.KindMissingPermission, "CF0001"
				if w.unknown {
					kind, id = analysis.KindUnknownEffect, "CF0002"
				}
				if d.Routine != w.routine || d.Callee != w.callee || d.Location.Line != w.line || d.Origin.Line != w.origin || d.Kind != kind || d.ID != id || d.Severity != analysis.SeverityWarning || len(d.Evidence) != w.steps {
					t.Fatalf("finding %d: %+v, want %+v", i, d, w)
				}
				if d.Evidence[0].Location != d.Location || d.Evidence[len(d.Evidence)-1].Location != d.Origin {
					t.Fatalf("broken evidence: %+v", d)
				}
				assertSourceLocation(t, content, d.Location.File, d.Location, "")
				assertSourceLocation(t, content, d.Origin.File, d.Origin, "")
			}
		})
	}
}

func TestCheckAutonomousInputOrdering(t *testing.T) {
	const fixture = "testdata/autonomous/routines.sql"
	report := analysis.Check(analysis.Options{Paths: []string{fixture}})
	reordered := analysis.Check(analysis.Options{Paths: []string{"testdata/autonomous", fixture}})
	if !reflect.DeepEqual(report, reordered) {
		t.Fatal("unstable autonomous findings")
	}
}

func TestCheckTransactionDeclarations(t *testing.T) {
	opts := analysis.Options{Paths: []string{"testdata/externals/source.sql"}, ExternalsPath: "testdata/externals/valid.json"}
	got := analysis.Check(opts)
	if len(got.Errors) != 1 || got.Errors[0].Kind != analysis.KindDeclarationConflict || got.Errors[0].Routine != "APP.CONFLICT" {
		t.Fatalf("conflicts: %+v", got.Errors)
	}
	conflict := got.Errors[0]
	if conflict.Location.Line != 7 || conflict.Origin.Line != 16 || len(conflict.Evidence) != 1 {
		t.Fatalf("conflict evidence: %+v", conflict)
	}
	counts := map[string]int{}
	for _, d := range got.Findings {
		counts[d.Routine+"|"+d.Callee+"|"+string(d.Kind)]++
		if d.Routine == "APP.CALLER" && d.Callee == "EXT.API.UNSAFE_CALL" {
			if d.Origin.Line != 5 || !strings.HasSuffix(d.Origin.File, "valid.json") || len(d.Evidence) != 2 || d.Evidence[1].Description != "external unsafe declaration" {
				t.Fatalf("declaration evidence: %+v", d)
			}
		}
	}
	want := map[string]int{
		"APP.CALLER|EXT.API.UNSAFE_CALL|missing-permission": 1,
		"APP.CALLER|EXT.API.MISSING_CALL|unknown-effect":    1,
		"APP.CALLER|EXT.MIXED.RUN|unknown-effect":           1,
		"APP.CALLER|APP.CONFLICT|missing-permission":        1,
		"APP.CALLER|APP.UNCERTAIN|missing-permission":       1,
		"APP.CALLER|APP.UNCERTAIN|unknown-effect":           1,
		"APP.CONFLICT||missing-permission":                  1,
		"APP.UNCERTAIN|EXT.OTHER.RUN|unknown-effect":        1,
		"APP.ISOLATED|EXT.OTHER.RUN|unknown-effect":         1,
	}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("findings: %+v\nreport: %+v", counts, got)
	}
	if !reflect.DeepEqual(got, analysis.Check(opts)) {
		t.Fatal("nondeterministic report")
	}
}

func TestCheckInvalidExternals(t *testing.T) {
	t.Run("entry diagnostics", func(t *testing.T) {
		const doc = `{
  "version": 1,
  "routines": [
    null,
    {},
    {"name":"run","transaction":"safe"},
    {"name":"a.b","transaction":"maybe"},
    {"name":"a.b.c.d","transaction":"unsafe"},
    {"name":"app.run","transaction":"safe"},
    {"name":"APP.RUN","transaction":"unsafe"}
  ]
}`
		path := filepath.Join(t.TempDir(), "externals.json")
		if err := os.WriteFile(path, []byte(doc), 0600); err != nil {
			t.Fatal(err)
		}
		got := analysis.Check(analysis.Options{Paths: []string{"testdata/externals/source.sql"}, ExternalsPath: path})
		want := []struct {
			line    int
			message string
		}{
			{4, "transaction declaration 1: name must be SCHEMA.ROUTINE or SCHEMA.PACKAGE.ROUTINE"},
			{5, "transaction declaration 2: name must be SCHEMA.ROUTINE or SCHEMA.PACKAGE.ROUTINE"},
			{6, "transaction declaration 3: name must be SCHEMA.ROUTINE or SCHEMA.PACKAGE.ROUTINE"},
			{7, "transaction declaration 4: transaction must be safe or unsafe"},
			{8, "transaction declaration 5: name must be SCHEMA.ROUTINE or SCHEMA.PACKAGE.ROUTINE"},
			{10, "transaction declaration 7: duplicate normalized identity APP.RUN"},
		}
		if len(got.Errors) != len(want) {
			t.Fatalf("errors: %+v", got.Errors)
		}
		if len(got.Findings) == 0 {
			t.Fatal("invalid config stopped independent source analysis")
		}
		lines := strings.Split(doc, "\n")
		for i, expected := range want {
			offset := 4 // Each offending entry starts after four spaces.
			for _, line := range lines[:expected.line-1] {
				offset += len(line) + 1
			}
			e := got.Errors[i]
			loc := analysis.Location{File: path, Line: expected.line, Column: 5, Offset: offset}
			if e.ID != "CF1003" || e.Kind != analysis.KindConfiguration || e.Message != expected.message || e.Location != loc {
				t.Errorf("entry diagnostic %d: got %+v, want message %q at %+v", i, e, expected.message, loc)
			}
		}
	})
	cases := []struct {
		name, doc string
		errors    int
	}{
		{"malformed", "{", 1},
		{"version", `{"version":2,"routines":[]}`, 1},
		{"missing version", `{"routines":[]}`, 1},
		{"missing routines", `{"version":1}`, 1},
		{"quoted distinct", `{"version":1,"routines":[{"name":"app.\"Run\"","transaction":"safe"},{"name":"APP.RUN","transaction":"unsafe"}]}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "externals.json")
			if err := os.WriteFile(path, []byte(tc.doc), 0600); err != nil {
				t.Fatal(err)
			}
			got := analysis.Check(analysis.Options{Paths: []string{"testdata/externals/source.sql"}, ExternalsPath: path})
			if len(got.Errors) != tc.errors {
				t.Fatalf("errors: %+v", got.Errors)
			}
			for _, e := range got.Errors {
				if e.ID != "CF1003" || e.Kind != analysis.KindConfiguration || e.Location.File != path || e.Location.Line < 1 || e.Location.Column < 1 {
					t.Fatalf("configuration location: %+v", e)
				}
			}
			if len(got.Findings) == 0 {
				t.Fatal("invalid config stopped independent source analysis")
			}
		})
	}
	got := analysis.Check(analysis.Options{Paths: []string{"testdata/externals/source.sql"}, ExternalsPath: "testdata/externals/absent.json"})
	if len(got.Errors) != 1 || got.Errors[0].Kind != analysis.KindConfiguration {
		t.Fatalf("missing file: %+v", got.Errors)
	}
}

func TestCheckDeclarationBoundaries(t *testing.T) {
	got := analysis.Check(analysis.Options{Paths: []string{"testdata/externals/boundaries.sql"}, ExternalsPath: "testdata/externals/boundaries.json"})
	if len(got.Errors) != 2 || got.Errors[0].Routine != "APP.API.ADVERTISED" || got.Errors[0].Kind != analysis.KindDeclarationConflict || got.Errors[0].Origin.Line != 2 {
		t.Fatalf("capability conflict: %+v", got.Errors)
	}
	unknown := got.Errors[1]
	if unknown.ID != "CF1004" || unknown.Kind != analysis.KindDeclarationConflict || unknown.Severity != analysis.SeverityError || unknown.Routine != "APP.UNKNOWN_BODY" || len(unknown.Evidence) != 1 {
		t.Fatalf("unknown dependency conflict: %+v", unknown)
	}
	externalsPath, _ := filepath.Abs("testdata/externals/boundaries.json")
	sourcePath, _ := filepath.Abs("testdata/externals/boundaries.sql")
	declarationsText, err := os.ReadFile(externalsPath)
	if err != nil {
		t.Fatal(err)
	}
	sourceText, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	assertSourceLocation(t, declarationsText, externalsPath, unknown.Location, "{\"name\":\"app.unknown_body\"")
	assertSourceLocation(t, sourceText, sourcePath, unknown.Origin, "ext.unknown;")
	if unknown.Location.Line != 5 || unknown.Origin.Line != 27 || unknown.Evidence[0].Location != unknown.Origin || unknown.Evidence[0].Routine != "APP.UNKNOWN_BODY" || unknown.Evidence[0].Callee != "EXT.UNKNOWN" {
		t.Fatalf("unknown conflict evidence: %+v", unknown)
	}
	counts := map[string]int{}
	for _, d := range got.Findings {
		counts[d.Routine+"|"+d.Callee+"|"+string(d.Kind)]++
	}
	want := map[string]int{
		"APP.AUTO|EXT.UNKNOWN|unknown-effect":               1,
		"APP.WRAPPER|APP.API.ADVERTISED|missing-permission": 1,
		"APP.WRAPPER|APP.EMPTY_BODY|missing-permission":     1,
		"APP.UNKNOWN_BODY|EXT.UNKNOWN|unknown-effect":       1,
	}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("boundaries: %+v", got)
	}
}

func TestCheckShortestEvidence(t *testing.T) {
	got := analysis.Check(analysis.Options{Paths: []string{"testdata/shortest.sql"}})
	if len(got.Errors) != 0 {
		t.Fatalf("errors: %+v", got.Errors)
	}
	for _, d := range got.Findings {
		if d.Routine != "APP.UPSTREAM" {
			continue
		}
		var chain []string
		for _, e := range d.Evidence {
			chain = append(chain, e.Routine)
		}
		if !reflect.DeepEqual(chain, []string{"APP.UPSTREAM", "APP.BRANCH", "APP.LEAF"}) || d.Origin.Line != 1 || filepath.Base(d.Origin.File) != "shortest.sql" || d.Evidence[1].Callee != "APP.LEAF" {
			t.Fatalf("shortest evidence: %+v", d)
		}
		return
	}
	t.Fatal("missing upstream finding")
}

func TestCheckSafeUnknownConflicts(t *testing.T) {
	for _, body := range []string{"ext.unknown;", "EXECUTE IMMEDIATE 'select 1 from dual';"} {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source.sql")
			externalsPath := filepath.Join(dir, "externals.json")
			writeFixture(t, source, "CREATE PROCEDURE app.dependency IS BEGIN "+body+" END dependency;\nCREATE PROCEDURE app.wrapper IS BEGIN app.dependency; END wrapper;\n")
			writeFixture(t, externalsPath, `{"version":1,"routines":[{"name":"app.wrapper","transaction":"safe"}]}`)
			got := analysis.Check(analysis.Options{Paths: []string{source}, ExternalsPath: externalsPath})
			if len(got.Errors) != 1 || got.Errors[0].Kind != analysis.KindDeclarationConflict || got.Errors[0].Severity != analysis.SeverityError || got.Errors[0].Routine != "APP.WRAPPER" || got.Errors[0].Origin.Line != 1 || len(got.Errors[0].Evidence) != 2 {
				t.Fatalf("unknown conflict: %+v", got)
			}
			if len(got.Findings) != 2 {
				t.Fatalf("unknown warnings lost: %+v", got.Findings)
			}
		})
	}
}

func TestCheckOversizedExternalsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = file.Truncate((16 << 20) + 1)
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	got := analysis.Check(analysis.Options{Paths: []string{"testdata/externals/source.sql"}, ExternalsPath: path})
	if len(got.Errors) != 1 || got.Errors[0].Kind != analysis.KindConfiguration || got.Errors[0].Message != "externals file exceeds 16 MiB limit" || got.Errors[0].Location.File != path || len(got.Findings) == 0 {
		t.Fatalf("bounded externals file read: %+v", got)
	}
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
