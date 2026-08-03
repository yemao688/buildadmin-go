package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/pkg/testutil"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// 回归测试：Attachment 的 Admin/User 关联必须解析到真实的（带前缀）admin/user 表，
// 而不是按结构体名推导出不存在的 attachment_admin/attachment_user 表。
func TestAttachmentAssociationTableNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		NamingStrategy:                           schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteAttachmentTables(db, "ba_attachment", "ba_admin", "ba_user"))
	require.True(t, db.Migrator().HasTable("ba_admin"))
	require.True(t, db.Migrator().HasTable("ba_user"))

	require.NoError(t, db.Exec("INSERT INTO ba_admin (id, username, nickname) VALUES (1, 'root', 'Root')").Error)
	require.NoError(t, db.Exec("INSERT INTO ba_user (id, username, nickname) VALUES (2, 'member', 'Member')").Error)
	require.NoError(t, db.Exec("INSERT INTO ba_attachment (id, topic, admin_id, user_id, url, width, height, name, size, mimetype, quote, storage, sha1, create_time, last_upload_time) VALUES (1, 't', 1, 2, '/u', 1, 1, 'n', 1, 'image/png', 0, 'local', 's', 1, 1)").Error)

	// 与 AttachmentRepository.List 相同的查询链（Preload + Joins）。
	var list []*upload.Attachment
	err = db.Table("ba_attachment AS attachment").Model(&upload.Attachment{}).
		Preload("Admin").Preload("User").
		Joins("Admin").Joins("User").
		Find(&list).Error
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "root", list[0].Admin.Username)
	require.Equal(t, "member", list[0].User.Username)
}


