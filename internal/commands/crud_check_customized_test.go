package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareRenderedWithDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	unchanged := filepath.Join(dir, "unchanged.go")
	changed := filepath.Join(dir, "changed.go")
	missing := filepath.Join(dir, "missing.go")
	if err := os.WriteFile(unchanged, []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changed, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift := compareRenderedWithDisk(map[string]string{
		unchanged: "same\n",
		changed:   "new\n",
		missing:   "created\n",
	})
	if len(drift) != 2 {
		t.Fatalf("drift = %+v", drift)
	}
	if drift[0].Path != changed || drift[0].Missing || drift[0].Before != "old\n" || drift[0].Rendered != "new\n" {
		t.Fatalf("changed entry = %+v", drift[0])
	}
	if drift[1].Path != missing || !drift[1].Missing {
		t.Fatalf("missing entry = %+v", drift[1])
	}
}
