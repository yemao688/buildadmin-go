package crud_helper

import (
	"go-build-admin/app/admin/model"
	"strings"
)

func GetMenuName(webDir WebDir) string {
	content := webDir.OriginalLastName
	if len(webDir.Path) > 0 {
		content = strings.Join(webDir.Path, "/") + "/" + content
	}
	return content
}

// 创建菜单
func CreateMenu(adminRuleM *model.AdminRuleModel, webViewsDir WebDir, tableComment string) error {
	menuName := GetMenuName(webViewsDir)
	adminRule := model.AdminRule{}
	result := adminRuleM.DB().Table("ba_admin_rule").Where("name=?", menuName).First(&adminRule)
	if result.RowsAffected == 0 {
		var pid int32
		for _, v := range webViewsDir.Path {
			parentRule := model.AdminRule{}
			result = adminRuleM.DB().Table("ba_admin_rule").Where("name=?", v).First(&parentRule)
			if result.RowsAffected == 1 {
				pid = parentRule.ID
				continue
			}

			newRule := model.AdminRule{
				Pid:   pid,
				Type:  "menu_dir",
				Title: v,
				Name:  v,
				Path:  v,
			}
			if err := adminRuleM.DB().Table("ba_admin_rule").Create(&newRule).Error; err != nil {
				return err
			}
			pid = newRule.ID
		}

		//建立菜单
		title := webViewsDir.OriginalLastName
		if tableComment != "" {
			title = tableComment
		}

		component := strings.ReplaceAll(webViewsDir.Views, "\\", "/")
		component = strings.ReplaceAll(component, "web/src", "/src") + "/index.vue"
		menuRule := &model.AdminRule{
			Pid:       pid,
			Type:      "menu",
			Title:     title,
			Name:      menuName,
			Path:      menuName,
			MenuType:  "tab",
			Component: component,
			Status:    "1",
		}
		if err := adminRuleM.DB().Table("ba_admin_rule").Create(&menuRule).Error; err != nil {
			return err
		}

		for _, v := range menuChildren {
			rule := model.AdminRule{}
			name := menuName + v.Name
			result = adminRuleM.DB().Table("ba_admin_rule").Where("name=?", name).First(&rule)
			if result.RowsAffected == 1 {
				adminRuleM.DB().Table("ba_admin_rule").Where("id=?", rule.ID).Updates(map[string]any{
					"pid":    menuRule.ID,
					"Type":   v.Type,
					"Title":  v.Title,
					"Status": v.Status,
				})
			} else {
				adminRuleM.DB().Table("ba_admin_rule").Create(&model.AdminRule{
					Pid:    menuRule.ID,
					Type:   v.Type,
					Title:  v.Title,
					Name:   name,
					Status: v.Status,
				})
			}
		}
	}
	return nil
}
