package crud_helper

// MySQL 门禁集成测试：generate 路径（HandleTableDesign + syncSpecIndexes）
// 对已有表补建 spec 声明的缺失索引——与 apply 路径行为一致，防止开发库
// 与部署库形状分叉（v3.0.5 修复的 F1）。

import (
	"testing"

	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"
)

func TestGeneratePathSyncsMissingIndexes(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = idxApplyTestPrefix
	table := idxApplyTestPrefix + "ops_generate_index"
	db.Exec("DROP TABLE IF EXISTS `" + table + "`")
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error })
	if err := db.Exec("CREATE TABLE `" + table + "` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `order_no` varchar(64) NOT NULL DEFAULT '', PRIMARY KEY (`id`)) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}

	fields := []crudmodel.Field{{Name: "id", Type: "bigint", PrimaryKey: true}, {Name: "order_no", Type: "varchar"}}
	tableSpec := crudmodel.Table{
		Name: "ops_generate_index",
		Indexes: []crudmodel.IndexSpec{
			{Name: "uk_order_no", Unique: true, Columns: []string{"order_no"}},
		},
	}

	// 模拟 crud:generate 的 alter 路径：HandleTableDesign（列变更，无索引）+ syncSpecIndexes（补索引）。
	if err := HandleTableDesign(db, table, tableSpec, fields); err != nil {
		t.Fatalf("HandleTableDesign: %v", err)
	}
	warnings, err := syncSpecIndexes(db, table, tableSpec, fields)
	if err != nil {
		t.Fatalf("syncSpecIndexes: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	assertIndexOnDB(t, db, table, "uk_order_no", true)

	// 幂等：再次同步不产生新增、不报错。
	warnings, err = syncSpecIndexes(db, table, tableSpec, fields)
	if err != nil {
		t.Fatalf("second syncSpecIndexes: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("second sync warnings: %v", warnings)
	}
}
