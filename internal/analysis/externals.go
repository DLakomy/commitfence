package analysis

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"os"
	"regexp"
)

type transactionDeclaration struct {
	name, transaction string
	location          Location
}

func (d transactionDeclaration) grantsPermission() bool {
	return d.transaction == "unsafe"
}

var declarationName = regexp.MustCompile("^(?:" + ident + ")\\s*\\.\\s*(?:" + ident + ")(?:\\s*\\.\\s*(?:" + ident + "))?$")

const maxExternalsBytes = 16 << 20 // 16 MiB, before decoding or indexing.

func loadDeclarations(path string, report *Report) map[string]transactionDeclaration {
	result := map[string]transactionDeclaration{}
	if path == "" {
		return result
	}
	f := sourceFile{path: absolutePath(path), lineStarts: []int{0}}
	fail := func(pos int, message string) {
		report.Errors = append(report.Errors, AnalysisError{Diagnostic{ID: "CF1003", Severity: SeverityError, Kind: KindConfiguration, Location: location(f, pos), Message: message}})
	}
	info, err := os.Stat(f.path)
	if err != nil {
		fail(0, "inspect externals file: "+fileError(err).Error())
		return result
	}
	if !info.Mode().IsRegular() {
		fail(0, "externals file must be a regular file")
		return result
	}
	file, err := os.Open(f.path)
	if err != nil {
		fail(0, "read externals file: "+fileError(err).Error())
		return result
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxExternalsBytes+1))
	if err != nil {
		fail(0, "read externals file: "+fileError(err).Error())
		return result
	}
	if len(data) > maxExternalsBytes {
		fail(0, "externals file exceeds 16 MiB limit")
		return result
	}
	f.text = data
	for i, b := range data {
		if b == '\n' {
			f.lineStarts = append(f.lineStarts, i+1)
		}
	}
	var doc struct {
		Version  int              `json:"version"`
		Routines []jsontext.Value `json:"routines"`
	}
	if err := json.Unmarshal(data, &doc, json.RejectUnknownMembers(true)); err != nil {
		fail(0, "decode externals file: "+err.Error())
		return result
	}
	if doc.Version != 1 {
		fail(0, "externals file version must be 1")
		return result
	}
	if doc.Routines == nil {
		fail(0, "externals file field routines must be an array")
		return result
	}
	cursor := 0
	seen := map[string]bool{}
	for i, raw := range doc.Routines {
		pos := cursor + bytes.Index(data[cursor:], raw)
		cursor = pos + len(raw)
		var entry struct {
			Name        string `json:"name"`
			Transaction string `json:"transaction"`
		}
		err := json.Unmarshal(raw, &entry, json.RejectUnknownMembers(true))
		prefix := fmt.Sprintf("transaction declaration %d: ", i+1)
		if err != nil {
			fail(pos, prefix+err.Error())
			continue
		}
		if !declarationName.MatchString(entry.Name) {
			fail(pos, prefix+"name must be SCHEMA.ROUTINE or SCHEMA.PACKAGE.ROUTINE")
			continue
		}
		key := normalized(entry.Name)
		if seen[key] {
			fail(pos, prefix+"duplicate normalized identity "+key)
			delete(result, key)
			continue
		}
		seen[key] = true
		if entry.Transaction != "safe" && entry.Transaction != "unsafe" {
			fail(pos, prefix+"transaction must be safe or unsafe")
			continue
		}
		result[key] = transactionDeclaration{key, entry.Transaction, location(f, pos)}
	}
	return result
}

func declarationEvidence(declaration transactionDeclaration) *evidencePath {
	return origin(Evidence{Location: declaration.location, Routine: declaration.name, Description: "external " + declaration.transaction + " declaration"})
}
