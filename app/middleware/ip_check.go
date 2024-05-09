package middleware

import (
	"go-build-admin/utils"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
)

func IpCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		store := persistence.NewInMemoryStore(time.Minute)

		var noAccessIp string
		store.Get("no_access_ip", noAccessIp)
		if noAccessIp != "" && strings.Contains(noAccessIp, clientIP) {
			msg := utils.Lang(c, "No permission request", nil)
			c.JSON(http.StatusOK, map[string]interface{}{
				"code": http.StatusForbidden,
				"data": nil,
				"msg":  msg,
				"time": 0,
			})
			c.Abort()
			return
		}
	}
}
