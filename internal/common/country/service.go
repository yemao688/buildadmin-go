package country

import (
	"context"
	"sync"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/i18n"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 注意：以下响应结构必须携带 json tag——它们会直接序列化进
// /api/index/index（前台初始化）与 /admin/index/index 的响应，
// 键名契约对齐 PHP 上游（全小写字段名）。
type Language struct {
	ID     int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Lan    string `gorm:"column:lan" json:"lan"`
	Name   string `gorm:"column:name" json:"name"`
	Remark string `gorm:"column:remark" json:"remark"`
	Status int8   `gorm:"column:status" json:"status"`
	Weigh  int32  `gorm:"column:weigh" json:"weigh"`
}

type LanguageContent struct {
	ID    int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Lan   string `gorm:"column:lan" json:"lan"`
	Group string `gorm:"column:group" json:"group"`
	Key   string `gorm:"column:key" json:"key"`
	Type  string `gorm:"column:type" json:"type"`
	Value string `gorm:"column:value" json:"value"`
}

const (
	ContentTypeText     = "0"
	ContentTypeRichText = "1"
	ContentTypeImg      = "2"
)

type Currency struct {
	ID     int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Code   string  `gorm:"column:code" json:"code"`
	Name   string  `gorm:"column:name" json:"name"`
	Symbol string  `gorm:"column:symbol" json:"symbol"`
	Rate   float64 `gorm:"column:rate" json:"rate"`
	Status int8    `gorm:"column:status" json:"status"`
	Weigh  int32   `gorm:"column:weigh" json:"weigh"`
}

// languageCacheTTL 是启用的语言列表进程内缓存生存时间。缓存按 Service 实例
// 持有：admin country_language CRUD 写成功经 requesttx 失效回调立即清空，
// TTL 作为直改库、其它进程写入等旁路路径的兜底。包内测试可临时调短以验证
// 过期重查路径。
var languageCacheTTL = 60 * time.Second

type languageCacheEntry struct {
	languages []Language
	expiresAt time.Time
}

// currencyCacheTTL 是启用的货币列表进程内缓存生存时间。缓存按 Service 实例
// 持有：admin country_currency CRUD 写成功经 requesttx 失效回调立即清空，
// TTL 作为直改库、其它进程写入等旁路路径的兜底。包内测试可临时调短以验证
// 过期重查路径。
var currencyCacheTTL = 60 * time.Second

type currencyCacheEntry struct {
	currencies []Currency
	expiresAt  time.Time
}

type Service struct {
	db     *gorm.DB
	prefix string

	mu    sync.RWMutex
	cache *languageCacheEntry

	currencyMu    sync.RWMutex
	currencyCache *currencyCacheEntry
}

func NewService(db *gorm.DB, config *conf.Configuration) *Service {
	prefix := ""
	if config != nil {
		prefix = config.Database.Prefix
	}
	return &Service{db: db, prefix: prefix}
}

// GetByRequest 按当前请求语言返回 DB 动态内容翻译（country_language_content）：
//   - 请求语言取 H1 解析缓存（i18n.LangFromContext，与静态 UI 文案共用同一
//     请求语言解析结果）；无请求语言时回退前台默认语言 DefaultLan；
//   - 再走 Get 的既有语义：目标语言缺条时回退默认语言，仍未命中返回
//     gorm.ErrRecordNotFound。
//
// 用途边界：静态 UI 文案走前端 t()/i18n YAML 语言包；动态内容翻译（商品名、
// 公告等 DB 内容）走本方法，两条链共用同一请求语言。注意 context 中的请求
// 语言是规范化 pack key（如 zh-cn→zh-CN），与 country_language.lan 原始值
// （如 zh-cn）可能不同，缺条时由 Get 的默认语言回退桥接。
func (s *Service) GetByRequest(ctx context.Context, group, key string) (string, error) {
	lan, ok := i18n.LangFromContext(ctx)
	if !ok {
		var err error
		lan, err = s.DefaultLan(ctx)
		if err != nil {
			return "", err
		}
	}
	return s.Get(ctx, lan, group, key)
}

func (s *Service) Get(ctx context.Context, lan, group, key string) (string, error) {
	var content LanguageContent
	if err := s.findContent(ctx, lan, group, key, &content); err == nil {
		return content.Value, nil
	} else if err != gorm.ErrRecordNotFound {
		return "", err
	}
	defaultLan, err := s.DefaultLan(ctx)
	if err != nil {
		return "", err
	}
	if defaultLan == lan {
		return "", gorm.ErrRecordNotFound
	}
	if err := s.findContent(ctx, defaultLan, group, key, &content); err == nil {
		return content.Value, nil
	} else if err != gorm.ErrRecordNotFound {
		return "", err
	}
	return "", gorm.ErrRecordNotFound
}

func (s *Service) findContent(ctx context.Context, lan, group, key string, content *LanguageContent) error {
	return s.db.WithContext(ctx).Table(s.table("country_language_content")).Where("lan = ? AND `group` = ? AND `key` = ?", lan, group, key).Take(content).Error
}

func (s *Service) BatchUpsert(ctx context.Context, rows []LanguageContent) error {
	return s.db.WithContext(ctx).Table(s.table("country_language_content")).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "lan"}, {Name: "group"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "type"}),
	}).Create(&rows).Error
}

