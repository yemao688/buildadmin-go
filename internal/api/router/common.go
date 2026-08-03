// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	api "buildadmin-go/internal/api/handler"

	"github.com/gin-gonic/gin"
)

type CommonRegistrar struct {
	handler *api.CommonHandler
}

func NewCommonRegistrar(handler *api.CommonHandler) *CommonRegistrar {
	return &CommonRegistrar{handler: handler}
}

func (r *CommonRegistrar) Register(g gin.IRoutes) {
	g.GET("common/captcha", r.handler.Captcha)
	g.GET("common/clickCaptcha", r.handler.ClickCaptcha)
	g.POST("common/checkClickCaptcha", r.handler.CheckClickCaptcha)
	g.POST("common/refreshToken", r.handler.RefreshToken)
}
