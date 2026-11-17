package crudgen

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"buildadmin-go/internal/pkg/util"
)

// crud_specs/schema.json 由 crud:schema 生成并入库：陈旧时编辑器补全与
// crud:validate 就会对不上，因此用测试守住同步。
func TestSpecSchemaFileIsUpToDate(t *testing.T) {
	t.Parallel()
	encoded, err := json.MarshalIndent(SpecSchema(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	path := filepath.Join(util.RootPath(), "crud_specs", "schema.json")
	current, err := os.ReadFile(path) //nolint:gosec // G304: 仓库内固定路径
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(current, encoded) {
		t.Fatalf("%s is stale; run: go run ./cmd/crudgen crud:schema --out crud_specs/schema.json", path)
	}
}
