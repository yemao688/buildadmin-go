package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buildadmin-go/internal/pkg/testutil"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// mustCobraCommand 提供 Sync 打印输出所需的最小 cobra.Command。
func mustCobraCommand(t *testing.T) *cobra.Command {
	t.Helper()
	return &cobra.Command{Use: "test"}
}

// writeSyncSpec 向 specsDir 写入一个含 inheritFrom 声明的最小 spec。
func writeSyncSpec(t *testing.T, specsDir, name, parent, byColumn string) {
	t.Helper()
	spec := "name: " + name + "\ncomment: test\n" +
		"dataScope:\n  mode: auto\n  inheritFrom:\n    table: " + parent + "\n    byColumn: " + byColumn + "\n" +
		"fields:\n" +
		"  - name: id\n    type: bigint\n    unsigned: true\n    primaryKey: true\n    autoIncrement: true\n    null: false\n    comment: ID\n" +
		"  - name: " + byColumn + "\n    type: bigint\n    unsigned: true\n    null: false\n    comment: 关联主表\n" +
		"  - name: admin_id\n    type: bigint\n    unsigned: true\n    null: false\n    comment: 管理员ID\n"
	if err := os.WriteFile(filepath.Join(specsDir, name+".yaml"), []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestRunCascadeJobReconcilesMySQL 集成测试（受 mysql_test 门禁，未配置时
// t.Skip）：构造 parent/child 表与不一致行，验证 runCascadeJob 按规格 SQL
// 修复、幂等且不触碰已一致/无主记录的行（归属列固定 admin_id）。
func TestRunCascadeJobReconcilesMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	prefix := "ba_cascade_sync_test_"
	cfg.Database.Prefix = prefix

	tables := []string{prefix + "user", prefix + "user_money_log"}
	for _, table := range tables {
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, table := range tables {
			_ = db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error
		}
	})

	statements := []string{
		"CREATE TABLE `" + prefix + "user` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `admin_id` bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE TABLE `" + prefix + "user_money_log` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `user_id` bigint unsigned NOT NULL DEFAULT 0, `admin_id` bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	// parent: user(id=1 admin 10, id=2 admin 20, id=3 admin 30)
	if err := db.Exec("INSERT INTO `" + prefix + "user` (`id`, `admin_id`) VALUES (1, 10), (2, 20), (3, 30)").Error; err != nil {
		t.Fatal(err)
	}
	// child: id1 一致 / id2 不一致→10 / id3 一致 / id4 不一致→20 / id5 无主记录保持 / id6 主记录存在
	if err := db.Exec("INSERT INTO `" + prefix + "user_money_log` (`id`, `user_id`, `admin_id`) VALUES (1, 1, 10), (2, 1, 99), (3, 2, 20), (4, 2, 5), (5, 9, 7), (6, 3, 30)").Error; err != nil {
		t.Fatal(err)
	}

	child := ChildRef{ChildTable: "user_money_log", ByColumn: "user_id"}
	affected, err := runCascadeJob(db, cfg, "user", child)
	if err != nil {
		t.Fatalf("runCascadeJob() error: %v", err)
	}
	if affected != 2 {
		t.Fatalf("rows affected = %d, want 2 (rows id=2 and id=4)", affected)
	}

	var bad int64
	if err := db.Raw("SELECT COUNT(*) FROM `" + prefix + "user_money_log` WHERE `admin_id` <> (SELECT `admin_id` FROM `" + prefix + "user` WHERE `id` = `" + prefix + "user_money_log`.`user_id`)").Scan(&bad).Error; err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("remaining inconsistent rows = %d, want 0", bad)
	}

	// 幂等：第二次执行不再有变化。
	again, err := runCascadeJob(db, cfg, "user", child)
	if err != nil {
		t.Fatalf("runCascadeJob() second run error: %v", err)
	}
	if again != 0 {
		t.Fatalf("second run rows affected = %d, want 0 (idempotent)", again)
	}
}

// TestRunCascadeJobMissingParentMySQL 父表缺失时返回语义化错误而非裸 1146：
// 只建子表不建父表，runCascadeJob 必须在执行 UPDATE 之前 fail-closed 报错，
// 错误包含 "missing"、父表名与子表名。
func TestRunCascadeJobMissingParentMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	prefix := "ba_cascade_missing_"
	cfg.Database.Prefix = prefix

	childTable := prefix + "user_money_log"
	if err := db.Exec("DROP TABLE IF EXISTS `" + childTable + "`").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS `" + childTable + "`").Error
	})
	if err := db.Exec("CREATE TABLE `" + childTable + "` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `user_id` bigint unsigned NOT NULL DEFAULT 0, `admin_id` bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	// 父表 ba_cascade_missing_user 故意不创建，模拟主实体已删、子表声明悬空。

	child := ChildRef{ChildTable: "user_money_log", ByColumn: "user_id"}
	_, err := runCascadeJob(db, cfg, "user", child)
	if err == nil {
		t.Fatal("runCascadeJob() error = nil, want missing-parent semantic error")
	}
	for _, part := range []string{"missing", prefix + "user", prefix + "user_money_log", "crud:apply"} {
		if !strings.Contains(err.Error(), part) {
			t.Fatalf("runCascadeJob() error = %v, want it to contain %q", err, part)
		}
	}
}

