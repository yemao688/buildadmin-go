package middleware

import (
	adminauth "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	middlewarecore "buildadmin-go/internal/middleware"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/token"
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
			middlewarecore.AbortMissingToken(c)
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
		// H8: one-time closure self-row verification for this request's
		// actor. The data-scope enforcer reads the cached result to skip the
		// per-query self-EXISTS subquery. On verification error the marker is
		// left unset and the enforcer falls back to the inline guard with
		// identical fail-closed semantics; unrestricted actors bypass scope
		// entirely and need no marker.
		if !actor.Unrestricted {
			if ok, cerr := m.authM.HasClosureSelfRow(actor.AdminID); cerr == nil {
				data_scope.MarkSelfRowChecked(c, ok)
			}
		}
		header.SetAdminAuth(c, authParam)
	}
}
