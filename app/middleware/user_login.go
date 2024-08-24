package middleware

import (
	"go-build-admin/app/common/model"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserLogin struct {
	config      *conf.Configuration
	tokenHelper *token.TokenHelper
	authM       *model.AuthModel
}

func NewUserLogin(config *conf.Configuration, tokenHelper *token.TokenHelper, authM *model.AuthModel) *UserLogin {
	return &UserLogin{
		config:      config,
		tokenHelper: tokenHelper,
		authM:       authM,
	}
}

func (m *UserLogin) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.Request.Header.Get("ba-user-token")
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

		tokenData, err := m.tokenHelper.Get(tokenStr)
		if err != nil {
			if v, ok := err.(*cErr.Error); ok {
				msg := utils.Lang(c, v.Error(), nil)
				c.JSON(http.StatusOK, map[string]interface{}{
					"code": v.ErrorCode(),
					"data": map[string]any{
						"type": "need login",
					},
					"msg":  msg,
					"time": 0,
				})
			}

			c.Abort()
			return
		}
		language := c.GetHeader("Accept-Language")
		authParam := header.UserAuth{
			Version:  "",
			Language: language,
			IsLogin:  true,
			Id:       tokenData.UserID,
			Token:    tokenStr,
		}
		c.Set("UserAuth", authParam)
	}
}
