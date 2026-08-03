package service

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// TestAttachmentDeleteHonorsQuoteAndRemovesLocalFileAfterLastReference
// verifies the reference-count semantics of the delete flow: the first
// delete drops the count, the last delete physically removes the row and the
// local file.
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
	actor, _ := data_scope.ActorFromContext(ctx)
	config := &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}
	enforcer := data_scope.NewClosureEnforcer(config)
	m := repository.NewAttachmentRepository(db, config, enforcer)
	svc := NewRoutineAttachmentService(m, config)

	require.NoError(t, svc.Del(ctx.Request.Context(), []int32{attachment.ID}, actor))
	var remaining upload.Attachment
	require.NoError(t, db.First(&remaining, attachment.ID).Error)
	require.Equal(t, int32(1), remaining.Quote)
	_, err = os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, svc.Del(ctx.Request.Context(), []int32{attachment.ID}, actor))
	require.ErrorIs(t, db.First(&remaining, attachment.ID).Error, gorm.ErrRecordNotFound)
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}
