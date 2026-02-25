package cli

import "testing"

func TestDefaultOutput_UsesAllowedEnvValue(t *testing.T) {
	t.Setenv("XCWRAP_DEFAULT_OUTPUT", " Table ")

	if got := defaultOutput(); got != outputTable {
		t.Fatalf("expected %q, got %q", outputTable, got)
	}
}

func TestDefaultOutput_FallsBackToJSONForInvalidEnvValue(t *testing.T) {
	t.Setenv("XCWRAP_DEFAULT_OUTPUT", "xml")

	if got := defaultOutput(); got != outputJSON {
		t.Fatalf("expected fallback %q, got %q", outputJSON, got)
	}
}
