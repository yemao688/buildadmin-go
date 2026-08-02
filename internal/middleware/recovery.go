package middleware

import (
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/utils"
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

func ServerError(c *gin.Context, err interface{}) {
	msg := "Internal Server Error"
	if gin.Mode() != gin.ReleaseMode {
		if _, ok := err.(error); ok {
			msg = err.(error).Error()
		}
	}
	msg = utils.Lang(c, msg, nil)
	c.JSON(http.StatusInternalServerError, map[string]interface{}{
		"code": cErr.ServerError,
		"data": nil,
		"msg":  msg,
		"time": 0,
	})
	c.Abort()
}

func CustomRecovery(loggerWriter *lumberjack.Logger) gin.HandlerFunc {
	return gin.RecoveryWithWriter(
		loggerWriter,
		ServerError)
}
