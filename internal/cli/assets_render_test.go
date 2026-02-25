package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderPruneResult_TableUsesAlignedColumns(t *testing.T) {
	var out bytes.Buffer
	result := pruneResult{
		Command:             "assets prune",
		Path:                "/tmp/repo",
		Apply:               true,
		Force:               false,
		DryRun:              false,
		UnusedCount:         3,
		PruneCandidateCount: 2,
		Deleted:             []string{"a", "b"},
	}

	if err := renderPruneResult(&out, outputTable, result); err != nil {
		t.Fatalf("render prune table: %v", err)
	}

	rendered := out.String()
	if strings.Contains(rendered, "\t") {
		t.Fatalf("expected expanded table output without raw tabs, got %q", rendered)
	}
	if !strings.Contains(rendered, "command") || !strings.Contains(rendered, "deleted_count") {
		t.Fatalf("expected table header fields in output, got %q", rendered)
	}
	if !strings.Contains(rendered, "assets prune") || !strings.Contains(rendered, "/tmp/repo") {
		t.Fatalf("expected table row values in output, got %q", rendered)
	}
}

func TestRenderUnusedResult_TablePreservesCatalogIdentity(t *testing.T) {
	var out bytes.Buffer
	result := unusedResult{
		Command:             "assets unused",
		Path:                "/tmp/repo",
		UnusedCount:         2,
		PruneCandidateCount: 2,
		Unused:              []string{"icon"},
		UnusedByFile: map[string]unusedFileResult{
			"/tmp/repo/Modules/A/Assets.xcassets": {UnusedAssets: []string{"icon.imageset"}},
			"/tmp/repo/Modules/B/Assets.xcassets": {UnusedAssets: []string{"icon.imageset"}},
		},
	}

	if err := renderUnusedResult(&out, outputTable, result); err != nil {
		t.Fatalf("render unused table: %v", err)
	}

	rendered := out.String()
	if strings.Contains(rendered, "\nAssets.xcassets\n  -  icon.imageset\nAssets.xcassets\n") {
		t.Fatalf("expected distinct catalog paths in output, got collapsed basenames: %q", rendered)
	}
	if !strings.Contains(rendered, "/tmp/repo/Modules/A/Assets.xcassets") {
		t.Fatalf("expected module A catalog path in output, got %q", rendered)
	}
	if !strings.Contains(rendered, "/tmp/repo/Modules/B/Assets.xcassets") {
		t.Fatalf("expected module B catalog path in output, got %q", rendered)
	}
}
