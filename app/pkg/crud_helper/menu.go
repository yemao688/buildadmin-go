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
func CreateMenu(adminRuleM *model.AdminRuleModel, webViewsDir WebDir, tableComment string) {
	// menueName := GetMenuName(webViewsDir)

}
