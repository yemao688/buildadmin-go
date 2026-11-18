package commands

import (
	"os"
	"path/filepath"
	"testing"

	"buildadmin-go/internal/pkg/crudhooks"

	"github.com/gin-gonic/gin"
)

func TestUnregisteredMutationHooksReportsMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	spec := `name: ops_demo
hooks:
  afterMutation: test-hook-missing
fields:
  - name: id
    type: bigint
    primaryKey: true
`
	if err := os.WriteFile(filepath.Join(dir, "ops_demo.yaml"), []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	missing, err := unregisteredMutationHooks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != "ops_demo -> test-hook-missing" {
		t.Fatalf("missing = %v", missing)
	}
	if err := crudhooks.RegisterMutation("test-hook-missing", func(*gin.Context) {}); err != nil {
		t.Fatal(err)
	}
	missing, err = unregisteredMutationHooks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("registered hook must not be reported: %v", missing)
	}
}
