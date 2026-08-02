package middleware

import (
	adminauth "go-build-admin/internal/admin/repository/auth"
	"go-build-admin/internal/conf"
	middlewarecore "go-build-admin/internal/middleware"
	"go-build-admin/internal/pkg/data_scope"
	cErr "go-build-admin/internal/pkg/error"
	"go-build-admin/internal/pkg/header"
	"go-build-admin/internal/pkg/token"
	"go-build-admin/internal/utils"
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
		tokenStr := c.Request.Header.Get("batoken")
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
