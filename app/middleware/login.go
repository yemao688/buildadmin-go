package middleware

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"
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
			abortLogin(c, err)
			return
		}
		if !m.authM.IsEnabledAdmin(tokenData.UserID) {
			abortLogin(c, cErr.Unauthorized("Please login first"))
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
		c.Set("AdminAuth", authParam)
	}
}

func abortLogin(c *gin.Context, err error) {
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
