package cli

import (
	"errors"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	exitSuccess      = 0
	exitFailure      = 1
	exitUsage        = 2
	exitUnusedAssets = 3
)

type usageError struct {
	Message string `json:"message"`
}

func (e usageError) Error() string {
	return e.Message
}

type unusedAssetsFoundError struct{}

func (e unusedAssetsFoundError) Error() string {
	return "unused assets detected"
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Execute(args []string, stdout io.Writer, stderr io.Writer) int {
	root := newRootCommand(stdout, stderr)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		if isUsageExecutionError(err) {
			writeError(stderr, "usage_error", err.Error())
			return exitUsage
		}
		var unusedErr unusedAssetsFoundError
		if errors.As(err, &unusedErr) {
			return exitUnusedAssets
		}

		writeError(stderr, "runtime_error", err.Error())
		return exitFailure
	}

	return exitSuccess
}

func isUsageExecutionError(err error) bool {
	var usageErr usageError
	if errors.As(err, &usageErr) {
		return true
	}

	message := strings.ToLower(err.Error())
	usageIndicators := []string{
		"unknown command",
		"unknown flag",
		"unknown shorthand flag",
		"flag needs an argument",
		"required flag",
		"accepts ",
		"requires at least",
		"requires at most",
		"requires exactly",
	}
	for _, indicator := range usageIndicators {
		if strings.Contains(message, indicator) {
			return true
		}
	}

	return false
}

func writeJSON(w io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(w, string(payload))
	return err
}

func writeError(w io.Writer, code, message string) {
	_ = writeJSON(w, errorEnvelope{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	})
}
