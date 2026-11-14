package repository

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/persistence"
	"buildadmin-go/internal/pkg/util"
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminLogRepository struct {
	persistence.BaseModel
}

func NewAdminLogRepository(sqlDB *gorm.DB, config *conf.Configuration) *AdminLogRepository {
	return &AdminLogRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"admin_log", "id", "title", sqlDB),
	}
}

// GetEnabledAdmin returns the username from the same status check used by the
// admin authentication middleware.
func (s *AuthRepository) GetEnabledAdmin(id int32) (string, bool) {
	if s == nil || s.sqlDB == nil {
		return "", false
	}
	var admin struct {
		Status   string
		Username string
	}
	err := s.sqlDB.Model(&model.Admin{}).Select("status, username").Where("id=?", id).First(&admin).Error
	return admin.Username, err == nil && admin.Status == "enable"
}

// UsernameByID 读取管理员用户名，供操作日志记录历史操作者。这里不做账户
// 状态过滤：日志要如实记录当次请求的实际操作者。
func (s *AuthRepository) UsernameByID(_ context.Context, id int32) (string, bool) {
	if s == nil || s.sqlDB == nil || id == 0 {
		return "", false
	}
	var admin struct {
		Username string
	}
	err := s.sqlDB.Model(&model.Admin{}).Select("username").Where("id=?", id).Take(&admin).Error
	if err != nil || admin.Username == "" {
		return "", false
	}
	return admin.Username, true
}

// CachedRuleTitle returns a title from the administrator's permission cache.
// It intentionally does not load the cache; authorization owns cache loading.
func (s *AuthRepository) CachedRuleTitle(uid int32, name string) (string, bool) {
	if s == nil {
		return "", false
	}
	rules, ok := s.cache.Rules(uid)
	if !ok {
		return "", false
	}
	name = strings.ToLower(name)
	for _, rule := range rules {
		if strings.ToLower(rule.Name) == name {
			return rule.Title, true
		}
	}
	return "", false
}

func (s *AdminLogRepository) List(ctx *gin.Context) (list []model.AdminLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	// count 与 find 必须使用独立 statement：复用同一 db 先 Count 再 Find 时，
	// GORM 的 Count 会重置 statement，导致 Find 丢失 scope/搜索 WHERE。
	countDB := s.DB().Model(&model.AdminLog{}).Scopes(IsSuperAdmin(ctx)).Where(whereS, whereP...)
	if err = countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	findDB := s.DB().Model(&model.AdminLog{}).Scopes(IsSuperAdmin(ctx)).Where(whereS, whereP...)
	err = findDB.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

// Create 单表插入一条后台操作日志。只做插入与 RowsAffected 完整性守卫，
// 请求解析、业务分支与错误上报由调用方（service）负责。
func (s *AdminLogRepository) Create(ctx context.Context, adminLog *model.AdminLog) error {
	if s == nil || s.DB() == nil {
		return errors.New("admin log repository database is unavailable")
	}
	if adminLog == nil {
		return gorm.ErrInvalidData
	}
	result := s.DBFor(ctx).Create(adminLog)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("admin log insert affected unexpected number of rows")
	}
	return nil
}

func (s *AdminLogRepository) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return gorm.ErrInvalidData
	}
	normalized, err := util.NormalizeIDs(values, "admin log", false, func(int32) error {
		return gorm.ErrInvalidData
	})
	if err != nil {
		return err
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.AdminLog{}).Where("id IN ?", normalized).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&model.AdminLog{}).Where("id IN ?", normalized).Delete(nil).Error
	})
}
