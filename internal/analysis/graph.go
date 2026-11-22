package analysis

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
)

// Candidates cover procedure statements and parenthesized functions, including
// expressions in SELECT, DML values, predicates, and procedural assignments.
var callPattern = regexp.MustCompile(`(` + qualified + `)\s*([(;])`)

// These are syntax/type words, not calls. Built-ins are deliberately enumerated;
// a resolved source identity always takes precedence over this list.
var syntaxWords = wordSet("BEGIN END IF THEN ELSE ELSIF LOOP WHILE FOR CASE WHEN RETURN NULL COMMIT ROLLBACK SAVEPOINT EXIT CONTINUE RAISE DECLARE EXCEPTION IS AS PROCEDURE FUNCTION PRAGMA AUTONOMOUS_TRANSACTION SELECT INSERT UPDATE DELETE MERGE VALUES IN EXISTS NOT AND OR NUMBER VARCHAR2 VARCHAR CHAR NCHAR NVARCHAR2 RAW INTEGER PLS_INTEGER BOOLEAN DATE TIMESTAMP INTERVAL RECORD TABLE TYPE CURSOR")
var dynamicBuiltins = wordSet("DBMS_SQL.PARSE DBMS_SQL.EXECUTE DBMS_SQL.EXECUTE_AND_FETCH")
var safeBuiltins = wordSet("ABS CEIL FLOOR MOD ROUND TRUNC LENGTH LOWER UPPER SUBSTR REPLACE TRIM LTRIM RTRIM NVL NVL2 COALESCE NULLIF TO_CHAR TO_DATE TO_NUMBER COUNT SUM MIN MAX AVG DBMS_OUTPUT.PUT_LINE DBMS_OUTPUT.PUT")

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for w := range strings.FieldsSeq(s) {
		m[w] = true
	}
	return m
}

type call struct {
	pos           int
	text, target  string
	raw           string
	unknownReason string
}
type summary struct{ effect, unknown *evidencePath }
type graphNode struct {
	r     *routine
	seed  summary
	calls []call
}

func excluded(r *routine, pos int) bool {
	if pos < r.body || pos >= r.end {
		return true
	}
	for _, span := range r.isolated {
		if span.start <= pos && pos < span.end {
			return true
		}
	}
	return false
}

func resolveCall(r *routine, name string, groups map[string][]*routine) string {
	// Search lexical scopes from the current body outwards, including package
	// members. Never construct an omitted schema or search by final segment.
	for scope := r; scope != nil; scope = scope.parent {
		sep := "\x00"
		if scope.packageScope {
			sep = "."
		}
		if key := scope.group + sep + name; len(groups[key]) > 0 {
			return key
		}
		if after, ok := strings.CutPrefix(name, scope.leaf+"."); ok {
			if key := scope.group + sep + after; len(groups[key]) > 0 {
				return key
			}
		}
		if name == scope.leaf && len(groups[scope.group]) > 0 {
			return scope.group
		}
	}
	// A standalone object's explicitly written schema is available context.
	top := r
	for top.parent != nil {
		top = top.parent
	}
	parts := nameParts.FindAllString(top.name, -1)
	if len(parts) > 1 {
		if key := parts[0] + "." + name; len(groups[key]) > 0 {
			return key
		}
	}
	if len(groups[name]) > 0 {
		return name
	}
	return ""
}

// Evidence paths share immutable tails; only emitted diagnostics materialize
// slices. Length and origin are cached so extending a chain takes constant work.
type evidencePath struct {
	step   Evidence
	next   *evidencePath
	length int
	origin Location
}

func origin(e Evidence) *evidencePath { return &evidencePath{step: e, length: 1, origin: e.Location} }
func materialize(p *evidencePath) []Evidence {
	if p == nil {
		return nil
	}
	result := make([]Evidence, 0, p.length)
	for ; p != nil; p = p.next {
		result = append(result, p.step)
	}
	return result
}

// Prefer shortest paths, then compare source order from caller to origin.
func better(a, b *evidencePath) *evidencePath {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.length != b.length {
		if a.length < b.length {
			return a
		}
		return b
	}
	for x, y := a, b; x != nil && x != y; x, y = x.next, y.next {
		for _, n := range []int{cmp.Compare(x.step.Location.File, y.step.Location.File), cmp.Compare(x.step.Location.Offset, y.step.Location.Offset), cmp.Compare(x.step.Routine, y.step.Routine), cmp.Compare(x.step.Callee, y.step.Callee), cmp.Compare(x.step.Description, y.step.Description)} {
			if n < 0 {
				return a
			}
			if n > 0 {
				return b
			}
		}
	}
	return a
}
func prepend(r *routine, c call, p *evidencePath) *evidencePath {
	if p == nil {
		return nil
	}
	return &evidencePath{step: Evidence{Location: location(r.file, c.pos), Routine: r.name, Callee: c.text, Description: "call"}, next: p, length: p.length + 1, origin: p.origin}
}

