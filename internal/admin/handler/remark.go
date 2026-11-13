package handler

import (
	"strings"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin"
)

// remark 从当前 /admin/<controller>/<action> 路径反查规则备注。路径解析
// 留在 handler，规则读取走 AdminRuleRepository，避免 handler 直接持有
// *gorm.DB。
func remark(ctx *gin.Context, adminRuleM *adminmodel.AdminRuleRepository) string {
	name := ctx.Request.URL.Path
	name = strings.ReplaceAll(name, ".", "/")
	name = strings.Replace(name, "/admin/", "", 1)
	slashIndex := strings.LastIndex(name, "/")
	if slashIndex < 0 {
		return ""
	}
	ruleRemark, err := adminRuleM.RemarkByNames(ctx.Request.Context(), []string{name[:slashIndex], name[slashIndex+1:]})
	if err != nil {
		return ""
	}
	return util.Lang(ctx, ruleRemark, nil)
}
