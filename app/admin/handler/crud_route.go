// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type CrudRegistrar struct {
	handler *CrudHandler
}

func NewCrudRegistrar(handler *CrudHandler) *CrudRegistrar {
	return &CrudRegistrar{handler: handler}
}

func (r *CrudRegistrar) Group() string { return "admin" }

func (r *CrudRegistrar) Register(g gin.IRoutes) {
	g.GET("crud.Crud/databaseList", r.handler.DatabaseList)
	g.GET("crud.Crud/checkCrudLog", r.handler.CheckCrudLog)
	g.POST("crud.Crud/parseFieldData", r.handler.ParseFieldData)
	g.GET("crud.Crud/getFileData", r.handler.GetFileData)
	g.POST("crud.Crud/generateCheck", r.handler.GenerateCheck)
	g.POST("crud.Crud/generate", r.handler.Generate)
	g.POST("crud.Crud/logStart", r.handler.LogStart)
	g.POST("crud.Crud/delete", r.handler.Delete)
}

func (r *CrudRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
