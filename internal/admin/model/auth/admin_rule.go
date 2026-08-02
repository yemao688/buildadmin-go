package auth

import (
	"context"
	"errors"
	"go-build-admin/internal/conf"
	cErr "go-build-admin/internal/pkg/error"
	"slices"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminRuleModel struct {
	BaseModel
}

func NewAdminRuleModel(sqlDB *gorm.DB, config *conf.Configuration) *AdminRuleModel {
	return &AdminRuleModel{
		BaseModel: NewBaseModel(config.Database.Prefix+"admin_rule", "id", "title", sqlDB),
	}
}

func (s *AdminRuleModel) GetOne(ctx *gin.Context, id int32) (adminRule AdminRule, err error) {
	err = s.DBFor(ctx).Where("id=?", id).Take(&adminRule).Error
	return
}

func (s *AdminRuleModel) List(ctx *gin.Context) (list []AdminRule, err error) {
	err = s.DBFor(ctx).Model(&AdminRule{}).Order("weigh desc,id desc").Find(&list).Error
	return
}

func (s *AdminRuleModel) Add(ctx *gin.Context, adminRule AdminRule) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Create(&adminRule)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("create failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *AdminRuleModel) Edit(ctx *gin.Context, adminRule AdminRule) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		parent := AdminRule{}
		if adminRule.Pid > 0 {
			if err := tx.Where("id=?", adminRule.Pid).First(&parent).Error; err != nil {
				return err
			}
		}
		if parent.Pid == adminRule.ID {
			result := tx.Model(&AdminRule{}).Where("id=?", parent.ID).Update("pid", 0)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return cErr.BadRequest("parent update failed")
			}
		}
		result := tx.Save(&adminRule)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("update failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *AdminRuleModel) Del(ctx *gin.Context, ids []int32) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var subIds []int32
		if err := tx.Model(&AdminRule{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
			return err
		}
		for _, v := range subIds {
			if !slices.Contains(ids, v) {
				return cErr.BadRequest("Please delete the child element first, or use batch deletion")
			}
		}
		query := tx.Model(&AdminRule{}).Where(" id in ? ", ids)
		var expected int64
		if err := query.Count(&expected).Error; err != nil {
			return err
		}
		result := query.Delete(nil)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != expected {
			return cErr.BadRequest("delete failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *AdminRuleModel) GetRulePIds(ids []string, contexts ...*gin.Context) ([]int32, error) {
	var ctx *gin.Context
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	pids := []int32{}
	err := s.DBFor(ctx).Model(&AdminRule{}).Where("id in ?", ids).Pluck("pid", &pids).Error
	return pids, err
}

// crud 删除菜单
// 对齐上游 Menu::delete:菜单不存在时视为已删除(幂等);删除后递归清理
// 已无子级的父级目录。
func (s *AdminRuleModel) Delete(path string, recursion bool) error {
	return s.Transaction(context.Background(), func(tx *gorm.DB) error {
		var deleteRule func(string) error
		deleteRule = func(name string) error {
			var adminRule AdminRule
			err := tx.Where(" name = ? ", name).Take(&adminRule).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			var list []AdminRule
			if err := tx.Model(&AdminRule{}).Where(" pid = ? ", adminRule.ID).Find(&list).Error; err != nil {
				return err
			}
			if recursion {
				for _, child := range list {
					if err := deleteRule(child.Name); err != nil {
						return err
					}
				}
			}
			if len(list) == 0 || recursion {
				if err := tx.Model(&AdminRule{}).Where(" id = ? ", adminRule.ID).Delete(nil).Error; err != nil {
					return err
				}
				// 父级目录已无子级时一并删除
				var parent AdminRule
				if err := tx.Take(&parent, adminRule.Pid).Error; err == nil {
					var childCount int64
					if err := tx.Model(&AdminRule{}).Where(" pid = ? ", parent.ID).Count(&childCount).Error; err != nil {
						return err
					}
					if childCount == 0 {
						return deleteRule(parent.Name)
					}
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			return nil
		}
		return deleteRule(path)
	})
}
