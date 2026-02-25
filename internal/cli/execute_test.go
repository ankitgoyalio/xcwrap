package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAssetsScan_DefaultJSONOutput(t *testing.T) {
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

	exitCode := Execute([]string{"assets", "scan", "--path", root}, &stdout, &stderr)
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

func TestUnknownSubcommand_ReturnsUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Execute([]string{"does-not-exist"}, &stdout, &stderr)
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

	t.Setenv("HOME", homeDir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", homeDir)
	}

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

	invalidPath := filepath.Join(t.TempDir(), "xcwrap-path-that-should-not-exist")
	exitCode := Execute([]string{"assets", "scan", "--path", invalidPath}, &stdout, &stderr)
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
	if payload["unusedCount"] != float64(1) {
		t.Fatalf("expected unusedCount=1, got %v", payload["unusedCount"])
	}
	if payload["pruneCandidateCount"] != float64(1) {
		t.Fatalf("expected pruneCandidateCount=1, got %v", payload["pruneCandidateCount"])
	}
	grouped, ok := payload["unusedByFile"].(map[string]any)
	if !ok || len(grouped) != 1 {
		t.Fatalf("unexpected unusedByFile payload: %#v", payload["unusedByFile"])
	}
	for key, raw := range grouped {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("unexpected grouped entry type for %s: %#v", key, raw)
		}
		if _, hasFilePath := entry["filePath"]; hasFilePath {
			t.Fatalf("filePath should not be present in unusedByFile entry: %#v", entry)
		}
	}
}

