package crud_helper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anchorFixture 构造一个含空锚点块的主实体 repo 文件内容（模板渲染产物的
// 等价形态，gofmt 后格式）。
func anchorFixture(className string) string {
	return `package repository

func (s *` + className + `Repository) CascadeOwners() []data_scope.CascadeOwner {
	return []data_scope.CascadeOwner{
		// @cascade:begin
		// @cascade:end
	}
}
`
}

func writeAnchorFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "user_gen.go")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func readAnchorFixture(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func TestCascadeAnchorEntryFormat(t *testing.T) {
	assert.Equal(t, "\t\t{Table: \"user_money_log\", ByColumn: \"user_id\", OwnerColumn: \"admin_id\"},", cascadeAnchorEntry("user_money_log", "user_id"))
}

func TestApplyCascadeAnchorInsertsEntry(t *testing.T) {
	path := writeAnchorFixture(t, anchorFixture("User"))
	require.NoError(t, applyCascadeAnchor(path, "user_order", "user_id"))
	content := readAnchorFixture(t, path)
	assert.Contains(t, content, "\t\t// @cascade:begin\n\t\t{Table: \"user_order\", ByColumn: \"user_id\", OwnerColumn: \"admin_id\"},\n\t\t// @cascade:end")
}

func TestApplyCascadeAnchorIdempotent(t *testing.T) {
	path := writeAnchorFixture(t, anchorFixture("User"))
	require.NoError(t, applyCascadeAnchor(path, "user_order", "user_id"))
	require.NoError(t, applyCascadeAnchor(path, "user_order", "user_id"))
	content := readAnchorFixture(t, path)
	assert.Equal(t, 1, strings.Count(content, "{Table: \"user_order\","))
}

func TestApplyCascadeAnchorReplacesChangedByColumn(t *testing.T) {
	path := writeAnchorFixture(t, anchorFixture("User"))
	require.NoError(t, applyCascadeAnchor(path, "user_order", "user_id"))
	require.NoError(t, applyCascadeAnchor(path, "user_money_log", "user_id"))
	// 子表重新生成改 byColumn：同 Table 整行原位替换，旧列不残留。
	require.NoError(t, applyCascadeAnchor(path, "user_order", "order_id"))
	content := readAnchorFixture(t, path)
	assert.Contains(t, content, "\t\t{Table: \"user_order\", ByColumn: \"order_id\", OwnerColumn: \"admin_id\"},")
	assert.NotContains(t, content, "\t\t{Table: \"user_order\", ByColumn: \"user_id\", OwnerColumn: \"admin_id\"},")
	assert.Equal(t, 1, strings.Count(content, "{Table: \"user_order\","))
	// 替换保持插入位置（在 user_money_log 之前，即原 user_order 行位置）。
	orderIdx := strings.Index(content, "{Table: \"user_order\", ByColumn: \"order_id\"")
	moneyIdx := strings.Index(content, "{Table: \"user_money_log\",")
	require.GreaterOrEqual(t, orderIdx, 0)
	assert.Less(t, orderIdx, moneyIdx)
	// 其它条目与锚点注释不动。
	assert.Contains(t, content, "{Table: \"user_money_log\", ByColumn: \"user_id\", OwnerColumn: \"admin_id\"},")
	assert.Contains(t, content, "// @cascade:begin")
	assert.Contains(t, content, "// @cascade:end")
}

func TestApplyCascadeAnchorMultipleEntries(t *testing.T) {
	path := writeAnchorFixture(t, anchorFixture("User"))
	require.NoError(t, applyCascadeAnchor(path, "user_order", "user_id"))
	require.NoError(t, applyCascadeAnchor(path, "user_money_log", "user_id"))
	content := readAnchorFixture(t, path)
	assert.Contains(t, content, "{Table: \"user_order\", ByColumn: \"user_id\", OwnerColumn: \"admin_id\"},")
	assert.Contains(t, content, "{Table: \"user_money_log\", ByColumn: \"user_id\", OwnerColumn: \"admin_id\"},")
}

func TestApplyCascadeAnchorMissingFileFails(t *testing.T) {
	err := applyCascadeAnchor(filepath.Join(t.TempDir(), "missing.go"), "user_order", "user_id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apply cascade anchor")
}

func TestApplyCascadeAnchorMissingAnchorFails(t *testing.T) {
	path := writeAnchorFixture(t, "package repository\n\nfunc (s *UserRepository) CascadeOwners() []data_scope.CascadeOwner {\n\treturn nil\n}\n")
	err := applyCascadeAnchor(path, "user_order", "user_id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anchor block not found")
}

// TestApplyCascadeAnchorToleratesWhitespace 覆盖空白容忍匹配（评审 MINOR：
// gofmt/手改的空格差异不得破坏幂等与替换判定）：手改条目带多余空格时，重新
// 生成（同 Table 新 ByColumn）应原位替换为规范格式，而不是追加第二条。
func TestApplyCascadeAnchorToleratesWhitespace(t *testing.T) {
	path := writeAnchorFixture(t, anchorFixture("User"))
	// 手改后的条目：Table/ByColumn 间多余空格、无尾逗号。
	handEdited := "\t\t{Table:  \"user_order\",  ByColumn:\"user_id\"  , OwnerColumn: \"admin_id\"}"
	content := readAnchorFixture(t, path)
	content = strings.Replace(content, "\t\t// @cascade:begin\n\t\t// @cascade:end", "\t\t// @cascade:begin\n"+handEdited+"\n\t\t// @cascade:end", 1)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	// 子表重新生成改 byColumn：同 Table 整行替换为规范格式，不追加。
	require.NoError(t, applyCascadeAnchor(path, "user_order", "order_id"))
	out := readAnchorFixture(t, path)
	assert.Contains(t, out, "\t\t{Table: \"user_order\", ByColumn: \"order_id\", OwnerColumn: \"admin_id\"},")
	assert.NotContains(t, out, handEdited)
	assert.Equal(t, 1, strings.Count(out, "{Table: \"user_order\","))
	// 幂等：规范条目已存在，再 apply 不产生重复。
	require.NoError(t, applyCascadeAnchor(path, "user_order", "order_id"))
	out = readAnchorFixture(t, path)
	assert.Equal(t, 1, strings.Count(out, "{Table: \"user_order\","))
	// remove 对空白变体同样有效。
	require.NoError(t, removeCascadeAnchor(path, "user_order"))
	assert.NotContains(t, readAnchorFixture(t, path), "user_order")
}

// TestApplyCascadeAnchorConvergesDuplicates 覆盖重复条目收敛：同 Table 出现
// 两条（历史手改/旧逻辑残留）时，apply 收敛为一条规范条目，避免级联重复执行。
func TestApplyCascadeAnchorConvergesDuplicates(t *testing.T) {
	path := writeAnchorFixture(t, anchorFixture("User"))
	dup1 := "\t\t{Table: \"user_order\", ByColumn: \"user_id\", OwnerColumn: \"admin_id\"},"
	dup2 := "\t\t{Table: \"user_order\", ByColumn: \"user_id\", OwnerColumn: \"admin_id\"}, // 手改注释"
	content := strings.Replace(readAnchorFixture(t, path),
		"\t\t// @cascade:begin\n\t\t// @cascade:end",
		"\t\t// @cascade:begin\n"+dup1+"\n"+dup2+"\n\t\t// @cascade:end", 1)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	require.NoError(t, applyCascadeAnchor(path, "user_order", "user_id"))
	out := readAnchorFixture(t, path)
	assert.Equal(t, 1, strings.Count(out, "{Table: \"user_order\","))
	assert.Equal(t, 1, strings.Count(out, "ByColumn: \"user_id\""))
}

// TestWritePreservingModeAtomic 覆盖原子写回：同目录临时文件 + rename，
// 原文件权限保留，写回后内容一致。
func TestWritePreservingModeAtomic(t *testing.T) {
	path := writeAnchorFixture(t, "package repository\n")
	require.NoError(t, os.Chmod(path, 0640))
	require.NoError(t, writePreservingMode(path, "package repository\n\n// v2\n"))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
	assert.Equal(t, "package repository\n\n// v2\n", readAnchorFixture(t, path))
	// 目录中不得残留临时文件。
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "leftover temp file: %s", e.Name())
	}
}

