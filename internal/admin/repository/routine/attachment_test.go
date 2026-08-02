package routine

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/utils"
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

func TestAttachmentDeleteHonorsQuoteAndRemovesLocalFileAfterLastReference(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "attachment_quote.db")), &gorm.Config{
		NamingStrategy:                           schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteAttachmentTables(db, "ba_attachment", "ba_admin", "ba_user"))

	url := "/storage/quote-tests/" + strings.ReplaceAll(t.Name(), "/", "_") + ".txt"
	path := filepath.Join(utils.RootPath(), "public", strings.TrimLeft(url, "/"))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte("shared"), 0600))
	t.Cleanup(func() { _ = os.Remove(path) })

	attachment := upload.Attachment{Topic: "test", AdminID: 1, UserID: 1, URL: url, Name: "shared.txt", Mimetype: "text/plain", Storage: "local", Sha1: "quote-test", Quote: 2}
	require.NoError(t, db.Create(&attachment).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("DELETE", "/admin/routine.attachment/del", nil)
	require.NoError(t, data_scope.SetActor(ctx, data_scope.Actor{AdminID: 1, Unrestricted: true}))
	m := NewAttachmentRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, data_scope.NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: "ba_"}}))

	require.NoError(t, m.Del(ctx, []int32{attachment.ID}))
	var remaining upload.Attachment
	require.NoError(t, db.First(&remaining, attachment.ID).Error)
	require.Equal(t, int32(1), remaining.Quote)
	_, err = os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, m.Del(ctx, []int32{attachment.ID}))
	require.ErrorIs(t, db.First(&remaining, attachment.ID).Error, gorm.ErrRecordNotFound)
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}
