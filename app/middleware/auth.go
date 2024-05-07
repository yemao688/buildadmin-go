package middleware

import (
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Auth struct {
	config      *conf.Configuration
	tokenHelper *token.TokenHelper
}

func NewAuth(config *conf.Configuration, tokenHelper *token.TokenHelper) *Auth {
	return &Auth{
		config:      config,
		tokenHelper: tokenHelper,
	}
}

func (m *Auth) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.Request.Header.Get("Authorization")
		if tokenStr == "" {
			msg := utils.Lange(c, "missing Authorization header", nil)
			c.JSON(http.StatusOK, map[string]interface{}{
				"code": http.StatusUnauthorized,
				"data": nil,
				"msg":  msg,
				"time": 0,
			})
			c.Abort()
			return
		}

		tokenData, err := m.tokenHelper.Get(tokenStr, true)
		if err != nil {
			msg := utils.Lange(c, err.Error(), nil)
			c.JSON(http.StatusOK, map[string]interface{}{
				"code": http.StatusUnauthorized,
				"data": nil,
				"msg":  msg,
				"time": 0,
			})
			c.Abort()
			return
		}
		language := c.GetHeader("Accept-Language")
		authParam := header.AdminAuth{
			Version:   "",
			Language:  language,
			IsLogin:   true,
			Id:        tokenData.UserID,
			Token:     tokenStr,
			Timestamp: time.Now().Unix(),
		}
		c.Set("auth", authParam)
	}
}
