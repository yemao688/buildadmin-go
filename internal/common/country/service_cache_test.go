package country

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/i18n"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// newCacheTestService 构造带 dw_ 前缀与 zh/en 两种语言、site.title 双语文案
// 的 Service（zh 为默认语言，weigh 更大）。dbName 需在测试间唯一——sqlite
// 命名内存库在同一进程内共享。
func newCacheTestService(t *testing.T, dbName string) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "dw_"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Table("dw_country_language").AutoMigrate(&Language{}))
	require.NoError(t, db.Table("dw_country_language_content").AutoMigrate(&LanguageContent{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX `uk_country_language_lan` ON `dw_country_language` (`lan`)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX `uk_country_language_content_lan_group_key` ON `dw_country_language_content` (`lan`, `group`, `key`)").Error)

	s := NewService(db, &conf.Configuration{Database: conf.Database{Prefix: "dw_"}})
	ctx := context.Background()
	require.NoError(t, db.Table("dw_country_language").Create(&[]Language{
		{Lan: "zh-cn", Name: "简体中文", Status: 1, Weigh: 2},
		{Lan: "en", Name: "English", Status: 1, Weigh: 1},
	}).Error)
	require.NoError(t, s.BatchUpsert(ctx, []LanguageContent{
		{Lan: "zh-cn", Group: "site", Key: "title", Type: ContentTypeText, Value: "标题"},
		{Lan: "en", Group: "site", Key: "title", Type: ContentTypeText, Value: "Title"},
	}))
	return s, db
}

// TestEnabledLanguagesCache 验证 H2 进程内缓存：首次查库、二次走缓存、显式
// 失效后重查、TTL 过期后重查。
func TestEnabledLanguagesCache(t *testing.T) {
	s, db := newCacheTestService(t, "country-i18n-cache")
	ctx := context.Background()

	languages, err := s.EnabledLanguages(ctx)
	require.NoError(t, err)
	require.Len(t, languages, 2)
	require.Equal(t, "zh-cn", languages[0].Lan)

	// 直改库（绕过服务写路径）：缓存未失效时仍返回旧快照
	require.NoError(t, db.Table("dw_country_language").Where("lan = ?", "en").Update("status", 0).Error)
	languages, err = s.EnabledLanguages(ctx)
	require.NoError(t, err)
	require.Len(t, languages, 2)

	// 显式失效后重查，命中最新数据
	s.InvalidateLanguageCache()
	languages, err = s.EnabledLanguages(ctx)
	require.NoError(t, err)
	require.Len(t, languages, 1)
	require.Equal(t, "zh-cn", languages[0].Lan)

	// 缓存命中时返回副本，调用方修改不影响后续读取
	languages[0].Lan = "mutated"
	fresh, err := s.EnabledLanguages(ctx)
	require.NoError(t, err)
	require.Equal(t, "zh-cn", fresh[0].Lan)

	// DefaultLan 与 Get 的默认语言回退共用同一缓存
	s.InvalidateLanguageCache()
	require.NoError(t, db.Table("dw_country_language").Where("status = ?", 0).Update("status", 1).Error)
	lan, err := s.DefaultLan(ctx)
	require.NoError(t, err)
	require.Equal(t, "zh-cn", lan)
}

// TestEnabledLanguagesCacheTTLExpiry 验证 TTL 过期后自动重查。
func TestEnabledLanguagesCacheTTLExpiry(t *testing.T) {
	s, db := newCacheTestService(t, "country-i18n-cache-ttl")
	ctx := context.Background()

	originalTTL := languageCacheTTL
	languageCacheTTL = time.Millisecond
	defer func() { languageCacheTTL = originalTTL }()

	languages, err := s.EnabledLanguages(ctx)
	require.NoError(t, err)
	require.Len(t, languages, 2)

	require.NoError(t, db.Table("dw_country_language").Where("lan = ?", "en").Update("status", 0).Error)
	time.Sleep(5 * time.Millisecond) // 越过 TTL
	languages, err = s.EnabledLanguages(ctx)
	require.NoError(t, err)
	require.Len(t, languages, 1)
	require.Equal(t, "zh-cn", languages[0].Lan)
}

// TestGetByRequest 验证 DB 翻译串联：有请求语言/无请求语言回退默认语言、
// 目标语言缺条回退默认语言、缺失 key 返回 ErrRecordNotFound。
func TestGetByRequest(t *testing.T) {
	s, _ := newCacheTestService(t, "country-i18n-cache-request")
	gin.SetMode(gin.TestMode)

	// 无请求语言：回退默认语言 zh-cn
	value, err := s.GetByRequest(context.Background(), "site", "title")
	require.NoError(t, err)
	require.Equal(t, "标题", value)

	// 有请求语言（H1 缓存写入）：en 直接命中
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/index/index", nil)
	i18n.SetLangToContext(c, "en")
	value, err = s.GetByRequest(c, "site", "title")
	require.NoError(t, err)
	require.Equal(t, "Title", value)

	// 请求语言缺条：回退默认语言
	i18n.SetLangToContext(c, "ja")
	value, err = s.GetByRequest(c, "site", "title")
	require.NoError(t, err)
	require.Equal(t, "标题", value)

	// 请求语言经 request context 传入同样可读（SetLangToContext 双写）
	value, err = s.GetByRequest(c.Request.Context(), "site", "title")
	require.NoError(t, err)
	require.Equal(t, "标题", value)

	// 缺失 key：默认语言也缺 → ErrRecordNotFound
	_, err = s.GetByRequest(c, "site", "missing")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
