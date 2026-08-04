package middleware

import (
	"net/http"

	"buildadmin-go/internal/api/service"
	"buildadmin-go/internal/conf"
	core "buildadmin-go/internal/middleware"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/token"
	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin"
)

type UserLogin struct {
	config      *conf.Configuration
	tokenHelper *token.TokenHelper
	authM       *service.MemberService
}

func NewUserLogin(config *conf.Configuration, tokenHelper *token.TokenHelper, authM *service.MemberService) *UserLogin {
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
			msg := util.Lang(c, "missing Authorization header", nil)
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
			core.AbortLogin(c, err)
			return
		}
		if !m.authM.IsEnabledUser(tokenData.UserID) {
			core.AbortLogin(c, cErr.Unauthorized("Please login first"))
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
		if err := m.authM.ValidateUserToken(tokenData.UserID, util.GetClientIP(c)); err != nil {
			core.AbortLogin(c, err)
			return
		}
		c.Set("UserAuth", authParam)
	}
}
