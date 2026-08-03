package crud_helper

import (
	"buildadmin-go/internal/conf"
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
