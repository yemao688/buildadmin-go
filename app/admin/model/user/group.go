package user

import (
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// TableName 经全局命名策略解析（前缀安全），对齐真实表 user_group。
func (Group) TableName(namer schema.Namer) string {
	return namer.TableName("user_group")
}

// Group 会员组表
type Group struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Name       string `gorm:"column:name;not null;comment:组名" json:"name"`                         // 组名
	Rules      string `gorm:"column:rules;comment:权限节点" json:"rules"`                              // 权限节点
	Status     string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"`   // 更新时间
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`   // 创建时间
}

type GroupModel struct {
	BaseModel
}

func NewGroupModel(sqlDB *gorm.DB, config *conf.Configuration) *GroupModel {
	return &GroupModel{
		BaseModel: NewBaseModel(config.Database.Prefix+"user_group", "id", "name", sqlDB),
	}
}

func (s *GroupModel) GetOne(ctx *gin.Context, id int32) (group Group, err error) {
	err = s.DB().Where("id=?", id).First(&group).Error
	return
}

func (s *GroupModel) List(ctx *gin.Context) (list []Group, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, err
	}
	err = s.DB().Model(&Group{}).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *GroupModel) Add(ctx *gin.Context, group Group) error {
	tx := s.DB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&group).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *GroupModel) Edit(ctx *gin.Context, group Group) error {
	tx := s.DB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Save(&group).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *GroupModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.DB().Model(&Group{}).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
