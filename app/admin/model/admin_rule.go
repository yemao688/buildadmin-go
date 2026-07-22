package model

import (
	"context"
	"errors"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"
	"slices"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

type AdminRuleModel struct {
	BaseModel
}

func NewAdminRuleModel(sqlDB *gorm.DB, config *conf.Configuration) *AdminRuleModel {
	return &AdminRuleModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "admin_rule",
			Key:              "id",
			QuickSearchField: "title",
			sqlDB:            sqlDB,
		},
	}
}

func (s *AdminRuleModel) GetOne(ctx *gin.Context, id int32) (adminRule AdminRule, err error) {
	err = s.DBFor(ctx).Where("id=?", id).Take(&adminRule).Error
	return
}

func (s *AdminRuleModel) List(ctx *gin.Context) (list []AdminRule, err error) {
	err = s.DBFor(ctx).Model(&AdminRule{}).Order("weigh desc,id desc").Find(&list).Error
	return
}

func (s *AdminRuleModel) Add(ctx *gin.Context, adminRule AdminRule) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Create(&adminRule)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("create failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *AdminRuleModel) Edit(ctx *gin.Context, adminRule AdminRule) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		parent := AdminRule{}
		if adminRule.Pid > 0 {
			if err := tx.Where("id=?", adminRule.Pid).First(&parent).Error; err != nil {
				return err
			}
		}
		if parent.Pid == adminRule.ID {
			result := tx.Model(&AdminRule{}).Where("id=?", parent.ID).Update("pid", 0)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return cErr.BadRequest("parent update failed")
			}
		}
		result := tx.Save(&adminRule)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("update failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *AdminRuleModel) Del(ctx *gin.Context, ids []int32) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var subIds []int32
		if err := tx.Model(&AdminRule{}).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
			return err
		}
		for _, v := range subIds {
			if !slices.Contains(ids, v) {
				return cErr.BadRequest("Please delete the child element first, or use batch deletion")
			}
		}
		query := tx.Model(&AdminRule{}).Where(" id in ? ", ids)
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

func (s *AdminRuleModel) GetRulePIds(ids []string, contexts ...*gin.Context) ([]int32, error) {
	var ctx *gin.Context
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	pids := []int32{}
	err := s.DBFor(ctx).Model(&AdminRule{}).Where("id in ?", ids).Pluck("pid", &pids).Error
	return pids, err
}

// crud 删除菜单
// 对齐上游 Menu::delete:菜单不存在时视为已删除(幂等);删除后递归清理
// 已无子级的父级目录。
func (s *AdminRuleModel) Delete(path string, recursion bool) error {
	return s.Transaction(context.Background(), func(tx *gorm.DB) error {
		var deleteRule func(string) error
		deleteRule = func(name string) error {
			var adminRule AdminRule
			err := tx.Where(" name = ? ", name).Take(&adminRule).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			var list []AdminRule
			if err := tx.Model(&AdminRule{}).Where(" pid = ? ", adminRule.ID).Find(&list).Error; err != nil {
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
				if err := tx.Model(&AdminRule{}).Where(" id = ? ", adminRule.ID).Delete(nil).Error; err != nil {
					return err
				}
				// 父级目录已无子级时一并删除
				var parent AdminRule
				if err := tx.Take(&parent, adminRule.Pid).Error; err == nil {
					var childCount int64
					if err := tx.Model(&AdminRule{}).Where(" pid = ? ", parent.ID).Count(&childCount).Error; err != nil {
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
