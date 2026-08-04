package repository

import (
	"context"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/persistence"

	"gorm.io/gorm"
)

type AdminGroupRepository struct {
	persistence.BaseModel
}

func NewAdminGroupRepository(sqlDB *gorm.DB, config *conf.Configuration) *AdminGroupRepository {
	return &AdminGroupRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"admin_group", "id", "name", sqlDB),
	}
}

func (s *AdminGroupRepository) GetOne(ctx context.Context, id int32) (adminGroup model.AdminGroup, err error) {
	err = s.DBFor(ctx).Omit("update_time").Where("id=?", id).Take(&adminGroup).Error
	return
}

// ListWhere executes a caller-built WHERE query against the admin_group
// table. The condition string and parameters are assembled by the service
// layer; persistence stays inside the repository (GORM single entry point).
func (s *AdminGroupRepository) ListWhere(ctx context.Context, where string, params ...any) ([]*model.AdminGroup, error) {
	list := []*model.AdminGroup{}
	err := s.DBFor(ctx).Table(s.TableName).Where(where, params...).Find(&list).Error
	return list, err
}

func (s *AdminGroupRepository) Add(ctx context.Context, adminGroup model.AdminGroup) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Create(&adminGroup)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("create failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *AdminGroupRepository) Edit(ctx context.Context, adminGroup model.AdminGroup) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Save(&adminGroup)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.AdminGroup{}).Where("id = ?", adminGroup.ID).Count(&visible).Error; err != nil {
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
	})
}

// SwitchStatus updates only the status column for a single group. It is used
// by the quick-edit path after the service authorization chain has run.
func (s *AdminGroupRepository) SwitchStatus(ctx context.Context, id int32, status string) error {
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		result = tx.Model(&model.AdminGroup{}).Where("id = ?", id).Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.AdminGroup{}).Where("id = ?", id).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return gorm.ErrRecordNotFound
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// ChildGroupIDs returns the ids of all groups whose parent is one of ids
// (the child-first delete guard data source).
func (s *AdminGroupRepository) ChildGroupIDs(tx *gorm.DB, ids []int32) ([]int32, error) {
	var subIds []int32
	if err := tx.Model(&model.AdminGroup{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
		return nil, err
	}
	return subIds, nil
}

// OperatorGroupIDs returns the group ids the operator currently belongs to
// (the self-group delete guard data source).
func (s *AdminGroupRepository) OperatorGroupIDs(tx *gorm.DB, operatorID int32) ([]int32, error) {
	var groupIds []int32
	if err := tx.Model(&model.AdminGroupAccess{}).Where("uid=?", operatorID).Pluck("group_id", &groupIds).Error; err != nil {
		return nil, err
	}
	return groupIds, nil
}

// DeleteGroupsExcluding deletes the groups in ids that are not also in
// excluded, verifying the count so the delete is all-or-nothing.
func (s *AdminGroupRepository) DeleteGroupsExcluding(tx *gorm.DB, ids, excluded []int32) error {
	query := tx.Model(&model.AdminGroup{}).Where(" id in ? AND id not in ?  ", ids, excluded)
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
