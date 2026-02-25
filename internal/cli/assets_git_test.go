package cli

import (
	"os"
	"path/filepath"
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

func TestDeletePruneTargets_AllowsCatalogRootAsPruneRoot(t *testing.T) {
	root := t.TempDir()
	catalogRoot := filepath.Join(root, "Assets.xcassets")
	target := filepath.Join(catalogRoot, "unused.imageset")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir prune target: %v", err)
	}

	if err := deletePruneTargets(catalogRoot, []string{target}); err != nil {
		t.Fatalf("delete prune targets: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected prune target to be removed, stat err=%v", err)
	}
}
