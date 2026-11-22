package cli

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"commitfence/internal/analysis"
)

type checkConfig struct {
	Version    int      `json:"version"`
	Paths      []string `json:"paths"`
	Extensions []string `json:"extensions"`
	PathStyle  string   `json:"path_style"`
	Color      string   `json:"color"`
	Externals  string   `json:"externals"`
}

func defaultConfig() checkConfig {
	return checkConfig{
		Extensions: analysis.DefaultExtensions(),
		PathStyle:  "relative",
		Color:      "auto",
	}
}

func (config *checkConfig) validate() error {
	if config.Paths != nil && len(config.Paths) == 0 {
		return errors.New("paths must not be empty")
	}
	if slices.Contains(config.Paths, "") {
		return errors.New("source path must not be empty")
	}
	if config.PathStyle != "relative" && config.PathStyle != "absolute" {
		return errors.New("path_style must be relative or absolute")
	}
	if config.Color != "auto" && config.Color != "always" && config.Color != "never" {
		return errors.New("color must be auto, always, or never")
	}
	extensions, err := analysis.NormalizeExtensions(config.Extensions)
	if err != nil {
		return err
	}
	config.Extensions = extensions
	return nil
}

// Apply the same size limit to project settings and externals files.
const maxConfigBytes = 16 << 20

func loadConfig(path string, explicit bool) (checkConfig, *analysis.Diagnostic) {
	config := defaultConfig()
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	fail := func(err error) (checkConfig, *analysis.Diagnostic) {
		if pathError, ok := errors.AsType[*fs.PathError](err); ok {
			err = pathError.Err
		}
		return config, &analysis.Diagnostic{
			ID: "CF1003", Severity: analysis.SeverityError, Kind: analysis.KindConfiguration,
			Location: analysis.Location{File: path}, Message: "application configuration: " + err.Error(),
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return config, nil
		}
		return fail(err)
	}
	if !info.Mode().IsRegular() {
		return fail(errors.New("must be a regular file"))
	}
	file, err := os.Open(path)
	if err != nil {
		return fail(err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return fail(err)
	}
	if len(data) > maxConfigBytes {
		return fail(errors.New("exceeds 16 MiB limit"))
	}
	if err := json.Unmarshal(data, &config, json.RejectUnknownMembers(true)); err != nil {
		return fail(err)
	}
	if config.Version != 1 {
		return fail(errors.New("version must be 1"))
	}
	// Null must not silently clear defaults or act like an omitted field.
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(data, &fields); err != nil {
		return fail(err)
	}
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		if bytes.Equal(fields[name], []byte("null")) {
			return fail(fmt.Errorf("%s must not be null", name))
		}
	}
	if err := config.validate(); err != nil {
		return fail(err)
	}
	base := filepath.Dir(path)
	for i, source := range config.Paths {
		config.Paths[i] = configPath(base, source)
	}
	if config.Externals != "" {
		config.Externals = configPath(base, config.Externals)
	}
	return config, nil
}

func configPath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	// Preserve symlink/../ semantics; filepath.Join would clean them prematurely.
	return base + string(filepath.Separator) + path
}
