package crudgen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buildadmin-go/internal/pkg/util"
)

func TestRenderOfflineWithoutDatabase(t *testing.T) {
	t.Parallel()
	specPath := writeSpecTest(t, `name: ops_offline_demo
comment: 离线演示表
generateRelativePath: ops_offline_demo
fields:
  id: {type: bigint, unsigned: true, primaryKey: true, autoIncrement: true, designType: pk, comment: ID}
  name: {type: varchar, length: 50, comment: 名称}
  status: {type: tinyint, length: 1, default: "1", designType: switch, comment: "状态:0=禁用,1=启用"}
  create_time: {type: bigint, comment: 创建时间}
`)
	outDir := t.TempDir()
	files, resolved, err := RenderOffline(specPath, outDir, "ba_")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != outDir {
		t.Fatalf("resolved out dir = %q, want %q", resolved, outDir)
	}
	if len(files) == 0 {
		t.Fatal("offline render wrote no files")
	}
	entityPath := filepath.Join(outDir, "internal/model/ops_offline_demo.go")
	entity, err := os.ReadFile(entityPath) //nolint:gosec // G304: 测试内临时目录夹具
	if err != nil {
		t.Fatalf("entity not rendered: %v", err)
	}
	for _, want := range []string{"validator.FlexBool", "buildadmin-go/internal/pkg/validator", "column:name"} {
		if !strings.Contains(string(entity), want) {
			t.Fatalf("entity missing %q:\n%s", want, entity)
		}
	}
	// 仓库自身不得被写入
	if _, err := os.Stat(filepath.Join(util.RootPath(), "internal/model/ops_offline_demo.go")); !os.IsNotExist(err) {
		t.Fatalf("offline render must not touch the repository, err = %v", err)
	}
}

func TestRenderOfflineRejectsRelations(t *testing.T) {
	t.Parallel()
	specPath := writeSpecTest(t, `name: ops_offline_relation
fields:
  id: {type: bigint, primaryKey: true}
  user_id:
    type: int
    designType: remoteSelect
    form: {remoteTable: user, remotePk: id, remoteField: username, relationFields: username}
`)
	if _, _, err := RenderOffline(specPath, t.TempDir(), "ba_"); err == nil || !strings.Contains(err.Error(), "relation columns") {
		t.Fatalf("error = %v", err)
	}
}

// 离线渲染必须完全确定：同一 spec 渲染两次，所有产物逐字节一致（map 迭代
// 顺序泄漏到模板会让 popupForm.vue 规则顺序漂移，check-customized 也会误报）。
func TestRenderOfflineDeterministic(t *testing.T) {
	t.Parallel()
	specPath := writeSpecTest(t, `name: ops_offline_deterministic
comment: 确定性演示
fields:
  id: {type: bigint, unsigned: true, primaryKey: true, autoIncrement: true, designType: pk, comment: ID}
  rate: {type: decimal, length: 10, precision: 2, designType: number, comment: 费率}
  status: {type: tinyint, length: 1, designType: number, comment: "状态:0=禁用,1=启用"}
  remark: {type: varchar, length: 255, designType: textarea, comment: 备注}
  create_time: {type: bigint, comment: 创建时间}
`)
	first := t.TempDir()
	second := t.TempDir()
	firstFiles, _, err := RenderOffline(specPath, first, "ba_")
	if err != nil {
		t.Fatal(err)
	}
	secondFiles, _, err := RenderOffline(specPath, second, "ba_")
	if err != nil {
		t.Fatal(err)
	}
	if len(firstFiles) != len(secondFiles) {
		t.Fatalf("file count differs: %d vs %d", len(firstFiles), len(secondFiles))
	}
	for _, relative := range firstFiles {
		left, err := os.ReadFile(filepath.Join(first, relative)) //nolint:gosec // G304: 测试内临时目录夹具
		if err != nil {
			t.Fatal(err)
		}
		right, err := os.ReadFile(filepath.Join(second, relative)) //nolint:gosec // G304: 测试内临时目录夹具
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(left, right) {
			t.Fatalf("%s differs between renders", relative)
		}
	}
}
