package siteconfig

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// newCacheTestService 构造带 ba_ 前缀与 basics/mail 两组的 Service。dbName
// 需在测试间唯一——sqlite 命名内存库在同一进程内共享。
func newCacheTestService(t *testing.T, dbName string) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	// 实体携带 MySQL 专用类型 tag（int unsigned、longtext 等），sqlite 无法
	// AutoMigrate，使用 sqlite 原生 DDL 建 fixture 表。
	require.NoError(t, testutil.CreateSQLiteConfigTable(db, "ba_config"))
	require.NoError(t, db.Table("ba_config").Create(&[]Config{
		{Name: "site_name", Group: "basics", Value: "站点名称", Weigh: 99},
		{Name: "site_url", Group: "basics", Value: "https://example.com", Weigh: 98},
		{Name: "smtp_server", Group: "mail", Value: "smtp.example.com", Weigh: 0},
	}).Error)
	return NewService(db), db
}

func cacheTestRequest(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c
}

// TestGetKVByGroupCache 验证 H2 进程内缓存：首次查库、二次走缓存、显式
// 失效后重查、分组隔离与副本防污染。
func TestGetKVByGroupCache(t *testing.T) {
	s, db := newCacheTestService(t, "siteconfig-cache")

	kv, err := s.GetKVByGroup(cacheTestRequest(t), "basics")
	require.NoError(t, err)
	require.Equal(t, "站点名称", kv["site_name"])

	// 直改库（绕过服务写路径）：缓存未失效时仍返回旧快照
	require.NoError(t, db.Table("ba_config").Where("name = ?", "site_name").Update("value", "新站点").Error)
	kv, err = s.GetKVByGroup(cacheTestRequest(t), "basics")
	require.NoError(t, err)
	require.Equal(t, "站点名称", kv["site_name"])

	// 分组隔离：未查询过的 mail 组不受 basics 缓存影响，直接走库
	mail, err := s.GetKVByGroup(cacheTestRequest(t), "mail")
	require.NoError(t, err)
	require.Equal(t, "smtp.example.com", mail["smtp_server"])
	// 返回副本，调用方修改不影响后续读取
	mail["smtp_server"] = "mutated"
	fresh, err := s.GetKVByGroup(cacheTestRequest(t), "mail")
	require.NoError(t, err)
	require.Equal(t, "smtp.example.com", fresh["smtp_server"])

	// 显式失效后重查，命中最新数据
	s.InvalidateSiteConfigCache()
	kv, err = s.GetKVByGroup(cacheTestRequest(t), "basics")
	require.NoError(t, err)
	require.Equal(t, "新站点", kv["site_name"])

	// 缓存命中时返回副本，调用方修改不影响后续读取
	kv["site_name"] = "mutated"
	fresh, err = s.GetKVByGroup(cacheTestRequest(t), "basics")
	require.NoError(t, err)
	require.Equal(t, "新站点", fresh["site_name"])
}

// TestGetKVByGroupCacheTTLExpiry 验证 TTL 过期后自动重查。
func TestGetKVByGroupCacheTTLExpiry(t *testing.T) {
	s, db := newCacheTestService(t, "siteconfig-cache-ttl")

	originalTTL := siteconfigTTL
	siteconfigTTL = time.Millisecond
	defer func() { siteconfigTTL = originalTTL }()

	kv, err := s.GetKVByGroup(cacheTestRequest(t), "basics")
	require.NoError(t, err)
	require.Equal(t, "站点名称", kv["site_name"])

	require.NoError(t, db.Table("ba_config").Where("name = ?", "site_name").Update("value", "新站点").Error)
	time.Sleep(5 * time.Millisecond) // 越过 TTL
	kv, err = s.GetKVByGroup(cacheTestRequest(t), "basics")
	require.NoError(t, err)
	require.Equal(t, "新站点", kv["site_name"])
}

// TestGetKVByGroupCacheBypassInTransaction 验证请求事务内的读取绕过缓存：
// 事务内可能看到未提交数据，不能入缓存；事务回滚后全局缓存保持旧快照。
func TestGetKVByGroupCacheBypassInTransaction(t *testing.T) {
	s, db := newCacheTestService(t, "siteconfig-cache-tx")

	// 预热缓存（旧快照）
	_, err := s.GetKVByGroup(cacheTestRequest(t), "basics")
	require.NoError(t, err)

	err = db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, tx.Table("ba_config").Where("name = ?", "site_name").Update("value", "事务内值").Error)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(requesttx.Bind(context.Background(), tx))
		kv, err := s.GetKVByGroup(c, "basics")
		require.NoError(t, err)
		require.Equal(t, "事务内值", kv["site_name"])
		return errors.New("rollback")
	})
	require.Error(t, err)

	// 回滚后：非事务读命中缓存旧快照，未混入未提交数据
	kv, err := s.GetKVByGroup(cacheTestRequest(t), "basics")
	require.NoError(t, err)
	require.Equal(t, "站点名称", kv["site_name"])
}
