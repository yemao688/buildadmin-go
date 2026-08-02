package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const installCompleteMessage = "The system has completed installation. If you need to reinstall, please delete the install.lock file first"

const installCompletePath = "/api/install/commandExecComplete"

// InstallGuard blocks the installer after the installation lock exists. The
// installer API follows the application's existing business-error response
// convention: HTTP 200 with a 403 business code.
func InstallGuard(lockPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == installCompletePath {
			c.Next()
			return
		}
		if !isInstallPath(c.Request.URL.Path) || !pathExists(lockPath) {
			c.Next()
			return
		}

		if isInstallPagePath(c.Request.URL.Path) {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": http.StatusForbidden,
			"data": nil,
			"msg":  installCompleteMessage,
			"time": 0,
		})
		c.Abort()
	}
}

func isInstallPath(path string) bool {
	return isInstallPagePath(path) || path == "/api/install" || strings.HasPrefix(path, "/api/install/")
}

func isInstallPagePath(path string) bool {
	return path == "/install" || strings.HasPrefix(path, "/install/")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
