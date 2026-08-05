package business

// MySQL 门禁集成测试：种子迁移依赖表的 EnsureSpecTable 物化全链路。
// 未配置 mysql_test 时 testutil.OpenMySQL 自动跳过。

import (
	"os"
	"path/filepath"
	"testing"

	helper "buildadmin-go/internal/pkg/crud_helper"
	"buildadmin-go/internal/pkg/testutil"
)

const specTableTestPrefix = "ba_spec_table_test_"

func writeSpecTableSpec(t *testing.T, dir, name, extra string) {
	t.Helper()
	spec := `name: ` + name + `
comment: 种子依赖表测试
fields:
  - name: id
    type: bigint
    primaryKey: true
    autoIncrement: true
    unsigned: true
  - name: code
    type: varchar
    length: 64
` + extra
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestEnsureSpecTableSeedMigrationFlow 模拟"种子迁移 Up 内 EnsureSpecTable 建表
// → 写种子 → 尾部 apply unchanged"：全新库上种子迁移必须先拿到表才能写数据，
// 且建表不得触发 requires-approval 阻塞。
func TestEnsureSpecTableSeedMigrationFlow(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = specTableTestPrefix
	for _, table := range []string{"ops_seed_target", "crud_log"} {
		if err := db.Exec("DROP TABLE IF EXISTS `" + specTableTestPrefix + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, table := range []string{"ops_seed_target", "crud_log"} {
			_ = db.Exec("DROP TABLE IF EXISTS `" + specTableTestPrefix + table + "`").Error
		}
	})
	if err := db.Exec("CREATE TABLE `" + specTableTestPrefix + "crud_log` (`id` int NOT NULL AUTO_INCREMENT, `admin_id` int NOT NULL, `table_name` varchar(200) NOT NULL, `table` blob, `fields` blob, `status` varchar(30) NOT NULL DEFAULT 'start', `comment` varchar(255), `connection` varchar(100) NOT NULL DEFAULT '', `sync` int NOT NULL DEFAULT 0, `create_time` bigint NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeSpecTableSpec(t, dir, "ops_seed_target", `indexes:
  - name: uk_code
    unique: true
    columns: [code]
`)

	// 1. 迁移 Up 内物化依赖表（全新库：ApplyCreated，无阻塞）。
	if err := EnsureSpecTableFrom(db, cfg, dir, "ops_seed_target"); err != nil {
		t.Fatalf("EnsureSpecTableFrom: %v", err)
	}
	table := specTableTestPrefix + "ops_seed_target"
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("table %s not created", table)
	}

	// 2. 种子写入（表已存在）。
	if err := db.Exec("INSERT INTO `" + table + "` (`code`) VALUES ('seed-code')").Error; err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	// 3. 尾部 apply 幂等：全量 spec 应用应为 unchanged（不阻塞全新安装）。
	results, err := helper.ApplySpecs(db, cfg, []string{filepath.Join(dir, "ops_seed_target.yaml")}, helper.ApplyOptions{AdminID: 1, SkipMenu: true})
	if err != nil {
		t.Fatalf("tail apply: %v", err)
	}
	if len(results) != 1 || results[0].Action != helper.ApplyUnchanged {
		t.Fatalf("tail apply results = %+v, want unchanged", results)
	}

	// 4. 幂等：重复执行 EnsureSpecTable 不报错、不产生新变更。
	if err := EnsureSpecTableFrom(db, cfg, dir, "ops_seed_target"); err != nil {
		t.Fatalf("second EnsureSpecTableFrom: %v", err)
	}
	var seedCount int64
	if err := db.Raw("SELECT COUNT(*) FROM `" + table + "` WHERE code = 'seed-code'").Scan(&seedCount).Error; err != nil {
		t.Fatal(err)
	}
	if seedCount != 1 {
		t.Fatalf("seed rows = %d, want 1", seedCount)
	}
}

// TestEnsureSpecTableRejectsMissingSpec 校验：spec 不存在时直接报错（防 typo）。
func TestEnsureSpecTableRejectsMissingSpec(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = specTableTestPrefix

	if err := EnsureSpecTableFrom(db, cfg, t.TempDir(), "no_such_table"); err == nil {
		t.Fatal("EnsureSpecTableFrom with missing spec = nil error, want error")
	}
}
