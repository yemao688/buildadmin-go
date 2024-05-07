package header

import "github.com/gin-gonic/gin"

type AdminAuth struct {
	Version   string `form:"version"`
	Language  string `form:"language"`
	IsLogin   bool   `form:"is_login"`
	Id        int32  `form:"id"`
	Token     string `form:"token"`
	Timestamp int64  `form:"timestamp"`
}

func GetAdminAuth(c *gin.Context) (adminAuth AdminAuth) {
	v, _ := c.Get("AdminAuth")
	if v != nil {
		adminAuth = v.(AdminAuth)
	}
	return adminAuth
}
