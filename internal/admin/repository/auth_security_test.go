package repository

import (
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/token"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type adminAuthSecurityTokenDriver struct {
	data *token.Token
}

type adminAuthSecurityRow struct {
	ID     int32  `gorm:"column:id;primaryKey"`
	Status string `gorm:"column:status"`
}

func (adminAuthSecurityRow) TableName() string { return "admins" }

func (d adminAuthSecurityTokenDriver) Set(string, string, int32, int64) error { return nil }
func (d adminAuthSecurityTokenDriver) Get(string) (*token.Token, error)       { return d.data, nil }
func (d adminAuthSecurityTokenDriver) Check(string, string, int32) bool       { return false }
func (d adminAuthSecurityTokenDriver) Delete(string) error                    { return nil }
func (d adminAuthSecurityTokenDriver) Clear(string, int32) error              { return nil }

func newAdminAuthSecurityModel(t *testing.T, tokenData *token.Token) (*AuthRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin-auth-security-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&adminAuthSecurityRow{}))
	config := &conf.Configuration{}
	return NewAuthRepository(db, &token.TokenHelper{Driver: adminAuthSecurityTokenDriver{data: tokenData}}, config), db
}

func TestAdminAuthIsLoginRejectsUserToken(t *testing.T) {
	m, db := newAdminAuthSecurityModel(t, &token.Token{Type: "user", UserID: 1})
	require.NoError(t, db.Create(&adminAuthSecurityRow{ID: 1, Status: "enable"}).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	ctx.Request.Header.Set("batoken", "user-token")

	got, ok := m.IsLogin(ctx)
	require.False(t, ok)
	require.Nil(t, got)
}
