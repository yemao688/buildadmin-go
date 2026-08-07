package crud_helper

import (
	"buildadmin-go/internal/conf"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/util"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTruncateCrudLogCommentFitsVarcharLimit(t *testing.T) {
	short := "stage=menu generation: duplicate name"
	if got := truncateCrudLogComment(short); got != short {
		t.Fatalf("short message changed: %q", got)
	}
	long := "stage=compile: " + strings.Repeat("x", 500)
	got := truncateCrudLogComment(long)
	if len([]rune(got)) > crudLogCommentLimit {
		t.Fatalf("truncated message exceeds limit: %d runes", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncated message missing ellipsis: %q", got[len(got)-12:])
	}
	// 多字节字符按 rune 截断，不产生非法 UTF-8
	multi := strings.Repeat("状态", 200)
	got = truncateCrudLogComment(multi)
	if len([]rune(got)) > crudLogCommentLimit || !utf8.ValidString(got) {
		t.Fatalf("multibyte truncation invalid: runes=%d", len([]rune(got)))
	}
}

func TestReconcileStaleGeneratingLogs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = "ba_"
	if err := db.Exec("CREATE TABLE ba_crud_log (id INTEGER PRIMARY KEY AUTOINCREMENT, admin_id INTEGER NOT NULL, table_name TEXT NOT NULL, `table` BLOB, fields BLOB, status TEXT NOT NULL, comment TEXT, connection TEXT NOT NULL, sync INTEGER, create_time INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	insert := func(status string) {
		if err := db.Exec("INSERT INTO ba_crud_log (admin_id, table_name, `table`, fields, status, connection, sync, create_time) VALUES (1, 'orders', '{}', '[]', ?, 'mysql', 0, 1)", status).Error; err != nil {
			t.Fatal(err)
		}
	}
	insert("start")
	insert("success")
	insert("error")

	reconcileStaleGeneratingLogs(db, cfg)

	type row struct {
		Status  string
		Comment string
	}
	var rows []row
	if err := db.Table("ba_crud_log").Select("status", "comment").Order("id").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows[0].Status != "error" || !strings.Contains(rows[0].Comment, "生成中断") {
		t.Fatalf("stale start row not reconciled: %+v", rows[0])
	}
	if rows[1].Status != "success" || rows[2].Status != "error" || strings.Contains(rows[2].Comment, "生成中断") {
		t.Fatalf("non-start rows must stay untouched: %+v", rows[1:])
	}
}

func TestPruneEmptyProviderScaffold(t *testing.T) {
	stop := "internal/admin/handler"
	pkgDir := filepath.Join(util.RootPath(), stop, "prune_scaffold_test")
	t.Cleanup(func() { _ = os.RemoveAll(pkgDir) })
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	scaffold := "package prune_scaffold_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet()\n"
	provider := filepath.Join(pkgDir, "provider.go")
	if err := os.WriteFile(provider, []byte(scaffold), 0644); err != nil {
		t.Fatal(err)
	}
	pruneEmptyProviderScaffold(filepath.Join(stop, "prune_scaffold_test"), stop)
	if _, err := os.Stat(provider); !os.IsNotExist(err) {
		t.Fatalf("empty provider scaffold was not removed: %v", err)
	}
	if _, err := os.Stat(pkgDir); !os.IsNotExist(err) {
		t.Fatalf("empty package dir was not pruned: %v", err)
	}

	// 仍有条目的 provider 必须保留，目录也不删
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	nonEmpty := "package prune_scaffold_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewKeepHandler,\n)\n"
	if err := os.WriteFile(provider, []byte(nonEmpty), 0644); err != nil {
		t.Fatal(err)
	}
	pruneEmptyProviderScaffold(filepath.Join(stop, "prune_scaffold_test"), stop)
	if _, err := os.Stat(provider); err != nil {
		t.Fatalf("non-empty provider must be kept: %v", err)
	}

	// 根包自身不参与清理
	rootProvider := filepath.Join(util.RootPath(), "internal", "admin", "repository", "provider.go")
	before, err := os.ReadFile(rootProvider)
	if err != nil {
		t.Fatal(err)
	}
	pruneEmptyProviderScaffold("internal/admin/repository", "internal/admin/repository")
	after, err := os.ReadFile(rootProvider)
	if err != nil || string(after) != string(before) {
		t.Fatal("root provider.go must not be touched")
	}
}

