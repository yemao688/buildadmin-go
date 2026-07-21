package model

import (
	"testing"

	"go-build-admin/app/pkg/header"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminLogScopeUsesCurrentAdminID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-log-scope?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AdminLog{}))
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
