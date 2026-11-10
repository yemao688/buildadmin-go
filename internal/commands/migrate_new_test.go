package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 版本号取注册表与目录文件名前缀的较大值加一，避免与未编译的手工文件撞号。
func TestNextBusinessVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0003_shop_orders.go"), []byte("package business\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := nextBusinessVersion(dir, []uint64{2})
	if err != nil {
		t.Fatalf("nextBusinessVersion: %v", err)
	}
	if got != 4 {
		t.Fatalf("nextBusinessVersion = %d, want 4", got)
	}
}

// 文件名用下划线，迁移名保留原样。
func TestMigrationFileStem(t *testing.T) {
	if got := migrationFileStem("Shop-Orders"); got != "shop_orders" {
		t.Fatalf("migrationFileStem = %q, want shop_orders", got)
	}
}

// 骨架必须带正确版本、迁移名与 package，业务只需填 Up；默认不生成可执行 Down。
func TestRenderBusinessMigration(t *testing.T) {
	rendered := renderBusinessMigration(7, "shop-orders")
	for _, want := range []string{"package business", "Version:       7", `MigrationName: "shop-orders"`, "func init()", "Register(Migration{"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered migration missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "\n\t\tDown: func(") {
		t.Fatalf("rendered migration must not include an executable Down:\n%s", rendered)
	}
}

// 非法迁移名不得落到文件系统。
func TestCreateBusinessMigrationRejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	if _, err := createBusinessMigration(dir, "../evil", nil); err == nil {
		t.Fatal("expected invalid migration name to be rejected")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("invalid name created files: %v", entries)
	}
}

// 连续创建两条不同迁移得到递增版本号，已有文件不被覆盖。
func TestCreateBusinessMigrationUsesNextVersion(t *testing.T) {
	dir := t.TempDir()
	first, err := createBusinessMigration(dir, "shop-orders", nil)
	if err != nil {
		t.Fatalf("first createBusinessMigration: %v", err)
	}
	original, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := createBusinessMigration(dir, "settle-orders", nil)
	if err != nil {
		t.Fatalf("second createBusinessMigration: %v", err)
	}
	if filepath.Base(first) != "0001_shop_orders.go" || filepath.Base(second) != "0002_settle_orders.go" {
		t.Fatalf("created %q and %q, want 0001_shop_orders.go / 0002_settle_orders.go", filepath.Base(first), filepath.Base(second))
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatal("existing migration file was overwritten")
	}
}

// 同名迁移（文件已存在但尚未重新编译）必须被拒绝，避免生成重复 MigrationName。
func TestCreateBusinessMigrationRejectsDuplicateStem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0001_shop_orders.go"), []byte("package business\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := createBusinessMigration(dir, "shop-orders", nil); err == nil {
		t.Fatal("expected duplicate migration name to be rejected")
	}
}
