package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// runCrudNew 直接调用 RunE 而不是 Execute：cobra 的 OnInitialize 会触发
// initConfig 读取进程 os.Args，在测试进程里会把命令误判为未豁免并退出。
func runCrudNew(t *testing.T, args ...string) error {
	t.Helper()
	newCmd := newCrudNewCommand()
	newCmd.SetOut(io.Discard)
	newCmd.SetErr(io.Discard)
	table := args[0]
	if err := newCmd.Flags().Set("dir", args[1]); err != nil {
		t.Fatal(err)
	}
	if len(args) > 2 {
		if err := newCmd.Flags().Set("comment", args[2]); err != nil {
			t.Fatal(err)
		}
	}
	return newCmd.RunE(newCmd, []string{table})
}

func TestCrudNewCreatesValidSpec(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := runCrudNew(t, "ops_banner", dir, "运营轮播"); err != nil {
		t.Fatalf("crud:new failed: %v", err)
	}
	path := filepath.Join(dir, "ops_banner.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("spec not written: %v", err)
	}
	if _, err := helper.ValidateSpec(path); err != nil {
		t.Fatalf("generated skeleton does not validate: %v", err)
	}
}

func TestCrudNewRefusesOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := runCrudNew(t, "ops_banner", dir); err != nil {
		t.Fatal(err)
	}
	if err := runCrudNew(t, "ops_banner", dir); err == nil {
		t.Fatal("second crud:new must refuse to overwrite")
	}
}

func TestCrudNewRejectsUnsafeTableName(t *testing.T) {
	t.Parallel()
	if err := runCrudNew(t, "bad-name;drop", t.TempDir()); err == nil {
		t.Fatal("unsafe table name must be rejected")
	}
}