func TestRemoveCascadeAnchorRemovesEntry(t *testing.T) {
	path := writeAnchorFixture(t, anchorFixture("User"))
	require.NoError(t, applyCascadeAnchor(path, "user_order", "user_id"))
	require.NoError(t, applyCascadeAnchor(path, "user_money_log", "user_id"))
	require.NoError(t, removeCascadeAnchor(path, "user_order"))
	content := readAnchorFixture(t, path)
	assert.NotContains(t, content, "{Table: \"user_order\",")
	assert.Contains(t, content, "{Table: \"user_money_log\",")
	// 前缀不误伤：child 与 child2 各自独立
	require.NoError(t, applyCascadeAnchor(path, "child2", "user_id"))
	require.NoError(t, removeCascadeAnchor(path, "child"))
	content = readAnchorFixture(t, path)
	assert.Contains(t, content, "{Table: \"child2\",")
	assert.NotContains(t, content, "{Table: \"child\",")
}

func TestRemoveCascadeAnchorIdempotentAndTolerant(t *testing.T) {
	path := writeAnchorFixture(t, anchorFixture("User"))
	// 条目不存在 → nil
	require.NoError(t, removeCascadeAnchor(path, "ghost"))
	// 文件不存在 → nil
	require.NoError(t, removeCascadeAnchor(filepath.Join(t.TempDir(), "missing.go"), "ghost"))
	// 锚点缺失但文件存在 → nil（宽容）
	noAnchor := writeAnchorFixture(t, "package repository\n\nfunc (s *UserRepository) CascadeOwners() []data_scope.CascadeOwner {\n\treturn nil\n}\n")
	require.NoError(t, removeCascadeAnchor(noAnchor, "ghost"))
}