func TestAssetsUnused_LowConfidenceTokenAndStringDoNotSuppressUnused(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "ghost.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}
	source := `// ghost appears only as comment/token/string
let ghost = "ghost"
`
	if err := os.WriteFile(filepath.Join(root, "Main.swift"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "unused", "--path", root}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v", err)
	}
	unused, ok := payload["unused"].([]any)
	if !ok || len(unused) != 1 || unused[0] != "ghost" {
		t.Fatalf("unexpected unused payload: %#v", payload["unused"])
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
	if !strings.Contains(out, "command") || !strings.Contains(out, "asset_catalogs") {
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

func TestAssetsScan_ExcludeFlag_CommaSeparatedValuesExcludeMultiplePaths(t *testing.T) {
	root := t.TempDir()
	externalAssetDir := filepath.Join(root, "ExternalLib", "Assets.xcassets", "externalIcon.imageset")
	vendorAssetDir := filepath.Join(root, "vendor", "SomeLib", "Assets.xcassets", "vendorIcon.imageset")
	if err := os.MkdirAll(externalAssetDir, 0o755); err != nil {
		t.Fatalf("mkdir external asset set: %v", err)
	}
	if err := os.MkdirAll(vendorAssetDir, 0o755); err != nil {
		t.Fatalf("mkdir vendor asset set: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "scan", "--path", root, "--exclude", "ExternalLib/,vendor/"}, &stdout, &stderr)
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
		t.Fatalf("expected assetSets=0 with comma-separated --exclude, got %v", summary["assetSets"])
	}
}

func TestAssetsScan_InvalidIncludeGlobReturnsUsageError(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "used.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "scan", "--path", root, "--include", "["}, &stdout, &stderr)
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
	message, _ := errVal["message"].(string)
	if !strings.Contains(message, "invalid value for --include") {
		t.Fatalf("expected include validation message, got: %q", message)
	}
}

func TestAssetsScan_InvalidExcludeGlobReturnsUsageError(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "used.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "scan", "--path", root, "--exclude", "["}, &stdout, &stderr)
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
	message, _ := errVal["message"].(string)
	if !strings.Contains(message, "invalid value for --exclude") {
		t.Fatalf("expected exclude validation message, got: %q", message)
	}
}

func TestAssetsScan_ReadErrorReturnsRuntimeError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission model differs on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("permission test is unreliable when running as root")
	}

	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "used.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "Main.swift"), []byte(`let _ = UIImage(named: "used")`), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	unreadable := filepath.Join(root, "Unreadable.swift")
	if err := os.WriteFile(unreadable, []byte("let value = 1"), 0o644); err != nil {
		t.Fatalf("write unreadable file: %v", err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod unreadable file: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unreadable, 0o644)
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- Execute([]string{"assets", "scan", "--path", root}, &stdout, &stderr)
	}()

	timeout := 10 * time.Second
	if testing.Short() {
		timeout = 3 * time.Second
	}

	select {
	case exitCode := <-done:
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	case <-time.After(timeout):
		t.Fatal("execute deadlocked after file read error")
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
	if errVal["code"] != "runtime_error" {
		t.Fatalf("unexpected error code: %v", errVal["code"])
	}
}

func TestAssetsUnused_DuplicateNamesAcrossCatalogs(t *testing.T) {
	root := t.TempDir()

	moduleACatalog := filepath.Join(root, "Modules", "ModuleA", "Assets.xcassets")
	moduleBCatalog := filepath.Join(root, "Modules", "ModuleB", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(moduleACatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module a asset: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(moduleBCatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module b asset: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Modules", "ModuleA", "Feature.swift"), []byte(`let _ = UIImage(named: "icon")`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "unused", "--path", root}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v, stdout=%s", err, stdout.String())
	}
	unused, ok := payload["unused"].([]any)
	if !ok || len(unused) != 1 || unused[0] != "icon" {
		t.Fatalf("unexpected unused payload: %#v", payload["unused"])
	}
	grouped, ok := payload["unusedByFile"].(map[string]any)
	if !ok || len(grouped) != 1 {
		t.Fatalf("expected one grouped unused catalog, got %#v", payload["unusedByFile"])
	}
	if _, exists := grouped[moduleBCatalog]; !exists {
		t.Fatalf("expected grouped key %s, got %#v", moduleBCatalog, grouped)
	}
}

func TestAssetsUnused_JSONGroupingDistinctCatalogsWithSameBasename(t *testing.T) {
	root := t.TempDir()

	moduleACatalog := filepath.Join(root, "Modules", "ModuleA", "Assets.xcassets")
	moduleBCatalog := filepath.Join(root, "Modules", "ModuleB", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(moduleACatalog, "a.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module a asset: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(moduleBCatalog, "b.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module b asset: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "unused", "--path", root}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v, stdout=%s", err, stdout.String())
	}
	grouped, ok := payload["unusedByFile"].(map[string]any)
	if !ok || len(grouped) != 2 {
		t.Fatalf("expected two grouped unused catalogs, got %#v", payload["unusedByFile"])
	}
	if _, exists := grouped[moduleACatalog]; !exists {
		t.Fatalf("expected grouped key %s, got %#v", moduleACatalog, grouped)
	}
	if _, exists := grouped[moduleBCatalog]; !exists {
		t.Fatalf("expected grouped key %s, got %#v", moduleBCatalog, grouped)
	}
}

func TestAssetsUnused_ExplicitCountsWhenPruneCandidatesExceedUnusedNames(t *testing.T) {
	root := t.TempDir()

	moduleACatalog := filepath.Join(root, "Modules", "ModuleA", "Assets.xcassets")
	moduleBCatalog := filepath.Join(root, "Modules", "ModuleB", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(moduleACatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module a asset: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(moduleBCatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module b asset: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "unused", "--path", root}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v, stdout=%s", err, stdout.String())
	}
	if payload["unusedCount"] != float64(1) {
		t.Fatalf("expected unusedCount=1, got %v", payload["unusedCount"])
	}
	if payload["pruneCandidateCount"] != float64(2) {
		t.Fatalf("expected pruneCandidateCount=2, got %v", payload["pruneCandidateCount"])
	}
}

func TestAssetsUnused_InvalidExcludeGlobReturnsUsageError(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "unused.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "unused", "--path", root, "--exclude", "["}, &stdout, &stderr)
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
	message, _ := errVal["message"].(string)
	if !strings.Contains(message, "invalid value for --exclude") {
		t.Fatalf("expected exclude validation message, got: %q", message)
	}
}

func TestAssetsUnused_ExcludeFlag_CommaSeparatedValuesExcludeMultiplePaths(t *testing.T) {
	root := t.TempDir()
	externalAssetDir := filepath.Join(root, "ExternalLib", "Assets.xcassets", "externalIcon.imageset")
	vendorAssetDir := filepath.Join(root, "vendor", "SomeLib", "Assets.xcassets", "vendorIcon.imageset")
	if err := os.MkdirAll(externalAssetDir, 0o755); err != nil {
		t.Fatalf("mkdir external asset set: %v", err)
	}
	if err := os.MkdirAll(vendorAssetDir, 0o755); err != nil {
		t.Fatalf("mkdir vendor asset set: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "unused", "--path", root, "--exclude", "ExternalLib/,vendor/"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v, stdout=%s", err, stdout.String())
	}
	if payload["unusedCount"] != float64(0) {
		t.Fatalf("expected unusedCount=0 with comma-separated --exclude, got %v", payload["unusedCount"])
	}
	if payload["pruneCandidateCount"] != float64(0) {
		t.Fatalf("expected pruneCandidateCount=0 with comma-separated --exclude, got %v", payload["pruneCandidateCount"])
	}
}

func TestAssetsUnused_UnusedByFilePreservesTypeDistinctNames(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "logo.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "logo.colorset"), 0o755); err != nil {
		t.Fatalf("mkdir color asset set: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "unused", "--path", root}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v, stdout=%s", err, stdout.String())
	}
	grouped, ok := payload["unusedByFile"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected unusedByFile payload: %#v", payload["unusedByFile"])
	}
	rawEntry, ok := grouped[catalog]
	if !ok {
		t.Fatalf("expected catalog %q in unusedByFile payload, got %#v", catalog, grouped)
	}
	entry, ok := rawEntry.(map[string]any)
	if !ok {
		t.Fatalf("unexpected entry type for catalog %s: %#v", catalog, rawEntry)
	}
	unusedAssets, ok := entry["unusedAssets"].([]any)
	if !ok {
		t.Fatalf("unexpected unusedAssets payload: %#v", entry["unusedAssets"])
	}
	if len(unusedAssets) != 2 || unusedAssets[0] != "logo.colorset" || unusedAssets[1] != "logo.imageset" {
		t.Fatalf("expected type-distinct unusedAssets, got %#v", unusedAssets)
	}
}

func TestAssetsPrune_DryRunReportsCandidatesWithoutDeleting(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	usedPath := filepath.Join(catalog, "used.imageset")
	unusedPath := filepath.Join(catalog, "unused.imageset")
	if err := os.MkdirAll(usedPath, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}
	if err := os.MkdirAll(unusedPath, 0o755); err != nil {
		t.Fatalf("mkdir unused asset set: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main.swift"), []byte(`let _ = UIImage(named: "used")`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "prune", "--path", root}, &stdout, &stderr)
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
	if payload["dryRun"] != true {
		t.Fatalf("expected dryRun=true, got %v", payload["dryRun"])
	}
	if payload["unusedCount"] != float64(1) {
		t.Fatalf("expected unusedCount=1, got %v", payload["unusedCount"])
	}
	if payload["pruneCandidateCount"] != float64(1) {
		t.Fatalf("expected pruneCandidateCount=1, got %v", payload["pruneCandidateCount"])
	}
	deleted, ok := payload["deleted"].([]any)
	if !ok || len(deleted) != 1 || deleted[0] != unusedPath {
		t.Fatalf("unexpected deleted payload: %#v", payload["deleted"])
	}

	if _, err := os.Stat(unusedPath); err != nil {
		t.Fatalf("expected dry-run to keep %s, stat err=%v", unusedPath, err)
	}
}

func TestAssetsPrune_ExplicitCountsWhenPruneCandidatesExceedUnusedNames(t *testing.T) {
	root := t.TempDir()

	moduleACatalog := filepath.Join(root, "Modules", "ModuleA", "Assets.xcassets")
	moduleBCatalog := filepath.Join(root, "Modules", "ModuleB", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(moduleACatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module a asset: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(moduleBCatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module b asset: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "prune", "--path", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON output, got err: %v, stdout=%s", err, stdout.String())
	}
	if payload["unusedCount"] != float64(1) {
		t.Fatalf("expected unusedCount=1, got %v", payload["unusedCount"])
	}
	if payload["pruneCandidateCount"] != float64(2) {
		t.Fatalf("expected pruneCandidateCount=2, got %v", payload["pruneCandidateCount"])
	}
}

func TestAssetsPrune_ApplyDeletesUnusedAssetsWhenGitTreeClean(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	usedPath := filepath.Join(catalog, "used.imageset")
	unusedA := filepath.Join(catalog, "a.imageset")
	unusedB := filepath.Join(catalog, "z.colorset")
	if err := os.MkdirAll(usedPath, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}
	if err := os.MkdirAll(unusedA, 0o755); err != nil {
		t.Fatalf("mkdir unused asset set A: %v", err)
	}
	if err := os.MkdirAll(unusedB, 0o755); err != nil {
		t.Fatalf("mkdir unused asset set B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Main.swift"), []byte(`let _ = UIImage(named: "used")`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	initCleanGitRepo(t, root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "prune", "--path", root, "--apply"}, &stdout, &stderr)
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
	deleted, ok := payload["deleted"].([]any)
	if !ok || len(deleted) != 2 {
		t.Fatalf("unexpected deleted payload: %#v", payload["deleted"])
	}
	if deleted[0] != unusedA || deleted[1] != unusedB {
		t.Fatalf("expected deterministic sorted deleted payload [%s %s], got %#v", unusedA, unusedB, deleted)
	}
	if payload["dryRun"] != false {
		t.Fatalf("expected dryRun=false, got %v", payload["dryRun"])
	}
	if _, err := os.Stat(usedPath); err != nil {
		t.Fatalf("expected used asset set to remain, stat err=%v", err)
	}
	if _, err := os.Stat(unusedA); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be deleted, stat err=%v", unusedA, err)
	}
	if _, err := os.Stat(unusedB); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be deleted, stat err=%v", unusedB, err)
	}
}

func TestAssetsPrune_ApplyRequiresCleanGitTree(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	unusedPath := filepath.Join(catalog, "unused.imageset")
	if err := os.MkdirAll(unusedPath, 0o755); err != nil {
		t.Fatalf("mkdir unused asset set: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unusedPath, "Contents.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write contents: %v", err)
	}
	initCleanGitRepo(t, root)
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "prune", "--path", root, "--apply"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
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
	if errVal["code"] != "runtime_error" {
		t.Fatalf("unexpected error code: %v", errVal["code"])
	}
	message, _ := errVal["message"].(string)
	if !strings.Contains(message, "git working tree is not clean") {
		t.Fatalf("unexpected error message: %q", message)
	}
	if _, err := os.Stat(unusedPath); err != nil {
		t.Fatalf("expected apply rejection to keep %s, stat err=%v", unusedPath, err)
	}
}

func TestAssetsPrune_ApplyForceOverridesDirtyGitTree(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	unusedPath := filepath.Join(catalog, "unused.imageset")
	if err := os.MkdirAll(unusedPath, 0o755); err != nil {
		t.Fatalf("mkdir unused asset set: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unusedPath, "Contents.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write contents: %v", err)
	}
	initCleanGitRepo(t, root)
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Execute([]string{"assets", "prune", "--path", root, "--apply", "--force"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}
	if _, err := os.Stat(unusedPath); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be deleted, stat err=%v", unusedPath, err)
	}
}

func initCleanGitRepo(t *testing.T, root string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	runGit(t, root, "init", "--quiet")
	runGit(t, root, "config", "user.email", "tests@example.com")
	runGit(t, root, "config", "user.name", "xcwrap tests")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "--quiet", "-m", "initial")
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = withEnvVar(os.Environ(), "LC_ALL", "C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v, output=%s", args, err, strings.TrimSpace(string(out)))
	}
}

func withEnvVar(base []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range base {
		if strings.HasPrefix(entry, prefix) {
			updated := append([]string{}, base...)
			updated[i] = prefix + value
			return updated
		}
	}
	return append(base, prefix+value)
}
