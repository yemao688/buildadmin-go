package repository

import (
	"context"
	"fmt"
	"os"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// TestUserCascadeOwnersRegistration 验证手写 CascadeOwners() 注册表包含
// user_money_log 条目（变更归属时同步的对象），且字段契约正确（byColumn
// user_id、ownerColumn admin_id）。注册表可扩展：业务 fork 追加子表后仍须含
// user_money_log 且契约正确，因此不硬断言注册表长度。
func TestUserCascadeOwnersRegistration(t *testing.T) {
	db := openMySQLAdminTestDB(t, "ba_uc_")
	repo := NewUserRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_uc_"}}, data_scope.NewDenyAllEnforcer())

	owners := repo.CascadeOwners()
	require.NotEmpty(t, owners, "CascadeOwners must register at least user_money_log")
	for _, owner := range owners {
		if owner.Table != "user_money_log" {
			continue
		}
		require.Equal(t, "user_id", owner.ByColumn)
		require.Equal(t, "admin_id", owner.OwnerColumn)
		return
	}
	require.Fail(t, "CascadeOwners must register user_money_log")
}

// TestUserSyncCascadeOwnersMySQL 验证变更归属时按 CascadeOwners() 遍历同步
// 子表：user 的 admin_id 从 10 改为 20 后，user_money_log 中该 user 的流水
// admin_id 全部跟随更新；同步后校验通过，未同步时校验失败。
func TestUserSyncCascadeOwnersMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	prefix := fmt.Sprintf("ba_ucs_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	cfg.Database.Prefix = prefix

	repo := NewUserRepository(db, cfg, data_scope.NewDenyAllEnforcer())
	// 建表与注册表联动：按 CascadeOwners() 为每张注册表建最小表（列名取条目
	// 的 byColumn/ownerColumn），业务 fork 追加子表注册后遍历同步不会 1146。
	for _, owner := range repo.CascadeOwners() {
		table := owner.Table
		if table == "" {
			continue
		}
		byColumn := owner.ByColumn
		if byColumn == "" {
			byColumn = "user_id"
		}
		ownerColumn := owner.OwnerColumn
		if ownerColumn == "" {
			ownerColumn = "admin_id"
		}
		require.NoError(t, db.Exec("CREATE TABLE `"+prefix+table+"` (`id` int NOT NULL AUTO_INCREMENT, `"+byColumn+"` int NOT NULL, `"+ownerColumn+"` int NOT NULL DEFAULT 0, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error)
		t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`").Error })
	}
	// 两行流水归属 user 1，当前归属 10
	require.NoError(t, db.Exec("INSERT INTO `"+prefix+"user_money_log` (`user_id`, `admin_id`) VALUES (1, 10), (1, 10)").Error)

	require.NoError(t, repo.Transaction(context.Background(), func(tx *gorm.DB) error {
		// 归属 10 时校验通过
		if err := repo.validateUserLogOwners(tx, 1, 10); err != nil {
			return fmt.Errorf("validate before sync: %w", err)
		}
		// 同步到新归属 20
		if err := repo.syncUserLogOwners(tx, 1, 20); err != nil {
			return err
		}
		// 新归属校验通过
		if err := repo.validateUserLogOwners(tx, 1, 20); err != nil {
			return fmt.Errorf("validate after sync: %w", err)
		}
		// 旧归属校验失败（已同步）
		if err := repo.validateUserLogOwners(tx, 1, 10); err == nil {
			return fmt.Errorf("validate with stale owner must fail")
		}
		return nil
	}))

	var admins []int32
	require.NoError(t, db.Table(prefix+"user_money_log").Where("user_id = ?", 1).Pluck("admin_id", &admins).Error)
	require.Equal(t, []int32{20, 20}, admins)
}
