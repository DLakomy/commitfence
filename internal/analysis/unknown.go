package analysis

import (
	"regexp"
	"strings"
)

var nativeDynamic = regexp.MustCompile(`(?i)\bEXECUTE\s+IMMEDIATE\b|\bOPEN\s+` + qualified + `\s+FOR\b`)
var staticQuery = regexp.MustCompile(`(?i)^\s*(SELECT|WITH)\b`)

// Literal contents are masked: native dynamic execution never interprets SQL.
// OPEN FOR SELECT/WITH is a static cursor query, not dynamic execution.
func dynamicCalls(code []byte) []call {
	var calls []call
	for _, m := range nativeDynamic.FindAllIndex(code, -1) {
		if !statementContext(code, 0, m[0]) {
			continue
		}
		text := "EXECUTE IMMEDIATE"
		if strings.EqualFold(string(code[m[0]:m[0]+4]), "OPEN") {
			if staticQuery.Match(code[m[1]:]) {
				continue
			}
			text = "OPEN FOR"
		}
		calls = append(calls, call{pos: m[0], text: text, raw: string(code[m[0]:m[1]]), unknownReason: "dynamic execution"})
	}
	return calls
}

func unknownOrigin(r *routine, c call) *evidencePath {
	reason := c.unknownReason
	if reason == "" {
		reason = "unresolved call"
	}
	return origin(Evidence{Location: location(r.file, c.pos), Routine: r.name, Callee: c.text, Description: reason})
}
