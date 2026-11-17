package crudgen

import (
	"os"
	"path/filepath"
	"testing"
)

// spec 声明了归属语义，就必须在删除管理员时可被转移；这里守住扫描器对两个键
// 的识别（含 inheritFrom 与 reassignable 同时出现、重复表名去重）。
func TestCollectOwnerTablesFindsSpecDeclarations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("ops_order.yaml", `name: ops_order
dataScope:
  mode: auto
  inheritFrom: {table: seller_user, byColumn: seller_id}
fields: []
`)
	write("seller_user.yaml", `name: seller_user
dataScope:
  mode: auto
  reassignable: true
fields: []
`)
	write("ops_public.yaml", `name: ops_public
dataScope:
  mode: none
fields: []
`)
	write("notes.txt", "not a spec")
	tables, err := CollectOwnerTables(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"ops_order": true, "seller_user": true}
	if len(tables) != len(want) {
		t.Fatalf("tables = %v", tables)
	}
	for _, table := range tables {
		if !want[table] {
			t.Fatalf("unexpected table %q in %v", table, tables)
		}
	}
}

func TestCollectOwnerTablesRejectsBrokenSpec(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte("name: [unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CollectOwnerTables(dir); err == nil {
		t.Fatal("a broken spec must fail loudly instead of being treated as owner-free")
	}
}
