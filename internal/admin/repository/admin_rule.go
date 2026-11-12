package repository

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/persistence"
	"context"
	"strings"

	"gorm.io/gorm"
)

type AdminRuleRepository struct {
	persistence.BaseModel
	CRUDStore
}

func NewAdminRuleRepository(sqlDB *gorm.DB, config *conf.Configuration) *AdminRuleRepository {
	s := &AdminRuleRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"admin_rule", "id", "title", sqlDB),
	}
	s.CRUDStore = NewCRUDStore(&s.BaseModel, s.NewRow, nil)
	return s
}

// NewRow 保持 admin_rule 原有的 map 输出形态（字段名来自数据库列名）。
func (s *AdminRuleRepository) NewRow() any {
	row := map[string]any{}
	return &row
}

func (s *AdminRuleRepository) GetOne(ctx context.Context, id int32) (adminRule model.AdminRule, err error) {
	err = s.DBFor(ctx).Where("id=?", id).Take(&adminRule).Error
	return
}

func (s *AdminRuleRepository) List(ctx context.Context) (list []model.AdminRule, err error) {
	err = s.DBFor(ctx).Model(&model.AdminRule{}).Order("weigh desc,id desc").Find(&list).Error
	return
}

// ListForMenu 读取菜单规则列表：isSuperAdmin 或空 ruleIDs 表示不按权限 id 过滤，
// types/status 为空表示不限制对应列；keyword 按空格分词后逐项 AND 匹配
// QuickSearchField。权限判定由 service 负责，本方法只按传入条件做 scoped 读。
func (s *AdminRuleRepository) ListForMenu(ctx context.Context, isSuperAdmin bool, ruleIDs []int32, keyword string, types []string, status string) ([]model.AdminRule, error) {
	query := s.DBFor(ctx).Model(&model.AdminRule{})
	if len(types) > 0 {
		query = query.Where(" type in ? ", types)
	}
	if status != "" {
		query = query.Where(" status = ? ", status)
	}
	if !isSuperAdmin && len(ruleIDs) > 0 {
		query = query.Where(" id in ? ", ruleIDs)
	}
	if keyword != "" {
		for _, term := range strings.Split(keyword, " ") {
			query = query.Where(s.QuickSearchField+" LIKE ? ", "%"+strings.ReplaceAll(term, "%", "\\%")+"%")
		}
	}

	list := []model.AdminRule{}
	err := query.Order("weigh desc,id asc").Find(&list).Error
	return list, err
}

// ListByIDs 批量读取规则（仅 id/title），供列表展示批量补全权限标题。
func (s *AdminRuleRepository) ListByIDs(ctx context.Context, ids []int32) ([]model.AdminRule, error) {
	list := []model.AdminRule{}
	err := s.DBFor(ctx).Model(&model.AdminRule{}).Select("id", "title").Where("id in ?", ids).Find(&list).Error
	return list, err
}

// RemarkByNames 按规则名读取备注，供操作日志展示；名称由调用方从路由解析。
func (s *AdminRuleRepository) RemarkByNames(ctx context.Context, names []string) (string, error) {
	var rule struct{ Remark string }
	if len(names) == 0 {
		return "", nil
	}
	if err := s.DBFor(ctx).Model(&model.AdminRule{}).Where("name in ?", names).Take(&rule).Error; err != nil {
		return "", err
	}
	return rule.Remark, nil
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
