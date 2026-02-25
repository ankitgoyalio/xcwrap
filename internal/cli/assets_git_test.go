package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRequireCleanGitWorkingTree_IncludesGitDiagnosticWhenCheckFails(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

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

func TestDeletePruneTargets_RejectsSymlinkTargetOutsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on windows")
	}

	root := t.TempDir()
	catalogRoot := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(catalogRoot, 0o755); err != nil {
		t.Fatalf("mkdir catalog root: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.imageset")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside target: %v", err)
	}

	linkPath := filepath.Join(catalogRoot, "linked.imageset")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("create symlink target: %v", err)
	}

	err := deletePruneTargets(root, []string{linkPath})
	if err == nil {
		t.Fatalf("expected symlink escape to be rejected")
	}
	if !strings.Contains(err.Error(), "outside root") {
		t.Fatalf("expected outside-root rejection, got %v", err)
	}
	if _, statErr := os.Lstat(linkPath); statErr != nil {
		t.Fatalf("expected symlink to remain after rejection, stat err=%v", statErr)
	}
}

func TestDeletePruneTargets_AllowsSymlinkedCatalogRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on windows")
	}

	root := t.TempDir()
	realCatalogRoot := filepath.Join(root, "real-catalog-root")
	if err := os.MkdirAll(realCatalogRoot, 0o755); err != nil {
		t.Fatalf("mkdir real catalog root: %v", err)
	}
	symlinkCatalogRoot := filepath.Join(root, "LinkedAssets.xcassets")
	if err := os.Symlink(realCatalogRoot, symlinkCatalogRoot); err != nil {
		t.Fatalf("create catalog root symlink: %v", err)
	}

	target := filepath.Join(symlinkCatalogRoot, "unused.imageset")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir prune target: %v", err)
	}

	if err := deletePruneTargets(symlinkCatalogRoot, []string{target}); err != nil {
		t.Fatalf("delete prune targets: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected prune target to be removed, stat err=%v", err)
	}
}
