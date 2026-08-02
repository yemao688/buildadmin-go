package middleware

import (
	"net/http"

	cErr "go-build-admin/internal/pkg/error"
	"go-build-admin/internal/utils"

	"github.com/gin-gonic/gin"
)

// abortLogin is kept for the user lane (user_login.go); AbortLogin is the
// shared exported form used by the admin middleware package.
func abortLogin(c *gin.Context, err error) {
	AbortLogin(c, err)
}

// AbortLogin rejects a request with an unauthorized payload shaped like the
// BuildAdmin login response.
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