// Read only the adjacent token rather than searching the whole source prefix.
func previousWord(code []byte, pos int) string {
	i := pos
	for i > 0 && (code[i-1] == ' ' || code[i-1] == '\t' || code[i-1] == '\n' || code[i-1] == '\r' || code[i-1] == '\f') {
		i--
	}
	end := i
	for i > 0 && identifierByte(code[i-1]) {
		i--
	}
	return strings.ToUpper(string(code[i:end]))
}

func buildGroups(inputs []extracted, declarations map[string]transactionDeclaration) (map[string][]*routine, map[string]bool, map[string]*evidencePath) {
	groups := map[string][]*routine{}
	permissions := map[string]bool{}
	capabilities := map[string]*evidencePath{}
	for _, input := range inputs {
		for _, r := range input.routines {
			groups[r.group] = append(groups[r.group], r)
			permissions[r.group] = permissions[r.group] || r.permission
			if r.permission {
				capabilities[r.group] = better(capabilities[r.group], origin(Evidence{Location: location(r.file, r.permissionOffset), Routine: r.name, Description: "transaction capability"}))
			}
		}
	}

	for key, declaration := range declarations {
		if declaration.grantsPermission() {
			permissions[key] = true
			capabilities[key] = better(capabilities[key], declarationEvidence(declaration))
		}
	}
	return groups, permissions, capabilities
}

