package repository

import (
	"context"
	"errors"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/persistence"

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

// CreateTx inserts one rule inside the caller's transaction (atomic write
// primitive).
func (s *AdminRuleRepository) CreateTx(tx *gorm.DB, adminRule model.AdminRule) error {
	result := tx.Create(&adminRule)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return cErr.BadRequest("create failed: rows affected mismatch")
	}
	return nil
}

// GetByIDTx loads one rule inside the caller's transaction.
func (s *AdminRuleRepository) GetByIDTx(tx *gorm.DB, id int32) (model.AdminRule, error) {
	var parent model.AdminRule
	err := tx.Where("id=?", id).First(&parent).Error
	return parent, err
}

// UpdateTx saves one rule inside the caller's transaction (atomic write
// primitive).
func (s *AdminRuleRepository) UpdateTx(tx *gorm.DB, adminRule model.AdminRule) error {
	result := tx.Save(&adminRule)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var visible int64
		if err := tx.Model(&model.AdminRule{}).Where("id = ?", adminRule.ID).Count(&visible).Error; err != nil {
			return err
		}
		if visible == 1 {
			return nil
		}
		return cErr.BadRequest("update failed: rows affected mismatch")
	}
	if result.RowsAffected != 1 {
		return cErr.BadRequest("update failed: rows affected mismatch")
	}
	return nil
}

// DetachParentTx clears the pid of one rule inside the caller's transaction
// (the cycle-break write primitive).
func (s *AdminRuleRepository) DetachParentTx(tx *gorm.DB, id int32) error {
	result := tx.Model(&model.AdminRule{}).Where("id=?", id).Update("pid", 0)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var visible int64
		if err := tx.Model(&model.AdminRule{}).Where("id = ?", id).Count(&visible).Error; err != nil {
			return err
		}
		if visible == 1 {
			return nil
		}
		return cErr.BadRequest("parent update failed")
	}
	if result.RowsAffected != 1 {
		return cErr.BadRequest("parent update failed")
	}
	return nil
}

// ChildRuleIDs returns the ids of all rules whose parent is one of ids
// (the child-first delete guard data source).
func (s *AdminRuleRepository) ChildRuleIDs(tx *gorm.DB, ids []int32) ([]int32, error) {
	var subIds []int32
	if err := tx.Model(&model.AdminRule{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
		return nil, err
	}
	return subIds, nil
}

// DeleteTx deletes the rules in ids, verifying the count so the batch delete
// is all-or-nothing.
func (s *AdminRuleRepository) DeleteTx(tx *gorm.DB, ids []int32) error {
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
}

func (s *AdminRuleRepository) GetRulePIds(ctx context.Context, ids []string) ([]int32, error) {
	pids := []int32{}
	err := s.DBFor(ctx).Model(&model.AdminRule{}).Where("id in ?", ids).Pluck("pid", &pids).Error
	return pids, err
}

// Delete removes a menu rule by path, optionally recursing into children and
// pruning parent directories that become childless. 对齐上游
// Menu::delete:菜单不存在时视为已删除（幂等）。
//
// 保留在 repo 的例外入口：它由 pkg 层 CRUD 生成器（crud_helper）在删除
// 业务模块时调用，pkg 层不允许反向依赖 admin 渠道 service；handler 侧的
// 菜单删除编排已全部迁往 AdminRuleService。
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
