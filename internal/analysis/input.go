package analysis

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type sourceFile struct {
	path       string
	text       []byte
	lineStarts []int
}

func absolutePath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return absolute
}

// The location already identifies the file; do not repeat an absolute path in
// messages that the CLI can otherwise render relative to its working directory.
func fileError(err error) error {
	if pathError, ok := errors.AsType[*fs.PathError](err); ok {
		return pathError.Err
	}
	return err
}

func loadSources(paths, extensions []string, report *Report) []sourceFile {
	fail := func(path, operation string, err error) {
		report.Errors = append(report.Errors, AnalysisError{Diagnostic{
			ID: "CF1002", Severity: SeverityError, Kind: KindInput,
			Location: Location{File: absolutePath(path)},
			Message:  fmt.Sprintf("%s: %v", operation, fileError(err)),
		}})
	}
	if len(paths) == 0 {
		report.Errors = append(report.Errors, AnalysisError{Diagnostic{
			ID: "CF1001", Severity: SeverityError, Kind: KindInput,
			Message: "at least one source path is required",
		}})
		return nil
	}
	extensions, err := NormalizeExtensions(extensions)
	if err != nil {
		fail("", "select extensions", err)
		return nil
	}
	selected := make(map[string]bool, len(extensions))
	for _, extension := range extensions {
		selected["."+extension] = true
	}
	candidates := map[string]bool{}
	for _, path := range paths {
		if path == "" {
			fail(path, "select input", fmt.Errorf("empty path"))
			continue
		}
		root, err := filepath.EvalSymlinks(path)
		if err != nil {
			fail(path, "resolve input", err)
			continue
		}
		root = absolutePath(root)
		info, err := os.Stat(root)
		if err != nil {
			fail(root, "inspect input", err)
			continue
		}
		if !info.IsDir() {
			candidates[root] = true
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				fail(path, "enumerate input", walkErr)
				return nil
			}
			// Do not follow links discovered during recursion, including directory links.
			if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
				return nil
			}
			if selected[strings.ToLower(filepath.Ext(path))] {
				candidates[path] = true
			}
			return nil
		})
	}
	var files []sourceFile
	var identities []fs.FileInfo
	var totalBytes int64
	for _, path := range slices.Sorted(maps.Keys(candidates)) {
		info, err := os.Stat(path)
		if err != nil {
			fail(path, "inspect source", err)
			continue
		}
		if !info.Mode().IsRegular() {
			fail(path, "read source", fmt.Errorf("not a regular file"))
			continue
		}
		duplicate := false
		for _, previous := range identities {
			if os.SameFile(info, previous) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		content, err := readSource(path, MaxTotalSourceBytes-totalBytes)
		if err != nil {
			fail(path, "read source", err)
			continue
		}
		totalBytes += int64(len(content))
		identities = append(identities, info)
		files = append(files, sourceFile{path: path, text: content})
		report.Files = append(report.Files, path)
	}
	return files
}

// Source limits bound retained input memory independently of file metadata.
const (
	MaxSourceBytes      int64 = 16 << 20
	MaxTotalSourceBytes int64 = 128 << 20
)

func readSource(path string, remaining int64) ([]byte, error) {
	limit := min(MaxSourceBytes, remaining)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("source byte limit exceeded (per file %d, remaining aggregate %d)", MaxSourceBytes, remaining)
	}
	return content, nil
}