func graphFindings(inputs []extracted, declarations map[string]transactionDeclaration, groups map[string][]*routine, permissions map[string]bool, capabilities map[string]*evidencePath, report *Report) {
	sourceGroups := map[string]bool{}
	for key := range groups {
		sourceGroups[key] = true
	}
	for key := range declarations {
		if !sourceGroups[key] {
			groups[key] = []*routine{{name: key, group: key}}
		}
	}
	summaries := map[string]summary{}
	for key, declaration := range declarations {
		if !sourceGroups[key] && declaration.transaction == "unsafe" {
			summaries[key] = summary{effect: declarationEvidence(declaration)}
		}
	}

	var nodes []graphNode
	for _, input := range inputs {
		if len(input.routines) == 0 {
			continue
		}
		code, _ := mask(input.routines[0].file.text)
		quoted := quotedNames.FindAllIndex(code, -1)
		quoteCursor := 0
		candidates := dynamicCalls(input.masked)
		for _, m := range callPattern.FindAllSubmatchIndex(code, -1) {
			pos := m[2]
			for quoteCursor < len(quoted) && quoted[quoteCursor][1] <= pos {
				quoteCursor++
			}
			if quoteCursor < len(quoted) && quoted[quoteCursor][0] < pos {
				continue
			}
			name := normalized(string(code[m[2]:m[3]]))
			if pos > 0 && (identifierByte(code[pos-1]) || code[pos-1] == '.' || code[pos-1] == '"') {
				continue
			}
			if syntaxWords[name] {
				continue
			}
			if code[m[4]] == ';' && !statementContext(input.masked, 0, pos) {
				continue
			}
			switch previousWord(code, pos) {
			case "END", "PROCEDURE", "FUNCTION", "PRAGMA", "INTO", "FROM", "UPDATE", "TABLE", "JOIN", "CURSOR", "TYPE":
				continue
			}

			candidates = append(candidates, call{pos: pos, text: name, raw: string(code[m[2]:m[3]])})
		}
		nodeByRoutine := map[*routine]*graphNode{}
		var localNodes []*graphNode
		for _, r := range input.routines {
			if !r.valid || !r.definition {
				continue
			}
			n := &graphNode{r: r}
			for _, e := range recognizedEffects(input.masked, r) {
				n.seed.effect = better(n.seed.effect, origin(e))
			}
			nodeByRoutine[r] = n
			localNodes = append(localNodes, n)
		}
		// Parent links avoid comparing every sibling pair.
		for _, child := range input.routines {
			if !child.valid || child.autonomous || !permissions[child.group] || !child.definition && allAutonomous(groups[child.group]) {
				continue
			}
			for r := child; r != nil; r = r.parent {
				if n := nodeByRoutine[r]; n != nil {
					n.seed.effect = better(n.seed.effect, capabilities[child.group])
				}
				if r.autonomous {
					break
				}
			}
		}
		// A source-order span sweep assigns and resolves each call once.
		slices.SortStableFunc(candidates, func(a, b call) int { return cmp.Compare(a.pos, b.pos) })
		var active []*routine
		headerCursor := 0
		for _, c := range candidates {
			for headerCursor < len(input.routines) && input.routines[headerCursor].start <= c.pos {
				r := input.routines[headerCursor]
				headerCursor++
				for len(active) > 0 && active[len(active)-1].end <= r.start {
					active = active[:len(active)-1]
				}
				active = append(active, r)
			}
			for len(active) > 0 && active[len(active)-1].end <= c.pos {
				active = active[:len(active)-1]
			}
			if len(active) == 0 {
				continue
			}
			owner := active[len(active)-1]
			if !owner.valid || !owner.definition || c.pos < owner.body {
				continue
			}
			if c.unknownReason == "" {
				c.target = resolveCall(owner, c.text, groups)
				if c.target == "" && dynamicBuiltins[c.text] {
					c.unknownReason = "dynamic execution"
				}
			}
			if c.target != "" {
				c.text = groups[c.target][0].name
			}
			for r := owner; r != nil; r = r.parent {
				if n := nodeByRoutine[r]; n != nil {
					if c.unknownReason != "" || c.target == "" && !safeBuiltins[c.text] {
						n.seed.unknown = better(n.seed.unknown, unknownOrigin(r, c))
					}
					n.calls = append(n.calls, c)
				}
				if r.autonomous {
					break
				}
			}
		}
		for _, n := range localNodes {
			nodes = append(nodes, *n)
		}

	}
	for key, rs := range groups {
		if !sourceGroups[key] {
			continue
		}
		hasBody, marked := false, false
		var seed *evidencePath
		for _, r := range rs {
			hasBody = hasBody || r.definition
			if !r.autonomous && permissions[key] {
				marked = true
				seed = better(seed, capabilities[key])
			}
		}
		// Definition-specific isolation is applied before overload aggregation.
		if !hasBody {
			if marked {
				summaries[key] = summary{effect: seed}
			} else if declarations[key].transaction != "safe" {
				r := rs[0]
				summaries[key] = summary{unknown: origin(Evidence{Location: location(r.file, r.start), Routine: r.name, Description: "declaration without body"})}
			}
		}
	}
	// Reverse edges notify only callers whose target summary has improved.
	type reverseEdge struct {
		r *routine
		c call
	}
	reverse := map[string][]reverseEdge{}
	for _, n := range nodes {
		if n.r.autonomous {
			continue
		}
		old := summaries[n.r.group]
		summaries[n.r.group] = summary{better(old.effect, n.seed.effect), better(old.unknown, n.seed.unknown)}
		for _, c := range n.calls {
			if c.target != "" {
				reverse[c.target] = append(reverse[c.target], reverseEdge{n.r, c})
			}
		}
	}
	queue := make([]string, 0, len(summaries))
	pending := map[string]bool{}
	for key, s := range summaries {
		if s.effect != nil || s.unknown != nil {
			queue = append(queue, key)
			pending[key] = true
		}
	}
	slices.Sort(queue)
	for head := 0; head < len(queue); head++ {
		key := queue[head]
		pending[key] = false
		target := summaries[key]
		for _, edge := range reverse[key] {
			group := edge.r.group
			old := summaries[group]
			next := summary{better(old.effect, prepend(edge.r, edge.c, target.effect)), better(old.unknown, prepend(edge.r, edge.c, target.unknown))}
			if next != old {
				summaries[group] = next
				if !pending[group] {
					pending[group] = true
					queue = append(queue, group)
				}
			}
		}
	}

	for key, declaration := range declarations {
		if declaration.transaction != "safe" || !sourceGroups[key] {
			continue
		}
		chain := better(summaries[key].effect, summaries[key].unknown)
		if chain != nil {
			report.Errors = append(report.Errors, AnalysisError{Diagnostic{
				ID: "CF1004", Severity: SeverityError, Kind: KindDeclarationConflict,
				Location: declaration.location, Routine: declaration.name, Origin: chain.origin,
				Evidence: materialize(chain),
				Message:  "safe transaction declaration contradicts outward transaction effects or unknown dependencies",
			}})
		}
	}
	for _, n := range nodes {
		for _, c := range n.calls {
			s := summaries[c.target]
			if c.unknownReason != "" || c.target == "" && !safeBuiltins[c.text] {
				s.unknown = unknownOrigin(n.r, c)
			}
			for _, unknown := range []bool{false, true} {
				chain := s.effect
				if unknown {
					chain = s.unknown
				}
				if chain == nil || !unknown && permissions[n.r.group] {
					continue
				}
				evidence := prepend(n.r, c, chain)
				if c.target == "" {
					evidence = chain
				}
				d := Diagnostic{ID: "CF0001", Severity: SeverityWarning, Kind: KindMissingPermission, Location: location(n.r.file, c.pos), Routine: n.r.name, Callee: c.text, Origin: chain.origin, Evidence: materialize(evidence), Message: "call may affect the transaction and requires permission"}
				if unknown {
					d.ID = "CF0002"
					d.Kind = KindUnknownEffect
					d.Message = "call has unknown transaction behavior"
					if c.target == "" {
						d.UnresolvedText = c.raw
					}
				}
				report.Findings = append(report.Findings, d)
			}
		}
	}
}

// Declarations inherit the isolation of their definitions. No body remains a
// capability assertion; any ordinary overload retains its outward effect.
func allAutonomous(rs []*routine) bool {
	found := false
	for _, r := range rs {
		if r.definition {
			found = true
			if !r.autonomous {
				return false
			}
		}
	}
	return found
}
