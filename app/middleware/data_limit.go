package middleware

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/header"
	"go-build-admin/conf"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
)

type DataLimit struct {
	config *conf.Configuration
	authM  *model.AuthModel
}

func NewDataLimit(config *conf.Configuration, authM *model.AuthModel) *DataLimit {
	return &DataLimit{
		config: config,
		authM:  authM,
	}
}

/**
 *数据权限控制-获取有权限访问的管理员Ids
 *""=关闭
 *personal=仅限个人
 *allAuth=拥有某管理员所有的权限时
 *allAuthAndOthers=拥有某管理员所有的权限并且还有其他权限时
 *parent=上级分组中的管理员可查
 *"2"=指定分组中的管理员可查
 */
func (m *DataLimit) Handler(limitType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminAuth := header.GetAdminAuth(c)
		if limitType == "" || adminAuth.IsSuperAdmin {
			c.Set("dataLimitAdminIds", []int32{})
			return
		}

		adminIds := []int32{}
		if limitType == "parent" {
			// 取得当前管理员的下级分组
			parentGroups := m.authM.GetAdminChildGroups(adminAuth.Id)
			if len(parentGroups) > 0 {
				// 取得分组内的所有管理员
				adminIds = m.authM.GetGroupAdmins(parentGroups)
			}
		} else if limitType == "personal" {
			adminIds = append(adminIds, adminAuth.Id)
		} else if limitType == "allAuth" || limitType == "allAuthAndOthers" {
			// 取得拥有他所有权限的分组
			allAuthGroups, _ := m.authM.GetAllAuthGroups(limitType, adminAuth.Id)
			// 取得分组内的所有管理员
			adminIds = m.authM.GetGroupAdmins(allAuthGroups)
		} else {
			// 在组内，可查看所有，不在组内，可查看自己的
			if v, err := strconv.Atoi(limitType); err == nil && v > 0 {
				adminIds = m.authM.GetGroupAdmins([]string{limitType})
				if !slices.Contains(adminIds, adminAuth.Id) {
					adminIds = append(adminIds, adminAuth.Id)
				}
			}
		}
		c.Set("dataLimitAdminIds", adminIds)
	}
}
