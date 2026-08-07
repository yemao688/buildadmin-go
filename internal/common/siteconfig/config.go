package siteconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"buildadmin-go/internal/pkg/requesttx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"slices"
	"strings"
)

type Config struct {
	ID       int32  `gorm:"column:id;type:int(11) unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Name     string `gorm:"column:name;type:varchar(30) default '';not null;uniqueIndex:name;comment:变量名" json:"name"`          // 变量名
	Group    string `gorm:"column:group;type:varchar(30) default '';not null;comment:分组" json:"group"`                          // 分组
	Title    string `gorm:"column:title;type:varchar(50) default '';not null;comment:变量标题" json:"title"`                        // 变量标题
	Tip      string `gorm:"column:tip;type:varchar(100) default '';not null;comment:变量描述" json:"tip"`                           // 变量描述
	Type     string `gorm:"column:type;type:varchar(30) default '';not null;comment:变量输入组件类型" json:"type"`                      // 变量输入组件类型
	Value    string `gorm:"column:value;type:longtext;comment:变量值" json:"value"`                                                // 变量值
	Content  string `gorm:"column:content;type:longtext;comment:字典数据" json:"content"`                                           // 字典数据
	Rule     string `gorm:"column:rule;type:varchar(100) default '';not null;comment:验证规则" json:"rule"`                         // 验证规则
	Extend   string `gorm:"column:extend;type:varchar(255) default '';not null;comment:扩展属性" json:"extend"`                     // 扩展属性
	AllowDel int32  `gorm:"column:allow_del;type:tinyint(4) unsigned;not null;default:0;comment:允许删除:0=否,1=是" json:"allow_del"` // 允许删除:0=否,1=是
	Weigh    int32  `gorm:"column:weigh;type:int(11);not null;default:0;comment:权重" json:"weigh"`                               // 权重
}

var jsonDecodeType = []string{"checkbox", "array", "selects"}
var needContent = []string{"radio", "checkbox", "select", "selects"}

func (s *Config) SetValueAttr(value any, t string) string {
	if slices.Contains(jsonDecodeType, t) {
		if v, err := json.Marshal(value); err == nil {
			return string(v)
		}
	} else if t == "switch" {
		if v, ok := value.(bool); ok && v {
			return "1"
		} else {
			return "0"
		}
	} else if t == "time" {
		return value.(string)
	} else if t == "city" || t == "remoteSelects" {
		cityIds := []string{}
		for _, v := range value.([]interface{}) {
			cityIds = append(cityIds, fmt.Sprintf("%v", v))
		}
		return strings.Join(cityIds, ",")
	}
	return fmt.Sprintf("%v", value)
}

func (s *Config) GetValueAttr() any {
	if slices.Contains(jsonDecodeType, s.Type) {
		resultArr := []any{}
		if len(s.Value) > 0 {
			if s.Type == "checkbox" || s.Type == "selects" {
				if err := json.Unmarshal([]byte(s.Value), &resultArr); err == nil {
					return resultArr
				}
			} else {
				result := []map[string]any{}
				if err := json.Unmarshal([]byte(s.Value), &result); err == nil {
					return result
				}
			}
		}
		return resultArr
	} else if s.Type == "switch" {
		if s.Value == "0" {
			return false
		} else {
			return true
		}
	} else if s.Type == "editor" {
		return s.Value
	} else if s.Type == "city" || s.Type == "remoteSelects" {
		if s.Value == "" {
			return []any{}
		}
		return strings.Split(s.Value, ",")
	}
	return s.Value
}

func (s *Config) GetContentAttr() any {
	if slices.Contains(needContent, s.Type) {
		content := map[string]any{}
		if err := json.Unmarshal([]byte(s.Content), &content); err == nil {
			return content
		}
	}
	return map[string]any{}
}

func (s *Config) GetExtendAttr() any {
	extend := map[string]any{}
	if s.Extend != "" {
		err := json.Unmarshal([]byte(s.Extend), &extend)
		if err == nil {
			delete(extend, "baInputExtend")
		}
	}
	return extend
}

func (s *Config) GetInputExtendAttr() any {
	extend := map[string]any{}
	if s.Extend != "" {
		err := json.Unmarshal([]byte(s.Extend), &extend)
		if err == nil {
			if _, ok := extend["baInputExtend"]; ok {
				return extend["baInputExtend"]
			}
		}
	}
	return extend
}

// siteconfigTTL 是 siteconfig KV 分组列表进程内缓存生存时间。缓存按 Service
// 实例持有：admin config CRUD 写成功经 requesttx 失效回调立即清空，
// TTL 作为直改库、其它进程写入等旁路路径的兜底。包内测试可临时调短以验证
// 过期重查路径。
var siteconfigTTL = 60 * time.Second

// siteconfigCacheEntry 保存一个分组的 KV 快照与过期时间。
type siteconfigCacheEntry struct {
	kv        map[string]string
	expiresAt time.Time
}

type Service struct {
	sqlDB *gorm.DB

	mu    sync.RWMutex
	cache map[string]*siteconfigCacheEntry
}

func NewService(sqlDB *gorm.DB) *Service {
	return &Service{sqlDB: sqlDB}
}

func requestContext(ctx context.Context) context.Context {
	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx.Request != nil {
		return ginCtx.Request.Context()
	}
	return ctx
}

func (s *Service) dbFor(ctx context.Context) *gorm.DB {
	ctx = requestContext(ctx)
	if db := requesttx.DB(ctx); db != nil {
		return db
	}
	return s.sqlDB
}

func (s *Service) GetValueByName(ctx *gin.Context, name string) (string, error) {
	var config Config
	err := s.dbFor(ctx).Where("`name`= ? ", name).Take(&config).Error
	return config.Value, err
}

func (s *Service) GetKVByGroup(ctx *gin.Context, group string) (map[string]string, error) {
	// 请求事务内的读可能看到未提交数据，绕过缓存直接走库（缓存只保存已提交
	// 快照；写路径在事务提交后经 InvalidateAfterMutation 清空）。
	if !requesttx.Active(requestContext(ctx)) {
		if entry := s.loadGroupCache(group); entry != nil {
			// 返回副本，避免调用方误改污染共享缓存
			return cloneStringMap(entry.kv), nil
		}
	}

	var configList []*Config
	err := s.dbFor(ctx).Where("`group`=?", group).Find(&configList).Error
	if err != nil {
		return nil, err
	}

	data := map[string]string{}
	for _, v := range configList {
		data[v.Name] = v.Value
	}
	if !requesttx.Active(requestContext(ctx)) {
		s.storeGroupCache(group, data)
	}
	return data, nil
}

// InvalidateSiteConfigCache 清空 KV 分组进程内缓存（下一次查询重新走库）。
// siteconfig 写操作（admin config CRUD）提交成功后经
// requesttx.InvalidateAfterMutation 调用；包内测试也用它把缓存复位到未加载态。
func (s *Service) InvalidateSiteConfigCache() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

func (s *Service) loadGroupCache(group string) *siteconfigCacheEntry {
	s.mu.RLock()
	entry := s.cache[group]
	s.mu.RUnlock()
	if entry == nil || !time.Now().Before(entry.expiresAt) {
		return nil
	}
	return entry
}

func (s *Service) storeGroupCache(group string, kv map[string]string) {
	s.mu.Lock()
	if s.cache == nil {
		s.cache = make(map[string]*siteconfigCacheEntry)
	}
	s.cache[group] = &siteconfigCacheEntry{
		kv:        cloneStringMap(kv),
		expiresAt: time.Now().Add(siteconfigTTL),
	}
	s.mu.Unlock()
}

func cloneStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
