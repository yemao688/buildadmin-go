// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type TestBuildRegistrar struct {
	handler *TestBuildHandler
}

func NewTestBuildRegistrar(handler *TestBuildHandler) *TestBuildRegistrar {
	return &TestBuildRegistrar{handler: handler}
}

const testBuildRoute = "testBuild"

func (r *TestBuildRegistrar) Group() string { return "admin" }

func (r *TestBuildRegistrar) Register(g gin.IRoutes) {
	g.GET(testBuildRoute+"/index", r.handler.Index)
	g.POST(testBuildRoute+"/add", r.handler.Add)
	g.GET(testBuildRoute+"/edit", r.handler.One)
	g.POST(testBuildRoute+"/edit", r.handler.Edit)
	g.DELETE(testBuildRoute+"/del", r.handler.Del)
}

func (r *TestBuildRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
