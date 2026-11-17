package crudgen

import (
	"buildadmin-go/internal/conf"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteSpecOnlyRemovesSpec(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ops_demo.yaml")
	if err := os.WriteFile(path, []byte("name: ops_demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := deleteSpecOnlyIn(dir, "ops_demo")
	var specOnly *SpecOnlyDeletedError
	if !errors.As(err, &specOnly) {
		t.Fatalf("error = %v", err)
	}
	if specOnly.Table != "ops_demo" || specOnly.Path != path {
		t.Fatalf("result = %+v", specOnly)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("spec must be removed, err = %v", statErr)
	}
}

func TestDeleteSpecOnlyWithoutSpecFails(t *testing.T) {
	t.Parallel()
	if err := deleteSpecOnlyIn(t.TempDir(), "ops_missing"); err == nil {
		t.Fatal("missing spec must fail")
	}
}

// spec-only 删除必须先过边界检查：registerOnly 的核心表从不写 crud_log，
// 会走到无 manifest 的回退分支，不能因此绕过保护名单；非法标识符（../）
// 也不能拼进 crud_specs/ 之外的路径。
func TestValidateSpecOnlyDeleteTargetGuards(t *testing.T) {
	t.Parallel()
	cfg := &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}
	if err := validateSpecOnlyDeleteTarget(cfg, "user_money_log"); err == nil {
		t.Fatal("protected table must be rejected")
	}
	if err := validateSpecOnlyDeleteTarget(cfg, "../../evil"); err == nil {
		t.Fatal("non-identifier table name must be rejected")
	}
	if err := validateSpecOnlyDeleteTarget(cfg, "ba_user"); err == nil {
		t.Fatal("prefixed protected table must be rejected")
	}
}
