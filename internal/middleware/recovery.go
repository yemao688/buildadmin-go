package middleware

import (
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/util"
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

func ServerError(c *gin.Context, err interface{}) {
	msg := "Internal Server Error"
	if gin.Mode() != gin.ReleaseMode {
		if e, ok := err.(error); ok {
			msg = e.Error()
		}
	}
	msg = util.Lang(c, msg, nil)
	response.WriteDirect(c, http.StatusInternalServerError, cErr.ServerError, msg, nil)
	c.Abort()
}

func CustomRecovery(loggerWriter *lumberjack.Logger) gin.HandlerFunc {
	return gin.RecoveryWithWriter(
		loggerWriter,
		ServerError)
}
