package repository_test

import (
	"testing"

	adminmodel "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type AdminLog = model.AdminLog

var IsSuperAdmin = adminmodel.IsSuperAdmin

func TestAdminLogScopeUsesCurrentAdminID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-log-scope?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture table with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteAdminLogTable(db, "admin_logs"))
	for _, adminID := range []int32{1, 2, 3} {
		require.NoError(t, db.Create(&AdminLog{AdminID: adminID}).Error)
	}

	ctx, _ := gin.CreateTestContext(nil)
	ctx.Set("AdminAuth", header.AdminAuth{Id: 2, IsSuperAdmin: false})
	var logs []AdminLog
	require.NoError(t, db.Scopes(IsSuperAdmin(ctx)).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, int32(2), logs[0].AdminID)
}
