package middleware

import (
	cErr "go-build-admin/app/pkg/error"
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
			msg := utils.Lange(c, "No permission request", nil)
			c.JSON(http.StatusForbidden, map[string]interface{}{
				"code": cErr.Forbidden,
				"data": nil,
				"msg":  msg,
				"time": 0,
			})
			c.Abort()
			return
		}
	}
}