func TestStaleInheritParent(t *testing.T) {
	logWith := func(table string) *crudmodel.Log {
		return &crudmodel.Log{Tablename: "order",
			Table: crudmodel.JSON_TABLE(crudmodel.Table{
				Name:      "order",
				DataScope: &data_scope.Config{Mode: data_scope.ModeAuto, InheritFrom: &data_scope.InheritRef{Table: table, ByColumn: "user_id"}},
			})}
	}
	// 无旧记录 → 无清理
	assert.Equal(t, "", staleInheritParent(nil, &data_scope.InheritRef{Table: "seller_user", ByColumn: "user_id"}))
	// 旧记录无 inheritFrom → 无清理
	assert.Equal(t, "", staleInheritParent(&crudmodel.Log{Table: crudmodel.JSON_TABLE(crudmodel.Table{Name: "order"})}, &data_scope.InheritRef{Table: "seller_user", ByColumn: "user_id"}))
	// 新声明去掉 inheritFrom → 清理旧主实体
	assert.Equal(t, "seller_user", staleInheritParent(logWith("seller_user"), nil))
	// 新声明改指向 → 清理旧主实体
	assert.Equal(t, "seller_user", staleInheritParent(logWith("seller_user"), &data_scope.InheritRef{Table: "order_user", ByColumn: "user_id"}))
	// 指向未变（含 byColumn 变化）→ 不清理（apply 整行替换处理）
	assert.Equal(t, "", staleInheritParent(logWith("seller_user"), &data_scope.InheritRef{Table: "seller_user", ByColumn: "order_id"}))
}

func TestParentRepositoryPath(t *testing.T) {
	path, err := parentRepositoryPath("seller_user")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(repoRoot(t), "internal", "admin", "repository", "seller_user.go"), path)

	_, err = parentRepositoryPath("evil/../escape")
	require.Error(t, err)
}
