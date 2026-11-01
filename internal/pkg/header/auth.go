package header

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

const adminAuthKey = "AdminAuth"

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
	v, _ := c.Get("UserAuth")
	if v != nil {
		cast, ok := v.(UserAuth)
		if !ok {
			panic(fmt.Sprintf("unexpected UserAuth value type %T", v))
		}
		userAuth = cast
	}
	return userAuth
}
