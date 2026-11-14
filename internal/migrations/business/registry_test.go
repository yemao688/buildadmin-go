package business

import (
	"testing"

	"buildadmin-go/internal/conf"
	"gorm.io/gorm"
)

func validMigration(version uint64, name string) Migration {
	return Migration{Version: version, MigrationName: name, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}
}

func resetRegistryForTest() {
	mu.Lock()
	defer mu.Unlock()
	registry = nil
	frozen = false
}

//nolint:paralleltest // 测试重置并写入包级 business 迁移注册表（registry/frozen），并行会污染同包其它用例
func TestRegistryValidationSortingCopyAndFreeze(t *testing.T) {
	resetRegistryForTest()
	second := validMigration(2, "two")
	second.Down = func(*gorm.DB, *conf.Configuration) error { return nil }
	Register(second)
	Register(validMigration(1, "one"))
	got, err := Migrations()
	if err != nil || len(got) != 2 || got[0].MigrationName != "one" || got[1].MigrationName != "two" {
		t.Fatalf("got migrations=%v, err=%v", got, err)
	}
	if got[1].Down == nil {
		t.Fatal("optional Down function was not retained")
	}
	got[0].MigrationName = "changed"
	again, err := Migrations()
	if err != nil || again[0].MigrationName != "one" {
		t.Fatalf("registry was not copied: %v, err=%v", again, err)
	}
	assertRegisterPanics(t, validMigration(3, "three"))
}

//nolint:paralleltest // 测试重置并写入包级 business 迁移注册表（registry/frozen），并行会污染同包其它用例
func TestRegistryValidationErrors(t *testing.T) {
	cases := [][]Migration{
		{validMigration(2, "one")},
		{{Version: 1, MigrationName: " ", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}},
		{{Version: 1, MigrationName: "one"}},
		{{Version: 1, MigrationName: "one", Up: nil}},
		{validMigration(1, "one"), validMigration(1, "two")},
		{validMigration(1, "one"), validMigration(2, "one")},
	}
	for i, migration := range cases {
		resetRegistryForTest()
		for _, item := range migration {
			Register(item)
		}
		if _, err := Migrations(); err == nil {
			t.Errorf("case %d accepted invalid migration", i)
		}
	}
}

func assertRegisterPanics(t *testing.T, migration Migration) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("Register after Migrations did not panic")
		}
	}()
	Register(migration)
}

func TestValidateMigrations(t *testing.T) {
	t.Parallel()
	noop := func(*gorm.DB, *conf.Configuration) error { return nil }

	valid := validMigration(1, "shop-orders")
	valid.NeedsSpecTables = []string{"ops_banner", "ops_support", "user_money_log"}
	if err := validateMigrations([]Migration{valid}); err != nil {
		t.Fatalf("合法的 NeedsSpecTables 被拒绝: %v", err)
	}

	cases := []struct {
		intent string
		list   []Migration
	}{
		{"版本必须从 1 开始连续", []Migration{validMigration(2, "one")}},
		{"迁移名不能为空白", []Migration{{Version: 1, MigrationName: " ", Up: noop}}},
		{"迁移名必须唯一", []Migration{validMigration(1, "one"), validMigration(2, "one")}},
		{"Up 不能为空", []Migration{{Version: 1, MigrationName: "one"}}},
		{"spec 表名不能为空白", []Migration{{Version: 1, MigrationName: "one", Up: noop, NeedsSpecTables: []string{" "}}}},
		{"spec 表名不能重复声明", []Migration{{Version: 1, MigrationName: "one", Up: noop, NeedsSpecTables: []string{"ops_banner", "ops_banner"}}}},
		{"spec 表名不能含路径分隔符", []Migration{{Version: 1, MigrationName: "one", Up: noop, NeedsSpecTables: []string{"ops/banner"}}}},
		{"spec 表名不能含反斜杠", []Migration{{Version: 1, MigrationName: "one", Up: noop, NeedsSpecTables: []string{`ops\banner`}}}},
		{"spec 表名不能含上跳路径", []Migration{{Version: 1, MigrationName: "one", Up: noop, NeedsSpecTables: []string{"../ops_banner"}}}},
	}
	for _, tc := range cases {
		if err := validateMigrations(tc.list); err == nil {
			t.Errorf("%s: 未拒绝非法迁移", tc.intent)
		}
	}
}
