package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

// attachmentConfigSQLCounter 统计命中的 ba_config SELECT 次数，用于断言
// 列表 alioss 路径的 settings 读取是单次批量查询而非逐行 N+1。
type attachmentConfigSQLCounter struct {
	logger.Interface
	configSelects int
}

func (l *attachmentConfigSQLCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if strings.Contains(sql, "ba_config") && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		l.configSelects++
	}
}

// TestAttachmentListAliossSettingsBatchedOnce 覆盖列表批处理路径：3 行
// alioss 附件只允许 1 次 ba_config（group='upload'）查询（原先每行 1 次 =
// 3 次），且 FullUrl/Suffix 与单行路径 DealData 逐字段一致。
func TestAttachmentListAliossSettingsBatchedOnce(t *testing.T) {
	counter := &attachmentConfigSQLCounter{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(sqlite.Open("file:attachment-alioss-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
		Logger:         counter,
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteAttachmentTables(db, "ba_attachment", "ba_admin", "ba_user"))
	require.NoError(t, testutil.CreateSQLiteConfigTable(db, "ba_config"))

	// alioss 上传配置（固定 group='upload'，与行数据无关）。
	require.NoError(t, db.Exec(`INSERT INTO ba_config (name, "group", value) VALUES
		('upload_mode','upload','alioss'),
		('upload_bucket','upload','demo'),
		('upload_url','upload','oss-cn-hangzhou'),
		('upload_cdn_url','upload','https://cdn.example.com')`).Error)
	require.NoError(t, db.Exec("INSERT INTO ba_admin (id, username, nickname) VALUES (1, 'root', 'Root')").Error)
	require.NoError(t, db.Exec("INSERT INTO ba_user (id, username, nickname) VALUES (2, 'member', 'Member')").Error)
	for i := 1; i <= 3; i++ {
		require.NoError(t, db.Exec(`INSERT INTO ba_attachment (id, admin_id, user_id, topic, url, quote, storage, create_time) VALUES (?, 1, 2, 't', ?, 1, 'alioss', 1)`, i, fmt.Sprintf("/storage/a%d.png", i)).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO ba_attachment (id, admin_id, user_id, topic, url, quote, storage, create_time) VALUES (4, 1, 2, 't', '/storage/local.png', 1, 'local', 1)`).Error)

	m := NewAttachmentRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}},
		data_scope.NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: "ba_"}}))
	ctx := adminTestContext(t, true)

	list, total, err := m.List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, list, 4)

	// 3 行 alioss 只允许 1 次 ba_config 查询（原先每行 1 次 = 3 次）。
	require.Equal(t, 1, counter.configSelects, "expected one batched settings query")

	// 边界：空列表不触发 settings 查询。
	require.NoError(t, m.DealDataList(ctx, nil))
	require.Equal(t, 1, counter.configSelects, "empty list must not query settings")

	// FullUrl/Suffix 与单行路径 DealData 逐字段一致（同一访问路径）。
	for _, v := range list {
		ref, err := m.DealData(ctx, &upload.Attachment{Storage: v.Storage, URL: v.URL})
		require.NoError(t, err)
		require.Equal(t, ref.FullUrl, v.FullUrl, "row %d (%s)", v.ID, v.URL)
		require.Equal(t, ref.Suffix, v.Suffix, "row %d (%s)", v.ID, v.URL)
	}
}


