package analysis

import (
	"cmp"
	"slices"
)

// Check reads the selected real files and returns failures while continuing
// independent inputs. External declarations and source are analyzed together.
func Check(options Options) Report {
	report := Report{Files: []string{}, Findings: []Diagnostic{}, Errors: []AnalysisError{}}
	declarations := loadDeclarations(options.ExternalsPath, &report)
	sources := loadSources(options.Paths, options.Extensions, &report)
	var inputs []extracted
	for _, source := range sources {
		inputs = append(inputs, extract(source, &report))
	}
	groups, permissions, capabilities := buildGroups(inputs, declarations)
	directFindings(inputs, permissions, &report)
	graphFindings(inputs, declarations, groups, permissions, capabilities, &report)
	slices.SortFunc(report.Findings, compareDiagnostic)
	slices.SortFunc(report.Errors, func(a, b AnalysisError) int {
		if n := cmp.Compare(a.Location.File, b.Location.File); n != 0 {
			return n
		}
		if n := cmp.Compare(a.Location.Offset, b.Location.Offset); n != 0 {
			return n
		}
		if n := cmp.Compare(a.ID, b.ID); n != 0 {
			return n
		}
		return cmp.Compare(a.Message, b.Message)
	})
	report.Errors = slices.CompactFunc(report.Errors, func(a, b AnalysisError) bool {
		return a.ID == b.ID && a.Location == b.Location && a.Message == b.Message
	})
	return report
}

func compareDiagnostic(a, b Diagnostic) int {
	if n := cmp.Compare(a.Location.File, b.Location.File); n != 0 {
		return n
	}
	if n := cmp.Compare(a.Location.Offset, b.Location.Offset); n != 0 {
		return n
	}
	if n := cmp.Compare(a.Routine, b.Routine); n != 0 {
		return n
	}
	return cmp.Compare(a.ID, b.ID)
}
