package middleware

import (
	"net/http"

	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/utils"

	"github.com/gin-gonic/gin"
)

// AbortLogin rejects a request with an unauthorized payload shaped like the
// BuildAdmin login response. Shared by the admin middleware package and the
// api user-login middleware (api imports the global middleware package).
func AbortLogin(c *gin.Context, err error) {
	code := http.StatusUnauthorized
	message := "Please login first"
	if v, ok := err.(*cErr.Error); ok {
		code = v.ErrorCode()
		message = v.Error()
	}
	c.JSON(http.StatusOK, map[string]interface{}{
		"code": code,
		"data": map[string]any{
			"type": "need login",
		},
		"msg":  utils.Lang(c, message, nil),
		"time": 0,
	})
	c.Abort()
}
