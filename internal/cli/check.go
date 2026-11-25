package cli

import (
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/DLakomy/commitfence/internal/analysis"
)

func newCheckCommand(stderr io.Writer, code *int) *cobra.Command {
	overrides := defaultConfig()
	var configFile string
	command := &cobra.Command{
		Use:   "check [PATH...]",
		Short: "Analyze source files and directories",
		Long: "Analyze source files and directories for PL/SQL transaction behavior.\n" +
			"Load .commitfence.json from the current directory, or select a file with --config.\n" +
			"Without source paths, use configured paths or the current directory.\n" +
			"Flags may precede or follow source paths. Use -- before paths beginning with -.",
		RunE: func(command *cobra.Command, args []string) error {
			if configFile == "" {
				return errors.New("--config must not be empty")
			}
			if err := overrides.validate(); err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			flags := command.Flags()
			config, failure := loadConfig(configFile, flags.Changed("config"))
			if failure != nil {
				newRenderer(stderr, cwd, overrides.PathStyle, overrides.Color).render(*failure)
				*code = 2
				return nil
			}
			if flags.Changed("externals") {
				config.Externals = overrides.Externals
			}
			if flags.Changed("extensions") {
				config.Extensions = overrides.Extensions
			}
			if flags.Changed("path-style") {
				config.PathStyle = overrides.PathStyle
			}
			if flags.Changed("color") {
				config.Color = overrides.Color
			}
			if len(args) > 0 {
				config.Paths = args
			} else if config.Paths == nil {
				config.Paths = []string{"."}
			}
			output := newRenderer(stderr, cwd, config.PathStyle, config.Color)
			*code = runCheck(analysis.Options{
				Paths: config.Paths, ExternalsPath: config.Externals, Extensions: config.Extensions,
			}, output)
			return nil
		},
	}
	command.Flags().StringVar(&configFile, "config", ".commitfence.json", "JSON application config file")
	command.Flags().StringVar(&overrides.Externals, "externals", "", "JSON file for external transaction declarations (empty disables declarations)")
	command.Flags().StringSliceVar(&overrides.Extensions, "extensions", overrides.Extensions, "file extensions selected in directory scans")
	command.Flags().StringVar(&overrides.PathStyle, "path-style", "relative", "reported paths: relative or absolute")
	command.Flags().StringVar(&overrides.Color, "color", "auto", "diagnostic colors: auto, always, or never")
	return command
}

func runCheck(options analysis.Options, output renderer) int {
	report := analysis.Check(options)
	hasError, hasWarning := len(report.Errors) > 0, false
	for _, diagnostic := range report.Findings {
		output.render(diagnostic)
		hasError = hasError || diagnostic.Severity == analysis.SeverityError
		hasWarning = hasWarning || diagnostic.Severity == analysis.SeverityWarning
	}
	for _, failure := range report.Errors {
		output.render(failure.Diagnostic)
	}
	if hasError {
		return 2
	}
	if hasWarning {
		return 1
	}
	return 0
}
