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