func (s *Service) DefaultLan(ctx context.Context) (string, error) {
	languages, err := s.EnabledLanguages(ctx)
	if err != nil {
		return "", err
	}
	if len(languages) == 0 {
		return "en", nil
	}
	return languages[0].Lan, nil
}

func (s *Service) EnabledLanguages(ctx context.Context) ([]Language, error) {
	if entry := s.loadLanguageCache(); entry != nil {
		// 返回副本，避免调用方误改污染共享缓存
		return append([]Language(nil), entry.languages...), nil
	}
	var values []Language
	if err := s.db.WithContext(ctx).Table(s.table("country_language")).Where("status = ?", 1).Order("weigh DESC, id ASC").Find(&values).Error; err != nil {
		return nil, err
	}
	s.storeLanguageCache(values)
	return values, nil
}

// InvalidateLanguageCache 清空语言列表进程内缓存（下一次查询重新走库）。
// country_language 写操作（admin CRUD）提交成功后经
// requesttx.InvalidateAfterMutation 调用；包内测试也用它把缓存复位到未加载态。
func (s *Service) InvalidateLanguageCache() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

func (s *Service) loadLanguageCache() *languageCacheEntry {
	s.mu.RLock()
	entry := s.cache
	s.mu.RUnlock()
	if entry == nil || !time.Now().Before(entry.expiresAt) {
		return nil
	}
	return entry
}

func (s *Service) storeLanguageCache(languages []Language) {
	s.mu.Lock()
	s.cache = &languageCacheEntry{
		languages: append([]Language(nil), languages...),
		expiresAt: time.Now().Add(languageCacheTTL),
	}
	s.mu.Unlock()
}

func (s *Service) EnabledCurrencies(ctx context.Context) ([]Currency, error) {
	if entry := s.loadCurrencyCache(); entry != nil {
		// 返回副本，避免调用方误改污染共享缓存
		return append([]Currency(nil), entry.currencies...), nil
	}
	var values []Currency
	if err := s.db.WithContext(ctx).Table(s.table("country_currency")).Where("status = ?", 1).Order("weigh DESC, id ASC").Find(&values).Error; err != nil {
		return nil, err
	}
	s.storeCurrencyCache(values)
	return values, nil
}

// InvalidateCurrencyCache 清空货币列表进程内缓存（下一次查询重新走库）。
// country_currency 写操作（admin CRUD）提交成功后经
// requesttx.InvalidateAfterMutation 调用；包内测试也用它把缓存复位到未加载态。
func (s *Service) InvalidateCurrencyCache() {
	s.currencyMu.Lock()
	s.currencyCache = nil
	s.currencyMu.Unlock()
}

func (s *Service) loadCurrencyCache() *currencyCacheEntry {
	s.currencyMu.RLock()
	entry := s.currencyCache
	s.currencyMu.RUnlock()
	if entry == nil || !time.Now().Before(entry.expiresAt) {
		return nil
	}
	return entry
}

func (s *Service) storeCurrencyCache(currencies []Currency) {
	s.currencyMu.Lock()
	s.currencyCache = &currencyCacheEntry{
		currencies: append([]Currency(nil), currencies...),
		expiresAt:  time.Now().Add(currencyCacheTTL),
	}
	s.currencyMu.Unlock()
}

func (s *Service) table(name string) string { return s.prefix + name }
