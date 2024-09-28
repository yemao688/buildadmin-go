package model

import (
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserGroup 会员组表
type UserGroup struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Name       string `gorm:"column:name;not null;comment:组名" json:"name"`                         // 组名
	Rules      string `gorm:"column:rules;comment:权限节点" json:"rules"`                              // 权限节点
	Status     string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"`   // 更新时间
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`   // 创建时间
}

type UserGroupModel struct {
	BaseModel
}

func NewUserGroupModel(sqlDB *gorm.DB, config *conf.Configuration) *UserGroupModel {
	return &UserGroupModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "user_group",
			Key:              "id",
			QuickSearchField: "name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *UserGroupModel) GetOne(ctx *gin.Context, id int32) (userGroup UserGroup, err error) {
	err = s.sqlDB.Where("id=?", id).First(&userGroup).Error
	return
}

func (s *UserGroupModel) List(ctx *gin.Context) (list []UserGroup, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Model(&UserGroup{}).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *UserGroupModel) Add(ctx *gin.Context, userGroup UserGroup) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&userGroup).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *UserGroupModel) Edit(ctx *gin.Context, userGroup UserGroup) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Save(&userGroup).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *UserGroupModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Model(&UserGroup{}).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
