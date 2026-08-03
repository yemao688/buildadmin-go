package repository

import (
	"context"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/persistence"
	"slices"

	"github.com/gin-gonic/gin"
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
		if result.RowsAffected != 1 {
			return cErr.BadRequest("update failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *AdminGroupRepository) Del(ctx *gin.Context, ids []int32) error {
	adminAuth := header.GetAdminAuth(ctx)
	return s.DelWithOperator(ctx, ids, adminAuth.Id)
}

// DelWithOperator is the transport-free counterpart of Del for the service
// layer: the operator's admin id is passed explicitly.
func (s *AdminGroupRepository) DelWithOperator(ctx context.Context, ids []int32, operatorID int32) error {
	var subIds []int32
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Model(&model.AdminGroup{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
			return err
		}
		for _, v := range subIds {
			if !slices.Contains(ids, v) {
				return cErr.BadRequest("Please delete the child element first, or use batch deletion")
			}
		}
		groupIds := []int32{}
		if err := tx.Model(&model.AdminGroupAccess{}).Where("uid=?", operatorID).Pluck("group_id", &groupIds).Error; err != nil {
			return err
		}
		query := tx.Model(&model.AdminGroup{}).Where(" id in ? AND id not in ?  ", ids, groupIds)
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
