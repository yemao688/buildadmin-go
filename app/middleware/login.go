package middleware

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Login struct {
	config      *conf.Configuration
	tokenHelper *token.TokenHelper
	authM       *model.AuthModel
}

func NewLogin(config *conf.Configuration, tokenHelper *token.TokenHelper, authM *model.AuthModel) *Login {
	return &Login{
		config:      config,
		tokenHelper: tokenHelper,
		authM:       authM,
	}
}

func (m *Login) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.Request.Header.Get("Authorization")
		if tokenStr == "" {
			msg := utils.Lang(c, "missing Authorization header", nil)
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
			msg := utils.Lang(c, err.Error(), nil)
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
			Version:      "",
			Language:     language,
			IsLogin:      true,
			Id:           tokenData.UserID,
			Token:        tokenStr,
			IsSuperAdmin: m.authM.IsSuperAdmin(tokenData.UserID),
		}
		c.Set("AdminAuth", authParam)
	}
}
