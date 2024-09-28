package model

import (
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"
	"slices"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UserRule 会员菜单权限规则表
type UserRule struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                                                           // ID
	Pid          int32  `gorm:"column:pid;not null;comment:上级菜单" json:"pid"`                                                                                            // 上级菜单
	Type         string `gorm:"column:type;not null;default:menu;comment:类型:route=路由,menu_dir=菜单目录,menu=菜单项,nav_user_menu=顶栏会员菜单下拉项,nav=顶栏菜单项,button=页面按钮" json:"type"` // 类型:route=路由,menu_dir=菜单目录,menu=菜单项,nav_user_menu=顶栏会员菜单下拉项,nav=顶栏菜单项,button=页面按钮
	Title        string `gorm:"column:title;not null;comment:标题" json:"title"`                                                                                          // 标题
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                                                                                          // 规则名称
	Path         string `gorm:"column:path;not null;comment:路由路径" json:"path"`                                                                                          // 路由路径
	Icon         string `gorm:"column:icon;not null;comment:图标" json:"icon"`                                                                                            // 图标
	MenuType     string `gorm:"column:menu_type;not null;default:tab;comment:菜单类型:tab=选项卡,link=链接,iframe=Iframe" json:"menu_type"`                                      // 菜单类型:tab=选项卡,link=链接,iframe=Iframe
	URL          string `gorm:"column:url;not null;comment:Url" json:"url"`                                                                                             // Url
	Component    string `gorm:"column:component;not null;comment:组件路径" json:"component"`                                                                                // 组件路径
	NoLoginValid int32  `gorm:"column:no_login_valid;not null;comment:未登录有效:0=否,1=是" json:"no_login_valid"`                                                             // 未登录有效:0=否,1=是
	Extend       string `gorm:"column:extend;not null;default:none;comment:扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单" json:"extend"`                       // 扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单
	Remark       string `gorm:"column:remark;not null;comment:备注" json:"remark"`                                                                                        // 备注
	Weigh        int32  `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                                                                                          // 权重
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`                                                                    // 状态:0=禁用,1=启用
	UpdateTime   int64  `gorm:"autoCreateTime;column:update_time;comment:更新时间" json:"update_time"`                                                                      // 更新时间
	CreateTime   int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`                                                                      // 创建时间
}

type UserRuleModel struct {
	BaseModel
}

func NewUserRuleModel(sqlDB *gorm.DB, config *conf.Configuration) *UserRuleModel {
	return &UserRuleModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "user_rule",
			Key:              "id",
			QuickSearchField: "title",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *UserRuleModel) GetOne(ctx *gin.Context, id int32) (userRule UserRule, err error) {
	err = s.sqlDB.Where("id=?", id).First(&userRule).Error
	return
}

func (s *UserRuleModel) List(ctx *gin.Context) (list []UserRule, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Model(&UserRule{}).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *UserRuleModel) Add(ctx *gin.Context, userRule UserRule) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&userRule).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *UserRuleModel) Edit(ctx *gin.Context, userRule UserRule) error {
	parent := UserRule{}
	if userRule.Pid > 0 {
		if err := s.sqlDB.Where("id=?", userRule.Pid).First(&parent).Error; err != nil {
			return err
		}
	}

	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if parent.Pid == userRule.ID {
		if err := tx.Model(&UserRule{}).Where("id=?", parent.ID).Update("pid", 0).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Save(&userRule).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *UserRuleModel) Del(ctx *gin.Context, ids []int32) error {
	var subIds []int32
	if err := s.sqlDB.Model(&UserRule{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
		return err
	}

	for _, v := range subIds {
		if !slices.Contains(ids, v) {
			return cErr.BadRequest("Please delete the child element first, or use batch deletion")
		}
	}

	err := s.sqlDB.Model(&UserRule{}).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}

func (s *UserRuleModel) GetRulePIds(ids []string) ([]int32, error) {
	pids := []int32{}
	err := s.sqlDB.Model(&UserRule{}).Where("id in ?", ids).Pluck("pid", &pids).Error
	return pids, err
}
