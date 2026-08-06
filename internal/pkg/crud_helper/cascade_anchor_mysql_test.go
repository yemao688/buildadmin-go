package crud_helper

import (
	"fmt"
	"os"
	"testing"

	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

// TestValidateInheritParentMySQL 验证 inheritFrom 主实体预校验：主实体无成功
// 生成记录或记录未声明 reassignable 时拒绝，合法记录通过。
func TestValidateInheritParentMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	prefix := fmt.Sprintf("ba_cas_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{TablePrefix: prefix}
	cfg.Database.Prefix = prefix

	require.NoError(t, db.Exec("CREATE TABLE `"+prefix+"crud_log` (`id` int NOT NULL AUTO_INCREMENT, `admin_id` int NOT NULL, `table_name` varchar(200) NOT NULL, `table` blob, `fields` blob, `status` varchar(30) NOT NULL DEFAULT 'start', `comment` varchar(255), `connection` varchar(100) NOT NULL DEFAULT '', `sync` int NOT NULL DEFAULT 0, `create_time` bigint NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + prefix + "crud_log`").Error })

	// 带字段与归属声明的日志（硬契约校验需要真实 PK/owner 形状）。
	insertLogWith := func(tableName string, ds *data_scope.Config, fields []crudmodel.Field) {
		table := crudmodel.Table{Name: tableName, DataScope: ds}
		record := crudmodel.Log{AdminID: 1, Tablename: tableName, Table: crudmodel.JSON_TABLE(table), Fields: crudmodel.JSON_FIELDS(fields), Status: "success"}
		require.NoError(t, db.Table(prefix+"crud_log").Create(&record).Error)
	}
	idPK := []crudmodel.Field{{Name: "id", Type: "int", PrimaryKey: true}, {Name: "admin_id", Type: "int"}}
	assignOnCreate := true

	// 无记录 → 拒绝
	err = validateInheritParent(db, cfg, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no successful CRUD generation record")

	// 记录未声明 reassignable → 拒绝
	insertLogWith("seller_user", &data_scope.Config{Mode: data_scope.ModeAuto}, idPK)
	err = validateInheritParent(db, cfg, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reassignable=true")

	// 记录声明 reassignable → 通过（auto 模式 ownerColumn 为空，视为 admin_id）
	insertLogWith("seller_user", &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}, idPK)
	require.NoError(t, validateInheritParent(db, cfg, "seller_user"))

	// 硬契约：owner 列非 admin_id → 拒绝
	insertLogWith("seller_user", &data_scope.Config{Mode: data_scope.ModeRequired, OwnerColumn: "agent_id", AssignOnCreate: &assignOnCreate, Reassignable: true}, idPK)
	err = validateInheritParent(db, cfg, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), `owner column "agent_id"`)
	require.Contains(t, err.Error(), "requires admin_id")

	// 硬契约：主键非 id → 拒绝
	nonIDPK := []crudmodel.Field{{Name: "user_id", Type: "int", PrimaryKey: true}, {Name: "admin_id", Type: "int"}}
	insertLogWith("seller_user", &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}, nonIDPK)
	err = validateInheritParent(db, cfg, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "primary key must be exactly id")

	// 合法记录（最后一条是合法形状）→ 通过
	insertLogWith("seller_user", &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}, idPK)
	require.NoError(t, validateInheritParent(db, cfg, "seller_user"))
}

// TestFindInheritReferrersMySQL 验证 inbound inheritFrom 引用扫描：无引用→空；
// 有子表声明指向主表→列出（含多子表排序）；主实体自身记录不计入。
func TestFindInheritReferrersMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	prefix := fmt.Sprintf("ba_casr_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{TablePrefix: prefix}
	cfg.Database.Prefix = prefix

	require.NoError(t, db.Exec("CREATE TABLE `"+prefix+"crud_log` (`id` int NOT NULL AUTO_INCREMENT, `admin_id` int NOT NULL, `table_name` varchar(200) NOT NULL, `table` blob, `fields` blob, `status` varchar(30) NOT NULL DEFAULT 'start', `comment` varchar(255), `connection` varchar(100) NOT NULL DEFAULT '', `sync` int NOT NULL DEFAULT 0, `create_time` bigint NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + prefix + "crud_log`").Error })

	insertLog := func(tableName string, inherit string) {
		ds := &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: inherit == ""}
		if inherit != "" {
			ds = &data_scope.Config{Mode: data_scope.ModeAuto, InheritFrom: &data_scope.InheritRef{Table: inherit, ByColumn: "user_id"}}
		}
		table := crudmodel.Table{Name: tableName, DataScope: ds}
		record := crudmodel.Log{AdminID: 1, Tablename: tableName, Table: crudmodel.JSON_TABLE(table), Status: "success"}
		require.NoError(t, db.Table(prefix+"crud_log").Create(&record).Error)
	}

	// 无任何引用 → 空
	insertLog("seller_user", "")
	referrers, err := findInheritReferrers(db, cfg, "seller_user")
	require.NoError(t, err)
	require.Empty(t, referrers)

	// 主实体自身记录（reassignable，无 inheritFrom）不计入引用
	insertLog("seller_user_child_a", "seller_user")
	referrers, err = findInheritReferrers(db, cfg, "seller_user")
	require.NoError(t, err)
	require.Equal(t, []string{"seller_user_child_a"}, referrers)

	// 多子表 + 确定性排序；指向其它父表的子表不计数
	insertLog("seller_user_child_b", "seller_user")
	insertLog("order_user_child", "order_user")
	referrers, err = findInheritReferrers(db, cfg, "seller_user")
	require.NoError(t, err)
	require.Equal(t, []string{"seller_user_child_a", "seller_user_child_b"}, referrers)

	// 指向未变但被 delete 消费后的记录不计数：最新记录为 delete 时排除
	require.NoError(t, db.Table(prefix+"crud_log").Where("table_name = ?", "seller_user_child_b").Update("status", "delete").Error)
	referrers, err = findInheritReferrers(db, cfg, "seller_user")
	require.NoError(t, err)
	require.Equal(t, []string{"seller_user_child_a"}, referrers)

	// 评审 blocker 回归：重新生成后再删除的模块不得被旧 success 行复活。
	// crud:delete 只把最新一条 success 行翻转为 delete（updateCrudStatus 按
	// log.ID），更早的 success 行仍存在——按"最新行去重后再过滤 success"的
	// 语义必须排除该表（旧实现先过滤 status 再取最新，会选中陈旧行）。
	insertLog("seller_user_child_c", "seller_user") // 第一次生成（旧行）
	insertLog("seller_user_child_c", "seller_user") // 重新生成（最新行）
	require.NoError(t, db.Table(prefix+"crud_log").Where("table_name = ? AND status = ?", "seller_user_child_c", "success").Order("id desc").Limit(1).Update("status", "delete").Error)
	referrers, err = findInheritReferrers(db, cfg, "seller_user")
	require.NoError(t, err)
	require.Equal(t, []string{"seller_user_child_a"}, referrers)
	require.NotContains(t, referrers, "seller_user_child_c")
}
