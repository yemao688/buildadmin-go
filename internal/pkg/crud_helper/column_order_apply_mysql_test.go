package crud_helper

// MySQL 门禁集成测试：crud:apply 新增列必须落在 spec 前驱字段之后，使存量库
// 与全新安装（CREATE TABLE 按 spec 顺序）列顺序一致；既有列序漂移须以
// column-order-drift 告警暴露。

import (
	"os"
	"slices"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/testutil"

	"gorm.io/gorm"
)

func setupColOrderDB(t *testing.T) (*gorm.DB, *conf.Configuration, string) {
	t.Helper()
	db, cfg := testutil.OpenMySQL(t)
	prefix := "ba_col_order_test_"
	cfg.Database.Prefix = prefix
	table := prefix + "ops_col_order"
	logTable := prefix + "crud_log"
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error
		_ = db.Exec("DROP TABLE IF EXISTS `" + logTable + "`").Error
	})
	_ = db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error
	_ = db.Exec("DROP TABLE IF EXISTS `" + logTable + "`").Error
	if err := db.Exec("CREATE TABLE `" + logTable + "` (`id` int NOT NULL AUTO_INCREMENT, `admin_id` int NOT NULL, `table_name` varchar(200) NOT NULL, `table` blob, `fields` blob, `status` varchar(30) NOT NULL DEFAULT 'start', `comment` varchar(255), `connection` varchar(100) NOT NULL DEFAULT '', `sync` int NOT NULL DEFAULT 0, `create_time` bigint NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	return db, cfg, table
}

func writeColOrderSpec(t *testing.T) string {
	t.Helper()
	content := "name: ops_col_order\ncomment: 列序测试\ntype: alter\nfields:\n" +
		"  - name: id\n    type: bigint\n    primaryKey: true\n    autoIncrement: true\n    unsigned: true\n    designType: pk\n    comment: ID\n" +
		"  - name: image\n    type: varchar\n    length: 255\n    defaultType: EMPTY STRING\n    comment: 图片\n" +
		"  - name: images\n    type: varchar\n    length: 1000\n    defaultType: EMPTY STRING\n    comment: 图片集\n" +
		"  - name: title\n    type: varchar\n    length: 255\n    defaultType: EMPTY STRING\n    comment: 标题\n"
	path := t.TempDir() + "/ops_col_order.yaml"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func colOrderNames(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()
	var names []string
	if err := db.Raw("SELECT COLUMN_NAME FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? ORDER BY ORDINAL_POSITION", table).Scan(&names).Error; err != nil {
		t.Fatal(err)
	}
	return names
}

func TestApplyAddsColumnAtSpecPosition(t *testing.T) {
	db, cfg, table := setupColOrderDB(t)
	if err := db.Exec("CREATE TABLE `" + table + "` (`id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID', `image` varchar(255) NOT NULL DEFAULT '' COMMENT '图片', `title` varchar(255) NOT NULL DEFAULT '' COMMENT '标题', PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	path := writeColOrderSpec(t)
	results, err := ApplySpecs(db, cfg, []string{path}, ApplyOptions{AdminID: 1, SkipMenu: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != ApplyAltered {
		t.Fatalf("results = %+v", results)
	}
	if got, want := colOrderNames(t, db, table), []string{"id", "image", "images", "title"}; !slices.Equal(got, want) {
		t.Fatalf("column order = %v, want %v", got, want)
	}
	again, err := ApplySpecs(db, cfg, []string{path}, ApplyOptions{AdminID: 1, SkipMenu: true})
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Action != ApplyUnchanged {
		t.Fatalf("second apply = %s", again[0].Action)
	}
	for _, change := range again[0].Unmanaged {
		if change.Type == "column-order-drift" {
			t.Fatalf("unexpected order drift after fix: %+v", change)
		}
	}
}

func TestApplyWarnsOnColumnOrderDrift(t *testing.T) {
	db, cfg, table := setupColOrderDB(t)
	if err := db.Exec("CREATE TABLE `" + table + "` (`id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID', `title` varchar(255) NOT NULL DEFAULT '' COMMENT '标题', `image` varchar(255) NOT NULL DEFAULT '' COMMENT '图片', PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	path := writeColOrderSpec(t)
	results, err := ApplySpecs(db, cfg, []string{path}, ApplyOptions{AdminID: 1, SkipMenu: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, change := range results[0].Unmanaged {
		if change.Type == "column-order-drift" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected column-order-drift warning, got %+v", results[0].Unmanaged)
	}
}
