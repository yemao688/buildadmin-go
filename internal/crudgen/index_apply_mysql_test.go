package crudgen

// MySQL 门禁集成测试：spec indexes 声明的 apply 全链路。
// 未配置 mysql_test 时 testutil.OpenMySQL 自动跳过（绝不改开发库）。

import (
	"path/filepath"
	"strings"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/testutil"

	"gorm.io/gorm"
)

const idxApplyTestPrefix = "ba_idx_apply_test_"

func writeIdxApplySpec(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "ops_test_index.yaml")
	if err := writeFile(path, body); err != nil {
		t.Fatal(err)
	}
	return path
}

// prepareIdxApplyDB 建立隔离前缀的测试库并清空/重建 fixture 表；清理范围与
// 建表集合保持一致。
func prepareIdxApplyDB(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = idxApplyTestPrefix
	for _, table := range []string{"ops_test_index", "crud_log"} {
		if err := db.Exec("DROP TABLE IF EXISTS `" + idxApplyTestPrefix + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, table := range []string{"ops_test_index", "crud_log"} {
			_ = db.Exec("DROP TABLE IF EXISTS `" + idxApplyTestPrefix + table + "`").Error
		}
	})
	// apply 尾部 adoptCrudLog 需要 crud_log 表（与 country_apply_mysql_test 同一形态）。
	if err := db.Exec("CREATE TABLE `" + idxApplyTestPrefix + "crud_log` (`id` int NOT NULL AUTO_INCREMENT, `admin_id` int NOT NULL, `table_name` varchar(200) NOT NULL, `table` blob, `fields` blob, `status` varchar(30) NOT NULL DEFAULT 'start', `comment` varchar(255), `connection` varchar(100) NOT NULL DEFAULT '', `sync` int NOT NULL DEFAULT 0, `create_time` bigint NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	return db, cfg
}

// applyIdxSpec 执行单文件 apply 并要求恰好产出一条结果。
func applyIdxSpec(t *testing.T, db *gorm.DB, cfg *conf.Configuration, path string) ApplyTableResult {
	t.Helper()
	results, err := ApplySpecs(db, cfg, NewTableStore(db, cfg), []string{path}, ApplyOptions{AdminID: 1, SkipMenu: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %+v, want exactly one", results)
	}
	return results[0]
}

//nolint:paralleltest // ApplySpecs 取进程级 generationMu 生成锁，并行会与其它取锁测试互相返回 ErrGenerationBusy
func TestApplyIndexesLifecycle(t *testing.T) {
	db, cfg := prepareIdxApplyDB(t)
	table := idxApplyTestPrefix + "ops_test_index"

	baseSpec := `name: ops_test_index
comment: 索引集成测试
fields:
  - {name: id, type: bigint, primaryKey: true}
  - name: order_no
    type: varchar
    length: 64
  - name: status
    type: tinyint
`
	indexedSpec := baseSpec + `indexes:
  - name: uk_order_no
    unique: true
    columns: [order_no]
`
	dir := t.TempDir()

	// 1. 全新表：apply 建表并内联唯一索引。
	path := writeIdxApplySpec(t, dir, indexedSpec)
	result := applyIdxSpec(t, db, cfg, path)
	if result.Action != ApplyCreated {
		t.Fatalf("create results = %+v", result)
	}
	assertIndexOnDB(t, db, table, "uk_order_no", true)

	// 2. 幂等：再次 apply 应为 unchanged。
	result = applyIdxSpec(t, db, cfg, path)
	if result.Action != ApplyUnchanged {
		t.Fatalf("second apply = %+v, want unchanged", result)
	}

	// 3. 线外索引：spec 未声明时保留并告警 unmanaged。
	if err := db.Exec("ALTER TABLE `" + table + "` ADD KEY `idx_status` (`status`)").Error; err != nil {
		t.Fatal(err)
	}
	result = applyIdxSpec(t, db, cfg, path)
	if result.Action != ApplyUnchanged {
		t.Fatalf("out-of-band apply = %+v, want unchanged with warning", result)
	}
	if !containsIndexField(result.Unmanaged, "idx_status") {
		t.Fatalf("unmanaged = %+v, want idx_status warning", result.Unmanaged)
	}
	assertIndexOnDB(t, db, table, "idx_status", false)

	// 4. spec 新增索引：safe-auto 补建。
	path2 := writeIdxApplySpec(t, dir, baseSpec+`indexes:
  - name: uk_order_no
    unique: true
    columns: [order_no]
  - name: uk_status_order
    unique: true
    columns: [status, order_no]
`)
	result = applyIdxSpec(t, db, cfg, path2)
	if result.Action != ApplyAltered {
		t.Fatalf("add-index apply = %+v, want altered", result)
	}
	if !containsChangeField(result.Changes, "add-index") {
		t.Fatalf("changes = %+v, want add-index", result.Changes)
	}
	assertIndexOnDB(t, db, table, "uk_status_order", true)

	// 5. spec 移除索引：实际索引保留并告警 unmanaged（索引删除不在 apply 语义内）。
	path3 := writeIdxApplySpec(t, dir, baseSpec+`indexes:
  - name: uk_order_no
    unique: true
    columns: [order_no]
`)
	result = applyIdxSpec(t, db, cfg, path3)
	if result.Action != ApplyUnchanged {
		t.Fatalf("drop-index apply = %+v, want unchanged with warning", result)
	}
	if !containsIndexField(result.Unmanaged, "uk_status_order") {
		t.Fatalf("unmanaged = %+v, want uk_status_order warning", result.Unmanaged)
	}
	assertIndexOnDB(t, db, table, "uk_status_order", true)
}

func assertIndexOnDB(t *testing.T, db *gorm.DB, table, index string, unique bool) {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?", table, index).Scan(&count).Error; err != nil {
		t.Fatalf("query index %s on %s: %v", index, table, err)
	}
	if count == 0 {
		t.Fatalf("index %s missing on %s", index, table)
	}
	var nonUnique int64
	if err := db.Raw("SELECT NON_UNIQUE FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ? AND SEQ_IN_INDEX = 1 LIMIT 1", table, index).Scan(&nonUnique).Error; err != nil {
		t.Fatalf("query uniqueness of %s on %s: %v", index, table, err)
	}
	if (nonUnique == 0) != unique {
		t.Fatalf("index %s unique=%t, want %t", index, nonUnique == 0, unique)
	}
}

func containsIndexField(changes []ApplyChange, field string) bool {
	for _, change := range changes {
		if change.Field == field {
			return true
		}
	}
	return false
}

func containsChangeField(changes []string, want string) bool {
	for _, change := range changes {
		if strings.Contains(change, want) {
			return true
		}
	}
	return false
}
