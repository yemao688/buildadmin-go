// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CommonRegistrar struct {
	handler *CommonHandler
}

func NewCommonRegistrar(handler *CommonHandler) *CommonRegistrar {
	return &CommonRegistrar{handler: handler}
}

func (r *CommonRegistrar) Group() string { return "api" }

func (r *CommonRegistrar) Register(g gin.IRoutes) {
	g.GET("common/captcha", r.handler.Captcha)
	g.GET("common/clickCaptcha", r.handler.ClickCaptcha)
	g.POST("common/checkClickCaptcha", r.handler.CheckClickCaptcha)
	g.POST("common/refreshToken", r.handler.RefreshToken)
}

func (r *CommonRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
