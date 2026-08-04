package middleware

import (
	adminauth "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	middlewarecore "buildadmin-go/internal/middleware"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/token"
	"buildadmin-go/internal/pkg/util"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Login struct {
	config      *conf.Configuration
	tokenHelper *token.TokenHelper
	authM       *adminauth.AuthRepository
}

func NewLogin(config *conf.Configuration, tokenHelper *token.TokenHelper, authM *adminauth.AuthRepository) *Login {
	return &Login{
		config:      config,
		tokenHelper: tokenHelper,
		authM:       authM,
	}
}

func (m *Login) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 免登录 action（NoNeedLoginer 声明）：跳过整个登录校验，不注入
		// auth/actor，由 handler 自行处理无登录态。语义对齐 PHP noNeedLogin。
		if route, action, ok := middlewarecore.NormalizeRouteAction(c.FullPath()); ok && IsNoNeedLogin(route, action) {
			c.Next()
			return
		}

		tokenStr := c.Request.Header.Get("batoken")
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

		tokenData, err := m.tokenHelper.GetFor(tokenStr, "admin")
		if err != nil {
			middlewarecore.AbortLogin(c, err)
			return
		}
		username, enabled := m.authM.GetEnabledAdmin(tokenData.UserID)
		if !enabled {
			middlewarecore.AbortLogin(c, cErr.Unauthorized("Please login first"))
			return
		}
		language := c.GetHeader("Accept-Language")
		authParam := header.AdminAuth{
			Version:      "",
			Language:     language,
			IsLogin:      true,
			Id:           tokenData.UserID,
			Username:     username,
			Token:        tokenStr,
			IsSuperAdmin: m.authM.IsSuperAdmin(tokenData.UserID),
		}
		var actor data_scope.Actor
		if authParam.IsSuperAdmin {
			actor, err = data_scope.NewUnrestrictedActor(tokenData.UserID)
		} else {
			actor, err = data_scope.NewActor(tokenData.UserID)
		}
		if err != nil || data_scope.SetActor(c, actor) != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "msg": "invalid authenticated actor"})
			return
		}
		header.SetAdminAuth(c, authParam)
	}
}
