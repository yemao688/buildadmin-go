package middleware

import (
	"buildadmin-go/internal/api/service"
	"buildadmin-go/internal/conf"
	core "buildadmin-go/internal/middleware"
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
			core.AbortMissingToken(c)
			return
		}

		tokenData, err := m.tokenHelper.GetFor(tokenStr, "user")
		if err != nil {
			core.AbortLogin(c, err)
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
