package model

import (
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/conf"
	"slices"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminGroup struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Pid        int32  `gorm:"column:pid;not null;comment:上级分组" json:"pid"`                         // 上级分组
	Name       string `gorm:"column:name;not null;comment:组名" json:"name"`                         // 组名
	Rules      string `gorm:"column:rules;comment:权限规则ID" json:"rules"`                            // 权限规则ID
	Status     string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"`   // 更新时间
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`   // 创建时间
}

type AdminGroupModel struct {
	BaseModel
}

func NewAdminGroupModel(sqlDB *gorm.DB, config *conf.Configuration) *AdminGroupModel {
	return &AdminGroupModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "admin_group",
			Key:              "id",
			QuickSearchField: "name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *AdminGroupModel) GetOne(ctx *gin.Context, id int32) (adminGroup AdminGroup, err error) {
	err = s.sqlDB.Omit("update_time").Where("id=?", id).Take(&adminGroup).Error
	return
}

func (s *AdminGroupModel) Add(ctx *gin.Context, adminGroup AdminGroup) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&adminGroup).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *AdminGroupModel) Edit(ctx *gin.Context, adminGroup AdminGroup) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Save(&adminGroup).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *AdminGroupModel) Del(ctx *gin.Context, ids []int32) error {
	var subIds []int32
	if err := s.sqlDB.Model(&AdminGroup{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
		return err
	}

	for _, v := range subIds {
		if !slices.Contains(ids, v) {
			return cErr.BadRequest("Please delete the child element first, or use batch deletion")
		}
	}

	adminAuth := header.GetAdminAuth(ctx)
	groupIds := []int32{}
	if err := s.sqlDB.Model(&AdminGroupAccess{}).Where("uid=?", adminAuth.Id).Pluck("group_id", &groupIds).Error; err != nil {
		return err
	}
	err := s.sqlDB.Model(&AdminGroup{}).Where(" id in ? AND id not in ?  ", ids, groupIds).Delete(nil).Error
	return err

}
