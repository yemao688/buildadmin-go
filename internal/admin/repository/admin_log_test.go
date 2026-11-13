package repository

import (
	"context"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAdminLogCreateInsertsRow(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-create")
	m := NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	log := model.AdminLog{
		AdminID:   7,
		Username:  "root",
		URL:       "/admin/auth.Admin/add",
		Title:     "管理员-新增",
		Data:      `{"name":"alice"}`,
		IP:        "203.0.113.9",
		Useragent: "test-agent",
	}
	require.NoError(t, m.Create(context.Background(), &log))

	var row model.AdminLog
	require.NoError(t, db.Last(&row).Error)
	require.Equal(t, int32(7), row.AdminID)
	require.Equal(t, "root", row.Username)
	require.Equal(t, "/admin/auth.Admin/add", row.URL)
	require.Equal(t, "管理员-新增", row.Title)
	require.JSONEq(t, `{"name":"alice"}`, row.Data)
	require.Equal(t, "203.0.113.9", row.IP)
	require.Equal(t, "test-agent", row.Useragent)
}

// 仓库只做插入：nil 入参、缺库句柄都必须返回错误而不是静默或 panic。
func TestAdminLogCreateGuardsInvalidInput(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-create-guard")
	m := NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	require.ErrorIs(t, m.Create(context.Background(), nil), gorm.ErrInvalidData)

	var nilModel *AdminLogRepository
	require.Error(t, nilModel.Create(context.Background(), &model.AdminLog{}))

	empty := &AdminLogRepository{}
	require.Error(t, empty.Create(context.Background(), &model.AdminLog{}))
}

func newAdminLogTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture table with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteAdminLogTable(db, "ba_admin_log"))
	return db
}