// TestCascadeHandlerSyncInvalidTargetMySQL 验证指定 table 参数在无任何子表
// spec 声明 inheritFrom 指向它时报错退出（临时 specs 目录为空）。
func TestCascadeHandlerSyncInvalidTargetMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = "ba_cascade_sync_miss_"

	h := &CascadeHandler{db: db, config: cfg, specsDir: t.TempDir()}
	// 无任何声明 → 明确报错。
	if err := h.Sync(mustCobraCommand(t), []string{"user"}); err == nil {
		t.Fatal("Sync with missing target: error = nil, want error")
	}
	// 有声明但目标不是任一父表 → 明确报错。
	writeSyncSpec(t, h.specsDir, "user_money_log", "user_ok", "user_id")
	if err := h.Sync(mustCobraCommand(t), []string{"user"}); err == nil {
		t.Fatal("Sync with target having no referrers: error = nil, want error")
	}
}

// TestCascadeSyncSkipsMissingTablesMySQL 集成回归：specs 目录中的声明指向
// 尚未物化（表未建）的父表/子表时，cascade:sync 语义化报错或跳过而非裸
// 1146——父表缺失是声明悬空（fail-closed 报错），子表缺失同样明确报错。
// 健康模块 user_ok 正常对账。
func TestCascadeSyncSkipsMissingTablesMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	prefix := "ba_cascade_sync_del_"
	cfg.Database.Prefix = prefix

	tables := []string{"user_ok", "user_ok_money_log"}
	for _, table := range tables {
		if err := db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, table := range tables {
			_ = db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`").Error
		}
	})
	// user_ok：健康模块，子表带一条不一致行。
	if err := db.Exec("CREATE TABLE `" + prefix + "user_ok` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `admin_id` bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE `" + prefix + "user_ok_money_log` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `user_id` bigint unsigned NOT NULL DEFAULT 0, `admin_id` bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO `" + prefix + "user_ok` (`id`, `admin_id`) VALUES (1, 10)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO `" + prefix + "user_ok_money_log` (`id`, `user_id`, `admin_id`) VALUES (1, 1, 99)").Error; err != nil {
		t.Fatal(err)
	}

	h := &CascadeHandler{db: db, config: cfg, specsDir: t.TempDir()}
	// 声明指向未建表的父表（user_deleted 未物化）→ 全量 sync fail-closed 报错
	writeSyncSpec(t, h.specsDir, "user_deleted_money_log", "user_deleted", "user_id")
	var buf bytes.Buffer
	cmd := mustCobraCommand(t)
	cmd.SetOut(&buf)
	syncErr := h.Sync(cmd, nil)
	if syncErr == nil {
		t.Fatal("full cascade:sync with missing parent: error = nil, want error")
	}
	if !strings.Contains(syncErr.Error(), "missing") {
		t.Fatalf("full sync error = %v, want missing-parent error", syncErr)
	}

	// 仅健康声明 → 正常对账并修复
	if err := os.Remove(filepath.Join(h.specsDir, "user_deleted_money_log.yaml")); err != nil {
		t.Fatal(err)
	}
	writeSyncSpec(t, h.specsDir, "user_ok_money_log", "user_ok", "user_id")
	buf.Reset()
	if err := h.Sync(cmd, nil); err != nil {
		t.Fatalf("full cascade:sync with healthy module failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, prefix+"user_ok_money_log: 1 rows reconciled") {
		t.Fatalf("sync output missing user_ok reconcile line:\n%s", output)
	}
	var bad int64
	if err := db.Raw("SELECT COUNT(*) FROM `" + prefix + "user_ok_money_log` WHERE `admin_id` <> (SELECT `admin_id` FROM `" + prefix + "user_ok` WHERE `id` = `" + prefix + "user_ok_money_log`.`user_id`)").Scan(&bad).Error; err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("user_ok remaining inconsistent rows = %d, want 0", bad)
	}
}

// TestCascadeSyncInheritFromAggregationMySQL 规格主场景：parent(user) +
// child(money_log) + child2(order_recharge) 三表，两个子表都在 specs 目录中
// 声明 inheritFrom user，带不一致行。验证：全量 cascade:sync 两个子表都
// 修复；指定 cascade:sync user 同样修复两表；无声明表指定时报错退出。
func TestCascadeSyncInheritFromAggregationMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	prefix := "ba_cascade_sync_agg_"
	cfg.Database.Prefix = prefix

	tables := []string{"user", "user_money_log", "order_recharge"}
	for _, table := range tables {
		if err := db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, table := range tables {
			_ = db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`").Error
		}
	})

	statements := []string{
		"CREATE TABLE `" + prefix + "user` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `admin_id` bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE TABLE `" + prefix + "user_money_log` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `user_id` bigint unsigned NOT NULL DEFAULT 0, `admin_id` bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE TABLE `" + prefix + "order_recharge` (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `user_id` bigint unsigned NOT NULL DEFAULT 0, `admin_id` bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Exec("INSERT INTO `" + prefix + "user` (`id`, `admin_id`) VALUES (1, 10), (2, 20)").Error; err != nil {
		t.Fatal(err)
	}
	// money_log：id1 不一致→10 / id2 一致 / id3 不一致→20；order：id1 不一致→10 / id2 一致。
	if err := db.Exec("INSERT INTO `" + prefix + "user_money_log` (`id`, `user_id`, `admin_id`) VALUES (1, 1, 99), (2, 1, 10), (3, 2, 5)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO `" + prefix + "order_recharge` (`id`, `user_id`, `admin_id`) VALUES (1, 1, 77), (2, 2, 20)").Error; err != nil {
		t.Fatal(err)
	}

	h := &CascadeHandler{db: db, config: cfg, specsDir: t.TempDir()}
	writeSyncSpec(t, h.specsDir, "user_money_log", "user", "user_id")
	writeSyncSpec(t, h.specsDir, "order_recharge", "user", "user_id")

	// 全量：两个子表都修复。
	var buf bytes.Buffer
	cmd := mustCobraCommand(t)
	cmd.SetOut(&buf)
	if err := h.Sync(cmd, nil); err != nil {
		t.Fatalf("full cascade:sync error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{
		prefix + "user → " + prefix + "user_money_log via user_id",
		prefix + "user → " + prefix + "order_recharge via user_id",
		prefix + "user_money_log: 2 rows reconciled",
		prefix + "order_recharge: 1 rows reconciled",
		"reconciled 3 rows across 2 tables",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("full sync output missing %q:\n%s", want, output)
		}
	}
	assertNoInconsistentCascadeRows(t, db, prefix, "user", []string{"user_money_log", "order_recharge"})

	// 重置为不一致，指定 cascade:sync user 同样修复两表。
	if err := db.Exec("UPDATE `" + prefix + "user_money_log` SET `admin_id` = 1234").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE `" + prefix + "order_recharge` SET `admin_id` = 4321").Error; err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := h.Sync(cmd, []string{"user"}); err != nil {
		t.Fatalf("targeted cascade:sync user error: %v", err)
	}
	output = buf.String()
	if !strings.Contains(output, prefix+"user_money_log: 3 rows reconciled") ||
		!strings.Contains(output, prefix+"order_recharge: 2 rows reconciled") {
		t.Fatalf("targeted sync output unexpected:\n%s", output)
	}
	assertNoInconsistentCascadeRows(t, db, prefix, "user", []string{"user_money_log", "order_recharge"})

	// 无声明表指定 → 报错退出。
	if err := h.Sync(mustCobraCommand(t), []string{"no_such_parent"}); err == nil {
		t.Fatal("targeted cascade:sync no_such_parent: error = nil, want error")
	}
}

// assertNoInconsistentCascadeRows 断言指定子表已无与主表 admin_id 不一致的行
// （本测试场景中所有子表均继承自同一主表 parent，关联列固定 user_id）。
func assertNoInconsistentCascadeRows(t *testing.T, db *gorm.DB, prefix, parent string, children []string) {
	t.Helper()
	for _, child := range children {
		var bad int64
		stmt := "SELECT COUNT(*) FROM `" + prefix + child + "` WHERE `admin_id` <> (SELECT `admin_id` FROM `" + prefix + parent + "` WHERE `id` = `" + prefix + child + "`.`user_id`)"
		if err := db.Raw(stmt).Scan(&bad).Error; err != nil {
			t.Fatal(err)
		}
		if bad != 0 {
			t.Fatalf("%s remaining inconsistent rows = %d, want 0", child, bad)
		}
	}
}
