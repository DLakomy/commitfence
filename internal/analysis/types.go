// Package analysis reads source inputs and returns structured results without
// printing, exiting, modifying source, or contacting a database.
package analysis

// Options selects source inputs and an optional JSON file with external transaction declarations.
type Options struct {
	Paths         []string
	ExternalsPath string
	// Extensions filters directory scans only. Nil selects DefaultExtensions().
	Extensions []string
}

// Location uses a canonical absolute file path, one-based line and byte column,
// and a zero-based byte offset. Line and column are zero for file-level errors.
type Location struct {
	File                 string
	Line, Column, Offset int
}

type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Kind string

const (
	KindMissingPermission   Kind = "missing-permission"
	KindUnknownEffect       Kind = "unknown-effect"
	KindInvalidAnnotation   Kind = "invalid-annotation"
	KindDeclarationConflict Kind = "declaration-conflict"
	KindInput               Kind = "input"
	KindConfiguration       Kind = "configuration"
	KindIncompleteAnalysis  Kind = "incomplete-analysis"
)

// Evidence records a representative step from the affected routine to an origin.
type Evidence struct {
	Location    Location
	Routine     string
	Callee      string
	Description string
}

// Diagnostic retains context for both direct and transitive findings.
type Diagnostic struct {
	ID             string
	Severity       Severity
	Kind           Kind
	Location       Location
	Message        string
	Routine        string
	Callee         string
	UnresolvedText string
	Origin         Location
	Evidence       []Evidence
}

// AnalysisError is a recognized input, configuration, or analysis failure.
// Its diagnostic always has error severity.
type AnalysisError struct {
	Diagnostic
}

// Report lists successfully read files, findings, and failures in deterministic
// order. Files lists successfully read inputs, including files with analysis errors.
type Report struct {
	Files    []string
	Findings []Diagnostic
	Errors   []AnalysisError
}
