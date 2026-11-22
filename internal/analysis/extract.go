package analysis

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
)

const ident = `(?:"(?:[^"]|"")+"|[A-Za-z_][A-Za-z0-9_$#]*)`
const qualified = ident + `(?:\s*\.\s*` + ident + `)*`

const createPrefix = `CREATE\s+(?:OR\s+REPLACE\s+)?(?:EDITIONABLE\s+|NONEDITIONABLE\s+)?`

var headers = regexp.MustCompile(`(?i)\b(?:(` + createPrefix + `)?(PROCEDURE|FUNCTION)|(` + createPrefix + `PACKAGE\s+(?:BODY\s+)?))\s*(` + qualified + `)`)
var effectPattern = regexp.MustCompile(`(?i)\b(COMMIT|ROLLBACK)\b`)
var autonomyPattern = regexp.MustCompile(`(?i)\bPRAGMA\s+AUTONOMOUS_TRANSACTION\s*;`)
var nameParts = regexp.MustCompile(ident)
var quotedNames = regexp.MustCompile(`"(?:[^"]|"")*"`)

// FindAllSubmatchIndex stores each capture as a start/end pair.
const (
	headerCreate  = 2
	headerPackage = 6
	headerName    = 8
)

type routine struct {
	file                                      sourceFile
	name, leaf, group                         string
	start, body, end                          int
	permissionOffset                          int
	definition, valid, permission, autonomous bool
	packageScope                              bool
	parent                                    *routine
	isolated                                  []sourceSpan
}
type sourceSpan struct{ start, end int }
type marker struct{ start, end int }
type extracted struct {
	routines []*routine
	masked   []byte
}

func location(f sourceFile, offset int) Location {
	line := sort.Search(len(f.lineStarts), func(i int) bool { return f.lineStarts[i] > offset })
	return Location{File: f.path, Line: line, Column: offset - f.lineStarts[line-1] + 1, Offset: offset}
}
func normalized(s string) string {
	parts := nameParts.FindAllString(s, -1)
	for i, p := range parts {
		if p[0] != '"' {
			parts[i] = strings.ToUpper(p)
		}
	}
	return strings.Join(parts, ".")
}

