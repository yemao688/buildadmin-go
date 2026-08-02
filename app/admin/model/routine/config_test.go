package routine

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	siteconfig "go-build-admin/app/common/siteconfig"
	"go-build-admin/conf"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestConfigAddAndEditRejectDuplicateNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:config-model-duplicate?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&siteconfig.Config{}))
	m := NewConfigModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/routine.config/add", nil)

	require.NoError(t, m.Add(ctx, siteconfig.Config{Name: "upload_mode"}))
	require.ErrorContains(t, m.Add(ctx, siteconfig.Config{Name: "upload_mode"}), "config name already exists")

	var first siteconfig.Config
	require.NoError(t, db.Where("name = ?", "upload_mode").First(&first).Error)
	require.NoError(t, m.Add(ctx, siteconfig.Config{Name: "upload_bucket"}))
	var second siteconfig.Config
	require.NoError(t, db.Where("name = ?", "upload_bucket").First(&second).Error)
	second.Name = first.Name
	require.ErrorContains(t, m.Edit(ctx, second), "config name already exists")
}
