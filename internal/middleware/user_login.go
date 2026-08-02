package middleware

import (
	"go-build-admin/internal/common/member"
	cErr "go-build-admin/internal/pkg/error"
	"go-build-admin/internal/pkg/header"
	"go-build-admin/internal/pkg/token"
	"go-build-admin/internal/conf"
	"go-build-admin/internal/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserLogin struct {
	config      *conf.Configuration
	tokenHelper *token.TokenHelper
	authM       *member.Service
}

func NewUserLogin(config *conf.Configuration, tokenHelper *token.TokenHelper, authM *member.Service) *UserLogin {
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

		tokenData, err := m.tokenHelper.GetFor(tokenStr, "user")
		if err != nil {
			abortLogin(c, err)
			return
		}
		if !m.authM.IsEnabledUser(tokenData.UserID) {
			abortLogin(c, cErr.Unauthorized("Please login first"))
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
		if err := m.authM.ValidateUserToken(c, tokenData.UserID, c.ClientIP()); err != nil {
			abortLogin(c, err)
			return
		}
		c.Set("UserAuth", authParam)
	}
}
