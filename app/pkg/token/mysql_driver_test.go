package token

import (
	"strconv"
	"testing"
	"time"

	"go-build-admin/conf"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newMysqlDriverTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:token-driver-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func newMysqlDriverTestConfig() *conf.Configuration {
	config := &conf.Configuration{}
	config.Token.Algo = "sha256"
	config.Token.Key = "test-key"
	return config
}

func TestMysqlDriverDeleteAndClearReturnStorageErrors(t *testing.T) {
	driver := NewMysqlDriver(newMysqlDriverTestDB(t), newMysqlDriverTestConfig())
	require.Error(t, driver.Delete("missing-table-token"))
	require.Error(t, driver.Clear("user", 1))
}

func TestMysqlDriverClearScopesByTypeAndUserAndDeleteInvalidatesToken(t *testing.T) {
	db := newMysqlDriverTestDB(t)
	require.NoError(t, db.AutoMigrate(&Token{}))
	driver := NewMysqlDriver(db, newMysqlDriverTestConfig())

	require.NoError(t, driver.Set("target", "user", 1, 3600))
	require.NoError(t, driver.Set("other-type", "user-refresh", 1, 3600))
	require.NoError(t, driver.Set("other-user", "user", 2, 3600))
	require.NoError(t, driver.Clear("user", 1))

	require.False(t, driver.Check("target", "user", 1))
	require.True(t, driver.Check("other-type", "user-refresh", 1))
	require.True(t, driver.Check("other-user", "user", 2))
	require.NoError(t, driver.Delete("other-type"))
	require.False(t, driver.Check("other-type", "user-refresh", 1))
}
