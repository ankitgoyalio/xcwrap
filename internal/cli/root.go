package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	outputJSON     = "json"
	outputTable    = "table"
	outputMarkdown = "markdown"
)

type runContext struct {
	stdout io.Writer
	stderr io.Writer

	output string
}

func newRootCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	ctx := &runContext{
		stdout: stdout,
		stderr: stderr,
		output: defaultOutput(),
	}

	cmd := &cobra.Command{
		Use:           "xcwrap",
		Short:         "Fast, lightweight CLI for Xcode .xc* workflows",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.PersistentFlags().StringVar(&ctx.output, "output", ctx.output, "Output format: json|table|markdown")
	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if !isAllowedOutput(ctx.output) {
			return usageError{
				Message: fmt.Sprintf("invalid value for --output: %q (allowed: json, table, markdown)", ctx.output),
			}
		}
		return nil
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{Message: err.Error()}
	})

	cmd.AddCommand(newAssetsCommand(ctx))

	return cmd
}

func defaultOutput() string {
	v, ok := os.LookupEnv("XCWRAP_DEFAULT_OUTPUT")
	if !ok || strings.TrimSpace(v) == "" {
		return outputJSON
	}
	normalized := strings.ToLower(strings.TrimSpace(v))
	if !isAllowedOutput(normalized) {
		return outputJSON
	}
	return normalized
}

func isAllowedOutput(v string) bool {
	switch v {
	case outputJSON, outputTable, outputMarkdown:
		return true
	default:
		return false
	}
}
