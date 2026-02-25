package cli

import (
	"strings"
	"testing"
)

func TestRequireCleanGitWorkingTree_IncludesGitDiagnosticWhenCheckFails(t *testing.T) {
	root := t.TempDir()

	err := requireCleanGitWorkingTree(root)
	if err == nil {
		t.Fatalf("expected git working tree check to fail for non-repo directory")
	}

	message := err.Error()
	if !strings.Contains(message, "failed to check git working tree") {
		t.Fatalf("expected wrapped check failure message, got %q", message)
	}
	if !strings.Contains(message, "not a git repository") {
		t.Fatalf("expected git stderr diagnostic in message, got %q", message)
	}
}