// mask replaces comments and string literals with spaces, retaining byte positions
// and every newline. Quoted identifiers remain available for identity matching.
func mask(src []byte) ([]byte, []marker) {
	out := bytes.Clone(src)
	var markers []marker
	blank := func(a, b int) {
		for k := a; k < b; k++ {
			if out[k] != '\n' && out[k] != '\r' {
				out[k] = ' '
			}
		}
	}
	for i := 0; i < len(src); {
		start := i
		switch {
		case src[i] == '"':
			i++
			for i < len(src) {
				if src[i] == '"' {
					i++
					if i < len(src) && src[i] == '"' {
						i++
						continue
					}
					break
				}
				i++
			}
		case i+1 < len(src) && src[i] == '-' && src[i+1] == '-':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			line := bytes.LastIndexByte(src[:start], '\n') + 1
			if len(bytes.TrimSpace(src[line:start])) == 0 && strings.TrimSpace(string(src[start:i])) == "--@commitfence" {
				markers = append(markers, marker{start, i})
			}
			blank(start, i)
		case i+1 < len(src) && src[i] == '/' && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i = min(i+2, len(src))
			blank(start, i)
		case i+2 < len(src) && (src[i] == 'q' || src[i] == 'Q') && src[i+1] == '\'':
			close := src[i+2]
			switch close {
			case '[':
				close = ']'
			case '{':
				close = '}'
			case '(':
				close = ')'
			case '<':
				close = '>'
			}
			i += 3
			for i+1 < len(src) && !(src[i] == close && src[i+1] == '\'') {
				i++
			}
			i = min(i+2, len(src))
			blank(start, i)
		case src[i] == '\'':
			i++
			for i < len(src) {
				if src[i] == '\'' {
					i++
					if i < len(src) && src[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			blank(start, i)
		default:
			i++
		}
	}
	return out, markers
}

func extractionError(report *Report, f sourceFile, offset int, kind Kind, id, name, message string) {
	report.Errors = append(report.Errors, AnalysisError{Diagnostic{ID: id, Severity: SeverityError, Kind: kind, Location: location(f, offset), Routine: name, Message: message}})
}

type boundaryToken struct {
	start, end int
	word       string
}

// Scan only identifiers and punctuation, retaining byte offsets. Comments and
// literals have already been masked; quoted identifiers are never keywords.
func nextBoundaryToken(code []byte, pos int) boundaryToken {
	for pos < len(code) && (code[pos] == ' ' || code[pos] == '\t' || code[pos] == '\r' || code[pos] == '\n' || code[pos] == '\f') {
		pos++
	}
	end := pos
	if end < len(code) {
		switch {
		case code[end] == '"':
			end++
			for end < len(code) {
				if code[end] == '"' {
					end++
					if end >= len(code) || code[end] != '"' {
						break
					}
				}
				end++
			}
		case identifierByte(code[end]):
			for end < len(code) && identifierByte(code[end]) {
				end++
			}
		case end+1 < len(code) && (code[end] == '<' && code[end+1] == '<' || code[end] == '>' && code[end+1] == '>'):
			end += 2
		default:
			end++
		}
	}
	word := string(code[pos:end])
	if len(word) > 0 && word[0] != '"' {
		word = strings.ToUpper(word)
	}
	return boundaryToken{pos, end, word}
}

func extract(f sourceFile, report *Report) extracted {
	f.lineStarts = []int{0}
	for i, b := range f.text {
		if b == '\n' {
			f.lineStarts = append(f.lineStarts, i+1)
		}
	}
	code, markers := mask(f.text)
	// Keep quoted identities in headers and END names, but do not interpret their
	// contents as keywords, pragmas, separators, or transaction statements.
	keywords := bytes.Clone(code)
	for _, span := range quotedNames.FindAllIndex(code, -1) {
		for i := span[0]; i < span[1]; i++ {
			if keywords[i] != '\n' && keywords[i] != '\r' {
				keywords[i] = ' '
			}
		}
	}
	var hs [][]int
	for _, h := range headers.FindAllSubmatchIndex(code, -1) {
		// Regex word boundaries alone do not exclude PL/SQL's $/# identifiers
		// or a keyword prefix such as procedure_name.
		if keywords[h[0]] == ' ' || h[0] > 0 && identifierByte(code[h[0]-1]) ||
			identifierByte(code[h[headerName]]) && identifierByte(code[h[headerName]-1]) {
			continue
		}
		hs = append(hs, h)
	}
	var records []*routine
	// Every frame has its actual lexical owner. Package frames determine
	// containment but never acquire permission or contribute findings.
	type frame struct {
		kind, label string
		owner       *routine
		begun       bool
	}
	var stack []frame
	owner := func() *routine {
		if len(stack) == 0 {
			return nil
		}
		return stack[len(stack)-1].owner
	}
	fail := func(r *routine, pos int, message string) {
		if r == nil {
			extractionError(report, f, pos, KindIncompleteAnalysis, "CF1004", "", message)
		} else if r.valid {
			r.valid = false
			extractionError(report, f, r.start, KindIncompleteAnalysis, "CF1004", r.name, message)
		}
	}
	reset := func(pos int) {
		for _, fr := range stack {
			if fr.owner != nil {
				fail(fr.owner, pos, "missing or ambiguous END boundary")
				fr.owner.end = pos
			}
		}
		stack = nil
	}
	label := ""
	for pos, headerCursor := 0, 0; pos < len(code); {
		t := nextBoundaryToken(code, pos)
		pos = t.end
		if t.word == "" {
			break
		}
		if headerCursor < len(hs) && t.start == hs[headerCursor][0] {
			h := hs[headerCursor]
			headerCursor++
			pkg := h[headerPackage] >= 0
			if h[headerCreate] >= 0 || pkg {
				reset(h[0])
			}
			label = ""
			raw := string(code[h[headerName]:h[headerName+1]])
			name := normalized(raw)
			parts := nameParts.FindAllString(raw, -1)
			r := &routine{file: f, name: name, group: name, leaf: normalized(parts[len(parts)-1]), start: h[0], end: h[1], valid: true, packageScope: pkg, parent: owner()}
			if r.parent != nil {
				r.name = r.parent.name + "." + name
				r.group = r.name
				if !r.parent.packageScope {
					// Lexical helpers cannot borrow a schema/package object's permission.
					r.group = r.parent.group + "\x00" + name
				}
			}
			// Skip headers as a unit: defaults/types may contain parentheses or
			// keyword-like quoted identifiers. Only outer separators matter.
			limit := len(code)
			if headerCursor < len(hs) {
				limit = hs[headerCursor][0]
			}
			depth := 0
			for cursor := h[1]; cursor < limit; {
				w := nextBoundaryToken(code, cursor)
				cursor = w.end
				if w.start >= limit {
					break
				}
				switch w.word {
				case "(":
					depth++
				case ")":
					depth--
				case "IS", "AS", ";":
					if depth == 0 {
						r.body = w.end
						r.definition = w.word != ";"
					}
				}
				if r.body != 0 {
					break
				}
			}
			pos = h[1]
			if r.body == 0 {
				fail(r, r.start, "unterminated routine header")
			} else {
				pos = r.body
			}
			if !pkg {
				records = append(records, r)
			}
			if r.definition {
				kind := "ROUTINE"
				if pkg {
					kind = "PACKAGE"
				}
				stack = append(stack, frame{kind: kind, owner: r})
			} else if r.valid {
				r.end = r.body
			}
			continue
		}
		switch t.word {
		case "/":
			a := bytes.LastIndexByte(code[:t.start], '\n') + 1
			b := t.end
			if n := bytes.IndexByte(code[b:], '\n'); n >= 0 {
				b += n
			} else {
				b = len(code)
			}
			if len(bytes.TrimSpace(code[a:t.start])) == 0 && len(bytes.TrimSpace(code[t.end:b])) == 0 {
				reset(t.start)
				label = ""
			}
		case "<<":
			name := nextBoundaryToken(code, pos)
			close := nextBoundaryToken(code, name.end)
			if close.word == ">>" {
				label, pos = name.word, close.end
			}
		case "DECLARE":
			stack = append(stack, frame{kind: "BLOCK", label: label, owner: owner()})
			label = ""
		case "BEGIN":
			if len(stack) > 0 && !stack[len(stack)-1].begun && label == "" {
				stack[len(stack)-1].begun = true
			} else {
				stack = append(stack, frame{kind: "BLOCK", label: label, owner: owner(), begun: true})
			}
			label = ""
		case "IF", "LOOP", "CASE":
			kind := t.word
			if kind == "CASE" {
				// THEN/ELSE within a CASE expression introduce expressions too.
				inExpression := len(stack) > 0 && stack[len(stack)-1].kind == "CASE_EXPR"
				if inExpression || label == "" && !statementContext(keywords, 0, t.start) {
					kind = "CASE_EXPR"
				}
			}
			stack = append(stack, frame{kind: kind, label: label, owner: owner(), begun: true})
			label = ""
		case "END":
			next := nextBoundaryToken(code, pos)
			if len(stack) > 0 && stack[len(stack)-1].kind == "CASE_EXPR" && next.word != "CASE" && next.word != "IF" && next.word != "LOOP" {
				// SQL/procedural CASE expressions end without a suffix or even a
				// semicolon (e.g. END + 1). They never close a surrounding block.
				stack = stack[:len(stack)-1]
				continue
			}
			kind, endName := "", ""
			if next.word == "IF" || next.word == "LOOP" || next.word == "CASE" {
				kind = next.word
				next = nextBoundaryToken(code, next.end)
			}
			if next.word != ";" {
				endName = next.word
				next = nextBoundaryToken(code, next.end)
			}
			if next.word != ";" {
				fail(owner(), t.start, "missing or ambiguous END boundary")
				continue
			}
			pos = next.end
			if len(stack) == 0 {
				fail(nil, t.start, "unmatched END boundary")
				continue
			}
			fr := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			expected := fr.label
			if fr.kind == "ROUTINE" || fr.kind == "PACKAGE" {
				expected = fr.owner.leaf
				fr.owner.end = pos
			}
			control := fr.kind == "IF" || fr.kind == "LOOP" || fr.kind == "CASE"
			if control && kind != fr.kind || !control && kind != "" ||
				endName != "" && endName != expected || !fr.begun && fr.kind != "PACKAGE" {
				fail(fr.owner, t.start, "mismatched END boundary")
			}
		case ";":
			label = ""
		}
	}
	reset(len(code))
	markerRoutineCursor := 0
	for _, m := range markers {
		var target *routine
		for markerRoutineCursor < len(records) && records[markerRoutineCursor].start < m.end {
			markerRoutineCursor++
		}
		if markerRoutineCursor < len(records) {
			r := records[markerRoutineCursor]
			if len(bytes.TrimSpace(code[m.end:r.start])) == 0 {
				target = r
			}
		}
		if target == nil {
			extractionError(report, f, m.start, KindInvalidAnnotation, "CF1005", "", "marker is not attached to a routine declaration or definition")
		} else {
			target.permission = true
			target.permissionOffset = m.start
		}
	}
	for _, r := range records {
		if !r.valid || !r.definition {
			continue
		}
		for _, p := range autonomyPattern.FindAllIndex(keywords[r.body:r.end], -1) {
			pos := r.body + p[0]
			if ownsPosition(r, records, pos) {
				r.autonomous = true
			}
		}
	}
	// These spans exclude autonomous descendants from each enclosing
	// routine's transaction analysis.
	for _, child := range records {
		if !child.autonomous {
			continue
		}
		for parent := child.parent; parent != nil; parent = parent.parent {
			parent.isolated = append(parent.isolated, sourceSpan{child.start, child.end})
			if parent.autonomous {
				break
			}
		}
	}
	return extracted{records, keywords}
}

// ownsPosition excludes all nested routines: even an ordinary helper's pragma
// establishes only that helper's transaction context.
func ownsPosition(r *routine, records []*routine, pos int) bool {
	for _, child := range records {
		if child != r && child.start > r.start && child.start <= pos && pos < child.end {
			return false
		}
	}
	return true
}

func directFindings(inputs []extracted, permissions map[string]bool, report *Report) {
	for _, input := range inputs {
		for _, r := range input.routines {
			if !r.valid || !r.definition || permissions[r.group] {
				continue
			}
			for _, e := range recognizedEffects(input.masked, r) {
				loc, effect := e.Location, e.Description
				if r.autonomous && ownsPosition(r, input.routines, loc.Offset) {
					continue
				}
				report.Findings = append(report.Findings, Diagnostic{ID: "CF0001", Severity: SeverityWarning, Kind: KindMissingPermission, Location: loc, Routine: r.name, Origin: loc, Message: effect + " requires transaction permission", Evidence: []Evidence{{Location: loc, Routine: r.name, Description: effect}}})
			}
		}
	}
}

func statementContext(code []byte, start, pos int) bool {
	i := pos - 1
	for i >= start && (code[i] == ' ' || code[i] == '\t' || code[i] == '\r' || code[i] == '\n') {
		i--
	}
	if i < start || code[i] == ';' {
		return true
	}
	end := i + 1
	for i >= start && ((code[i] >= 'A' && code[i] <= 'Z') || (code[i] >= 'a' && code[i] <= 'z') || code[i] == '_') {
		i--
	}
	word := strings.ToUpper(string(code[i+1 : end]))
	return word == "BEGIN" || word == "THEN" || word == "ELSE" || word == "LOOP"
}

func identifierByte(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_' || b == '$' || b == '#'
}

// recognizedEffects shares statement recognition between diagnostics and summaries.
func recognizedEffects(code []byte, r *routine) []Evidence {
	var result []Evidence
	for _, match := range effectPattern.FindAllIndex(code[r.body:r.end], -1) {
		pos, end := r.body+match[0], r.body+match[1]
		if excluded(r, pos) || pos > 0 && identifierByte(code[pos-1]) || end < len(code) && identifierByte(code[end]) || !statementContext(code, r.body, pos) {
			continue
		}
		result = append(result, Evidence{Location: location(r.file, pos), Routine: r.name, Description: strings.ToUpper(string(code[pos:end]))})
	}
	return result
}
