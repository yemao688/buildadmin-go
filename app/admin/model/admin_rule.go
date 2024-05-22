package model

import (
	cErr "go-build-admin/app/pkg/error"
	"slices"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameAdminRule = "ba_admin_rule"

type AdminRule struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                                     // ID
	Pid        int32  `gorm:"column:pid;not null;comment:上级菜单" json:"pid"`                                                                      // 上级菜单
	Type       string `gorm:"column:type;not null;default:menu;comment:类型:menu_dir=菜单目录,menu=菜单项,button=页面按钮" json:"type"`                      // 类型:menu_dir=菜单目录,menu=菜单项,button=页面按钮
	Title      string `gorm:"column:title;not null;comment:标题" json:"title"`                                                                    // 标题
	Name       string `gorm:"column:name;not null;comment:规则名称" json:"name"`                                                                    // 规则名称
	Path       string `gorm:"column:path;not null;comment:路由路径" json:"path"`                                                                    // 路由路径
	Icon       string `gorm:"column:icon;not null;comment:图标" json:"icon"`                                                                      // 图标
	MenuType   string `gorm:"column:menu_type;comment:菜单类型:tab=选项卡,link=链接,iframe=Iframe" json:"menu_type"`                                     // 菜单类型:tab=选项卡,link=链接,iframe=Iframe
	URL        string `gorm:"column:url;not null;comment:Url" json:"url"`                                                                       // Url
	Component  string `gorm:"column:component;not null;comment:组件路径" json:"component"`                                                          // 组件路径
	Keepalive  int32  `gorm:"column:keepalive;not null;comment:缓存:0=关闭,1=开启" json:"keepalive"`                                                  // 缓存:0=关闭,1=开启
	Extend     string `gorm:"column:extend;not null;default:none;comment:扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单" json:"extend"` // 扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单
	Remark     string `gorm:"column:remark;not null;comment:备注" json:"remark"`                                                                  // 备注
	Weigh      int32  `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                                                                    // 权重
	Status     string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`                                              // 状态:0=禁用,1=启用
	UpdateTime int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"`                                                // 更新时间
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`                                                // 创建时间
}

func (*AdminRule) TableName() string {
	return TableNameAdminRule
}

type AdminRuleModel struct {
	BaseModel
}

func NewAdminRuleModel(sqlDB *gorm.DB) *AdminRuleModel {
	return &AdminRuleModel{
		BaseModel: BaseModel{
			TableName:        TableNameAdminRule,
			Key:              "id",
			QuickSearchField: "title",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *AdminRuleModel) GetOne(ctx *gin.Context, id int32) (adminRule AdminRule, err error) {
	err = s.sqlDB.Table(s.TableName).Where("id=?", id).First(&adminRule).Error
	return
}

func (s *AdminRuleModel) List(ctx *gin.Context) (list []AdminRule, err error) {
	err = s.sqlDB.Table(s.TableName).Order("weigh desc,id desc").Find(&list).Error
	return
}

func (s *AdminRuleModel) Add(ctx *gin.Context, adminRule AdminRule) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(s.TableName).Create(&adminRule).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *AdminRuleModel) Edit(ctx *gin.Context, adminRule AdminRule) error {
	parent := AdminRule{}
	if err := s.sqlDB.Table(s.TableName).Where("id=?", adminRule.Pid).First(&parent).Error; err != nil {
		return err
	}

	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if parent.Pid == adminRule.ID {
		if err := tx.Table(s.TableName).Where("id=?", parent.ID).Update("pid", 0).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Table(s.TableName).Save(&adminRule).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *AdminRuleModel) Del(ctx *gin.Context, ids []int32) error {
	var subIds []int32
	if err := s.sqlDB.Table(s.TableName).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
		return err
	}

	for _, v := range subIds {
		if !slices.Contains(ids, v) {
			return cErr.BadRequest("Please delete the child element first, or use batch deletion")
		}
	}

	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}

func (s *AdminRuleModel) GetRulePIds(ids []string) ([]int32, error) {
	pids := []int32{}
	err := s.sqlDB.Table(s.TableName).Where("id in ?", ids).Pluck("pid", &pids).Error
	return pids, err
}