func TestPruneEmptyDirsUpTo(t *testing.T) {
	stop := t.TempDir()
	nested := filepath.Join(stop, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	pruneEmptyDirsUpTo(nested, stop)
	if _, err := os.Stat(filepath.Join(stop, "a")); !os.IsNotExist(err) {
		t.Fatalf("empty dir chain not pruned: %v", err)
	}
	if _, err := os.Stat(stop); err != nil {
		t.Fatalf("stop dir must be kept: %v", err)
	}

	// 链上有内容时停在该层
	withFile := filepath.Join(stop, "x", "y")
	if err := os.MkdirAll(withFile, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stop, "x", "keep.txt"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	pruneEmptyDirsUpTo(withFile, stop)
	if _, err := os.Stat(filepath.Join(stop, "x", "keep.txt")); err != nil {
		t.Fatalf("non-empty parent must be kept: %v", err)
	}
	if _, err := os.Stat(withFile); !os.IsNotExist(err) {
		t.Fatalf("empty leaf dir not pruned: %v", err)
	}
}

// TestPruneFrontendEmptyDirs 验证生成/删除流程共用的前端空目录修剪：
// 空模块目录链向上清空（止于 views/lang 根），共享父目录中的其他模块
// 文件必须保留，stopAt 根目录绝不删除。路径按生产推导（LangFile 的末段
// 是文件名，模块目录是其父目录）构造，与真实残留形态一致。
func TestPruneFrontendEmptyDirs(t *testing.T) {
	root := util.RootPath()
	table := crudmodel.Table{Name: "prune_fixture", WebViewsDir: "web/src/views/backend/prune_fixture/user/account"}
	viewsDir := ParseWebDirNameData(table.Name, "views", table.WebViewsDir)
	langDir := ParseWebDirNameData(table.Name, "lang", table.WebViewsDir)
	viewsModule := filepath.Join(root, viewsDir.Views)
	langENModule := filepath.Dir(filepath.Join(root, langDir.LangFile("en")))
	langZhModule := filepath.Dir(filepath.Join(root, langDir.LangFile("zh-cn")))
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(root, "web/src/views/backend/prune_fixture"))
		_ = os.RemoveAll(filepath.Join(root, "web/src/lang/backend/en/prune_fixture"))
		_ = os.RemoveAll(filepath.Join(root, "web/src/lang/backend/zh-cn/prune_fixture"))
	})
	// 残留场景：模块目录已建但文件被清（生成失败回滚后的形态），全是空目录
	for _, dir := range []string{viewsModule, langENModule, langZhModule} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// en 下与 account 平级的另一模块文件：共享父目录必须保留
	sibling := filepath.Join(langENModule, "Orders.ts")
	if err := os.WriteFile(sibling, []byte("export default {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pruneFrontendEmptyDirs(table)

	if _, err := os.Stat(langENModule); err != nil {
		t.Fatalf("en module dir with sibling file must be kept: %v", err)
	}
	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("sibling file must be kept: %v", err)
	}
	if _, err := os.Stat(langZhModule); !os.IsNotExist(err) {
		t.Fatalf("empty zh-cn module dir must be pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "web/src/lang/backend/zh-cn/prune_fixture")); !os.IsNotExist(err) {
		t.Fatalf("empty zh-cn ancestor chain must be pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "web/src/lang/backend/zh-cn")); err != nil {
		t.Fatalf("zh-cn locale root must be kept: %v", err)
	}
	if _, err := os.Stat(viewsModule); !os.IsNotExist(err) {
		t.Fatalf("empty views module dir must be pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "web/src/views/backend")); err != nil {
		t.Fatalf("views root must be kept: %v", err)
	}

	// 非法 WebViewsDir 推导出零值 WebDir：no-op，不 panic、不误删根目录
	pruneFrontendEmptyDirs(crudmodel.Table{Name: "prune_fixture", WebViewsDir: "../escape"})
	if _, err := os.Stat(filepath.Join(root, "web/src/views/backend")); err != nil {
		t.Fatalf("views root must survive invalid input: %v", err)
	}
	// 未生成过的模块目录：修剪 no-op，不创建任何东西
	pruneFrontendEmptyDirs(crudmodel.Table{Name: "never_generated_fixture", WebViewsDir: "web/src/views/backend/never_generated_fixture/account"})
	if _, err := os.Stat(filepath.Join(root, "web/src/lang/backend/en/never_generated_fixture")); !os.IsNotExist(err) {
		t.Fatalf("prune must not create dirs: %v", err)
	}
}

// TestRollbackPrunesEmptyFrontendDirs 复现生成失败回滚的真实序列：快照（目标
// 文件均不存在）→ 生成写入文件并 MkdirAll 建目录 → 失败回滚 snapshot.Restore()
// 只删文件、目录残留（修复前的 bug 形态）→ pruneFrontendEmptyDirs 清空目录链。
// 路径全部按生产推导构造（LangFile 末段是文件名、Views 末段是模块目录）。
func TestRollbackPrunesEmptyFrontendDirs(t *testing.T) {
	root := util.RootPath()
	table := crudmodel.Table{Name: "rollback_fixture", WebViewsDir: "web/src/views/backend/rollback_fixture/account"}
	viewsDir := ParseWebDirNameData(table.Name, "views", table.WebViewsDir)
	langDir := ParseWebDirNameData(table.Name, "lang", table.WebViewsDir)
	langEN := filepath.Join(root, langDir.LangFile("en"))
	langZh := filepath.Join(root, langDir.LangFile("zh-cn"))
	viewIndex := filepath.Join(root, viewsDir.Views, "index.vue")
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(root, "web/src/views/backend/rollback_fixture"))
		_ = os.RemoveAll(filepath.Join(root, "web/src/lang/backend/en/rollback_fixture"))
		_ = os.RemoveAll(filepath.Join(root, "web/src/lang/backend/zh-cn/rollback_fixture"))
	})
	// 生成前快照：目标文件均不存在
	snapshot, err := NewFileSnapshot([]string{langEN, langZh, viewIndex})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = snapshot.Cleanup() }()
	// 模拟生成写入：MkdirAll + 写文件
	for _, p := range []string{langEN, langZh, viewIndex} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("generated"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// 失败回滚：文件被删，目录残留（修复前的 bug 形态）
	if err := snapshot.Restore(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(langEN); !os.IsNotExist(err) {
		t.Fatalf("rollback must remove generated file: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(langEN)); err != nil {
		t.Fatalf("pre-fix: dir left after rollback must exist: %v", err)
	}
	// 修复：修剪空目录
	pruneFrontendEmptyDirs(table)
	for _, dir := range []string{filepath.Dir(langEN), filepath.Dir(langZh), filepath.Dir(viewIndex)} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("empty dir must be pruned after rollback: %s: %v", dir, err)
		}
	}
	for _, rootDir := range []string{
		filepath.Join(root, "web/src/lang/backend/en"),
		filepath.Join(root, "web/src/lang/backend/zh-cn"),
		filepath.Join(root, "web/src/views/backend"),
	} {
		if _, err := os.Stat(rootDir); err != nil {
			t.Fatalf("stop root must be kept: %s: %v", rootDir, err)
		}
	}
}
