package analysis

import (
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

type externalEntry struct {
	name   string
	offset int
	kind   jsontext.Kind
}

func (entry *externalEntry) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	raw, err := dec.ReadValue()
	if err != nil {
		return err
	}
	entry.offset = int(dec.InputOffset()) - len(raw)
	entry.kind = raw.Kind()
	if entry.kind == '"' {
		return json.Unmarshal(raw, &entry.name)
	}
	return nil // Report wrong entry types without discarding the other entries.
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
		Version *int            `json:"version"`
		Safe    []externalEntry `json:"safe"`
		Unsafe  []externalEntry `json:"unsafe"`
	}
	// An omitted list stays empty. An explicit null decodes to nil.
	doc.Safe, doc.Unsafe = []externalEntry{}, []externalEntry{}
	if err := json.Unmarshal(data, &doc, json.RejectUnknownMembers(true)); err != nil {
		fail(0, "decode externals file: "+err.Error())
		return result
	}
	if doc.Version == nil || *doc.Version != 0 {
		fail(0, "externals file version must be 0")
		return result
	}
	lists := [...]struct {
		transaction string
		entries     []externalEntry
	}{
		{"safe", doc.Safe},
		{"unsafe", doc.Unsafe},
	}
	for _, list := range lists {
		if list.entries == nil {
			fail(0, "externals file field "+list.transaction+" must be an array")
			return result
		}
	}
	// Walk both lists in file order so a duplicate points to the later entry.
	if len(doc.Safe) > 0 && len(doc.Unsafe) > 0 && doc.Unsafe[0].offset < doc.Safe[0].offset {
		lists[0], lists[1] = lists[1], lists[0]
	}
	seen := map[string]bool{}
	for _, list := range lists {
		for i, entry := range list.entries {
			prefix := fmt.Sprintf("%s declaration %d: ", list.transaction, i+1)
			if entry.kind != '"' {
				fail(entry.offset, prefix+"name must be a string")
				continue
			}
			if !declarationName.MatchString(entry.name) {
				fail(entry.offset, prefix+"name must be SCHEMA.ROUTINE or SCHEMA.PACKAGE.ROUTINE")
				continue
			}
			key := normalized(entry.name)
			if seen[key] {
				fail(entry.offset, prefix+"duplicate normalized identity "+key)
				delete(result, key)
				continue
			}
			seen[key] = true
			result[key] = transactionDeclaration{key, list.transaction, location(f, entry.offset)}
		}
	}
	return result
}

func declarationEvidence(declaration transactionDeclaration) *evidencePath {
	return origin(Evidence{Location: declaration.location, Routine: declaration.name, Description: "external " + declaration.transaction + " declaration"})
}
