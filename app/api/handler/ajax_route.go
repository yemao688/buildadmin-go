// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type AjaxRegistrar struct {
	handler *AjaxHandler
}

func NewAjaxRegistrar(handler *AjaxHandler) *AjaxRegistrar {
	return &AjaxRegistrar{handler: handler}
}

func (r *AjaxRegistrar) Group() string { return "api" }

func (r *AjaxRegistrar) Register(g gin.IRoutes) {
	middleware.RegisterPermissionExempt("ajax", "*")
	middleware.RegisterPermissionExempt("alioss", "callback")
	g.POST("ajax/area", r.handler.Area)
	g.POST("ajax/buildSuffixSvg", r.handler.BuildSuffixSvg)
	g.POST("ajax/upload", r.handler.Upload)
	g.POST("Alioss/callback", r.handler.AliossCallback)
}

func (r *AjaxRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
