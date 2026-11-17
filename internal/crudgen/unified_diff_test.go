package crudgen

import (
	"strings"
	"testing"
)

func TestUnifiedDiffIdenticalIsEmpty(t *testing.T) {
	t.Parallel()
	if diff := UnifiedDiff("a.go", "one\ntwo\n", "one\ntwo\n"); diff != "" {
		t.Fatalf("identical files must produce no diff, got %q", diff)
	}
}

func TestUnifiedDiffSingleLineChange(t *testing.T) {
	t.Parallel()
	before := "one\ntwo\nthree\nfour\nfive\n"
	after := "one\ntwo\nTHREE\nfour\nfive\n"
	diff := UnifiedDiff("internal/model/demo.go", before, after)
	if !strings.Contains(diff, "--- a/internal/model/demo.go") || !strings.Contains(diff, "+++ b/internal/model/demo.go") {
		t.Fatalf("missing file headers:\n%s", diff)
	}
	if !strings.Contains(diff, "-three") || !strings.Contains(diff, "+THREE") {
		t.Fatalf("missing changed lines:\n%s", diff)
	}
	if !strings.Contains(diff, "@@ -") {
		t.Fatalf("missing hunk header:\n%s", diff)
	}
	// 上下文行必须出现在同一 hunk 里。
	if !strings.Contains(diff, " two") || !strings.Contains(diff, " four") {
		t.Fatalf("missing context lines:\n%s", diff)
	}
}

func TestUnifiedDiffNewFile(t *testing.T) {
	t.Parallel()
	diff := UnifiedDiff("web/index.vue", "", "line one\nline two\n")
	if !strings.Contains(diff, "+line one") || !strings.Contains(diff, "+line two") {
		t.Fatalf("new file diff must be all additions:\n%s", diff)
	}
	if strings.Contains(diff, "\n-") {
		t.Fatalf("new file diff must not contain deletions:\n%s", diff)
	}
}
