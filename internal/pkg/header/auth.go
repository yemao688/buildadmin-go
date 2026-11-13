package header

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

const adminAuthKey = "AdminAuth"

const userAuthKey = "UserAuth"

type AdminAuth struct {
	Version      string `form:"version"`
	Language     string `form:"language"`
	IsLogin      bool   `form:"is_login"`
	Id           int32  `form:"id"`
	Username     string `form:"username"`
	Token        string `form:"token"`
	IsSuperAdmin bool   `form:"is_super_admin"`
}

func GetAdminAuth(c *gin.Context) (adminAuth AdminAuth) {
	v, _ := c.Get(adminAuthKey)
	if v != nil {
		cast, ok := v.(AdminAuth)
		if !ok {
			panic(fmt.Sprintf("unexpected %s value type %T", adminAuthKey, v))
		}
		adminAuth = cast
	}
	return adminAuth
}

func SetAdminAuth(c *gin.Context, adminAuth AdminAuth) {
	c.Set(adminAuthKey, adminAuth)
}

type UserAuth struct {
	Version  string `form:"version"`
	Language string `form:"language"`
	IsLogin  bool   `form:"is_login"`
	Id       int32  `form:"id"`
	Token    string `form:"token"`
}

func GetUserAuth(c *gin.Context) (userAuth UserAuth) {
	v, _ := c.Get(userAuthKey)
	if v != nil {
		cast, ok := v.(UserAuth)
		if !ok {
			panic(fmt.Sprintf("unexpected %s value type %T", userAuthKey, v))
		}
		userAuth = cast
	}
	return userAuth
}

// SetUserAuth 把会员认证信息写入 gin 上下文，与 SetAdminAuth 对称；读取侧
// 统一走 GetUserAuth。
func SetUserAuth(c *gin.Context, userAuth UserAuth) {
	c.Set(userAuthKey, userAuth)
}
