package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssetsScan_DefaultJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute([]string{"assets", "scan"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v", err)
	}
	if payload["command"] != "assets scan" {
		t.Fatalf("unexpected command value: %v", payload["command"])
	}
}

func TestInvalidOutputValue_ReturnsUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute([]string{"--output", "xml", "assets", "scan"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %s", stdout.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON error output, got err: %v, stderr=%s", err, stderr.String())
	}
	errVal, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object: %v", payload)
	}
	if errVal["code"] != "usage_error" {
		t.Fatalf("unexpected error code: %v", errVal["code"])
	}
}

func TestAssetsPrune_ForceWithoutApply_IsUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute([]string{"assets", "prune", "--force"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %s", stdout.String())
	}
}

func TestAssetsScan_ExpandsTildePath(t *testing.T) {
	homeDir := t.TempDir()
	projectDir := filepath.Join(homeDir, "Developer", "fsm-ios")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	prevHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("HOME", prevHome)
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute([]string{"assets", "scan", "--path", "~/Developer/fsm-ios"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v", err)
	}
	if payload["path"] != projectDir {
		t.Fatalf("expected expanded path %q, got %v", projectDir, payload["path"])
	}
}

func TestAssetsScan_InvalidPath_ReturnsRuntimeError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute([]string{"assets", "scan", "--path", "/tmp/xcwrap-path-that-should-not-exist-12345"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %s", stdout.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON error output, got err: %v", err)
	}
	errVal, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object: %v", payload)
	}
	if errVal["code"] != "runtime_error" {
		t.Fatalf("unexpected error code: %v", errVal["code"])
	}
}

func TestAssetsUnused_ReturnsExitCode3WhenUnusedFound(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "used.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "unused.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir unused asset set: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main.swift"), []byte(`let _ = UIImage(named: "used")`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "unused", "--path", root}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v", err)
	}
	unused, ok := payload["unused"].([]any)
	if !ok || len(unused) != 1 || unused[0] != "unused" {
		t.Fatalf("unexpected unused payload: %#v", payload["unused"])
	}
	grouped, ok := payload["unusedByFile"].(map[string]any)
	if !ok || len(grouped) != 1 {
		t.Fatalf("unexpected unusedByFile payload: %#v", payload["unusedByFile"])
	}
}

func TestAssetsScan_TableOutput_IsNotJSON(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "used.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main.swift"), []byte(`let _ = UIImage(named: "used")`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"--output", "table", "assets", "scan", "--path", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	out := stdout.String()
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("expected table output, got JSON: %s", out)
	}
	if !strings.Contains(out, "command\tpath\tworkers\tasset_catalogs") {
		t.Fatalf("unexpected table output: %s", out)
	}
}

func TestAssetsScan_DefaultExcludesExternalLibraries(t *testing.T) {
	root := t.TempDir()
	podAssetDir := filepath.Join(root, "Pods", "SomeLib", "Assets.xcassets", "ic_unassigned_2_28.imageset")
	if err := os.MkdirAll(podAssetDir, 0o755); err != nil {
		t.Fatalf("mkdir pod asset set: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "scan", "--path", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v", err)
	}
	summary, ok := payload["summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing summary payload: %#v", payload)
	}
	if summary["assetSets"] != float64(0) {
		t.Fatalf("expected assetSets=0 when only Pods assets exist, got %v", summary["assetSets"])
	}
}

func TestAssetsScan_ExcludeFlag_AllowsAdditionalExcludes(t *testing.T) {
	root := t.TempDir()
	assetDir := filepath.Join(root, "ExternalLib", "Assets.xcassets", "externalIcon.imageset")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("mkdir external asset set: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "scan", "--path", root, "--exclude", "ExternalLib/"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v", err)
	}
	summary, ok := payload["summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing summary payload: %#v", payload)
	}
	if summary["assetSets"] != float64(0) {
		t.Fatalf("expected assetSets=0 with explicit --exclude, got %v", summary["assetSets"])
	}
}
