package auth

import (
	"go-build-admin/internal/conf"
	cErr "go-build-admin/internal/pkg/error"
	"go-build-admin/internal/pkg/header"
	"slices"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminGroupModel struct {
	BaseModel
}

func NewAdminGroupModel(sqlDB *gorm.DB, config *conf.Configuration) *AdminGroupModel {
	return &AdminGroupModel{
		BaseModel: NewBaseModel(config.Database.Prefix+"admin_group", "id", "name", sqlDB),
	}
}

func (s *AdminGroupModel) GetOne(ctx *gin.Context, id int32) (adminGroup AdminGroup, err error) {
	err = s.DBFor(ctx).Omit("update_time").Where("id=?", id).Take(&adminGroup).Error
	return
}

func (s *AdminGroupModel) Add(ctx *gin.Context, adminGroup AdminGroup) error {
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

func (s *AdminGroupModel) Edit(ctx *gin.Context, adminGroup AdminGroup) error {
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

func (s *AdminGroupModel) Del(ctx *gin.Context, ids []int32) error {
	var subIds []int32
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Model(&AdminGroup{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
			return err
		}
		for _, v := range subIds {
			if !slices.Contains(ids, v) {
				return cErr.BadRequest("Please delete the child element first, or use batch deletion")
			}
		}
		adminAuth := header.GetAdminAuth(ctx)
		groupIds := []int32{}
		if err := tx.Model(&AdminGroupAccess{}).Where("uid=?", adminAuth.Id).Pluck("group_id", &groupIds).Error; err != nil {
			return err
		}
		query := tx.Model(&AdminGroup{}).Where(" id in ? AND id not in ?  ", ids, groupIds)
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
