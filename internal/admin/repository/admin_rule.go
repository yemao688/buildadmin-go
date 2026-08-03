package repository

import (
	"context"
	"errors"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/persistence"
	"slices"

	"gorm.io/gorm"
)

type AdminRuleRepository struct {
	persistence.BaseModel
}

func NewAdminRuleRepository(sqlDB *gorm.DB, config *conf.Configuration) *AdminRuleRepository {
	return &AdminRuleRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"admin_rule", "id", "title", sqlDB),
	}
}

func (s *AdminRuleRepository) GetOne(ctx context.Context, id int32) (adminRule model.AdminRule, err error) {
	err = s.DBFor(ctx).Where("id=?", id).Take(&adminRule).Error
	return
}

func (s *AdminRuleRepository) List(ctx context.Context) (list []model.AdminRule, err error) {
	err = s.DBFor(ctx).Model(&model.AdminRule{}).Order("weigh desc,id desc").Find(&list).Error
	return
}

func (s *AdminRuleRepository) Add(ctx context.Context, adminRule model.AdminRule) error {
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

func (s *AdminRuleRepository) Edit(ctx context.Context, adminRule model.AdminRule) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		parent := model.AdminRule{}
		if adminRule.Pid > 0 {
			if err := tx.Where("id=?", adminRule.Pid).First(&parent).Error; err != nil {
				return err
			}
		}
		if parent.Pid == adminRule.ID {
			result := tx.Model(&model.AdminRule{}).Where("id=?", parent.ID).Update("pid", 0)
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

func (s *AdminRuleRepository) Del(ctx context.Context, ids []int32) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var subIds []int32
		if err := tx.Model(&model.AdminRule{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
			return err
		}
		for _, v := range subIds {
			if !slices.Contains(ids, v) {
				return cErr.BadRequest("Please delete the child element first, or use batch deletion")
			}
		}
		query := tx.Model(&model.AdminRule{}).Where(" id in ? ", ids)
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

func (s *AdminRuleRepository) GetRulePIds(ctx context.Context, ids []string) ([]int32, error) {
	pids := []int32{}
	err := s.DBFor(ctx).Model(&model.AdminRule{}).Where("id in ?", ids).Pluck("pid", &pids).Error
	return pids, err
}

// crud 删除菜单
// 对齐上游 Menu::delete:菜单不存在时视为已删除(幂等);删除后递归清理
// 已无子级的父级目录。
func (s *AdminRuleRepository) Delete(path string, recursion bool) error {
	return s.Transaction(context.Background(), func(tx *gorm.DB) error {
		var deleteRule func(string) error
		deleteRule = func(name string) error {
			var adminRule model.AdminRule
			err := tx.Where(" name = ? ", name).Take(&adminRule).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			var list []model.AdminRule
			if err := tx.Model(&model.AdminRule{}).Where(" pid = ? ", adminRule.ID).Find(&list).Error; err != nil {
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
				if err := tx.Model(&model.AdminRule{}).Where(" id = ? ", adminRule.ID).Delete(nil).Error; err != nil {
					return err
				}
				// 父级目录已无子级时一并删除
				var parent model.AdminRule
				if err := tx.Take(&parent, adminRule.Pid).Error; err == nil {
					var childCount int64
					if err := tx.Model(&model.AdminRule{}).Where(" pid = ? ", parent.ID).Count(&childCount).Error; err != nil {
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
