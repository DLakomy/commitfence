package analysis

import (
	"fmt"
	"strings"
	"unicode"
)

// DefaultExtensions returns a fresh list of extensions selected in directory scans.
func DefaultExtensions() []string {
	return []string{"sql", "pks", "pkb", "pls", "tps", "tpb"}
}

// NormalizeExtensions applies the defaults to nil, removes optional leading dots,
// folds case, and deduplicates extensions. An explicit list must be nonempty and
// contain simple suffixes, without whitespace, path separators, or glob syntax.
func NormalizeExtensions(extensions []string) ([]string, error) {
	if extensions == nil {
		return DefaultExtensions(), nil
	}
	if len(extensions) == 0 {
		return nil, fmt.Errorf("extensions must not be empty")
	}
	var normalized []string
	seen := make(map[string]bool)
	for _, extension := range extensions {
		ext := strings.ToLower(strings.TrimPrefix(extension, "."))
		invalid := strings.ContainsFunc(ext, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("./\\*?[]{}", r)
		})
		if ext == "" || invalid {
			return nil, fmt.Errorf("invalid extension %q: expected a simple file suffix", extension)
		}
		if !seen[ext] {
			seen[ext] = true
			normalized = append(normalized, ext)
		}
	}
	return normalized, nil
}
