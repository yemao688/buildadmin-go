package service

import (
	"context"
	"testing"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// newSecurityRuleSQLiteFixture 提供两张安全规则表的 sqlite 副本，用于验证
// 删除守卫与 repo 委托，不依赖 MySQL 门禁。
func newSecurityRuleSQLiteFixture(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:security-rules?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS security_data_recycle",
		"DROP TABLE IF EXISTS security_sensitive_data",
		"CREATE TABLE security_data_recycle (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT '', controller TEXT NOT NULL DEFAULT '', controller_as TEXT NOT NULL DEFAULT '', data_table TEXT NOT NULL DEFAULT '', primary_key TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '1', connection TEXT NOT NULL DEFAULT '', update_time INTEGER DEFAULT NULL, create_time INTEGER DEFAULT NULL)",
		"CREATE TABLE security_sensitive_data (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT '', controller TEXT NOT NULL DEFAULT '', controller_as TEXT NOT NULL DEFAULT '', data_table TEXT NOT NULL DEFAULT '', primary_key TEXT NOT NULL DEFAULT '', data_fields TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '1', connection TEXT NOT NULL DEFAULT '', update_time INTEGER DEFAULT NULL, create_time INTEGER DEFAULT NULL)",
		"INSERT INTO security_data_recycle (id,name) VALUES (1,'custom'),(5,'会员')",
		"INSERT INTO security_sensitive_data (id,name) VALUES (1,'custom'),(2,'会员数据')",
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}
	config := &conf.Configuration{}
	return db, config
}

func TestSecuritySeedRuleCannotBeDeleted(t *testing.T) {
	t.Parallel()
	// nil repo 也可：seed 守卫在触达 repo 之前返回。
	require.ErrorContains(t, (&SecurityDataRecycleService{}).Del(context.Background(), []int32{1, 5}), "framework seed rule cannot be deleted")
	require.ErrorContains(t, (&SensitiveDataService{}).Del(context.Background(), []int32{2}), "framework seed rule cannot be deleted")

	db, config := newSecurityRuleSQLiteFixture(t)
	recycleSvc := NewSecurityDataRecycleService(adminmodel.NewSecurityDataRecycleRepository(db, config))
	sensitiveSvc := NewSensitiveDataService(adminmodel.NewSecuritySensitiveDataRepository(db, config))
	ctx := context.Background()

	require.ErrorContains(t, recycleSvc.Del(ctx, []int32{5}), "framework seed rule cannot be deleted")
	require.NoError(t, recycleSvc.Del(ctx, []int32{1}))
	var recycleRows, sensitiveRows int64
	require.NoError(t, db.Table("security_data_recycle").Where("id = 5").Count(&recycleRows).Error)
	require.Equal(t, int64(1), recycleRows)
	require.NoError(t, db.Table("security_data_recycle").Where("id = 1").Count(&recycleRows).Error)
	require.Zero(t, recycleRows)

	require.ErrorContains(t, sensitiveSvc.Del(ctx, []int32{2}), "framework seed rule cannot be deleted")
	require.NoError(t, sensitiveSvc.Del(ctx, []int32{1}))
	require.NoError(t, db.Table("security_sensitive_data").Where("id = 2").Count(&sensitiveRows).Error)
	require.Equal(t, int64(1), sensitiveRows)
	require.NoError(t, db.Table("security_sensitive_data").Where("id = 1").Count(&sensitiveRows).Error)
	require.Zero(t, sensitiveRows)
}
