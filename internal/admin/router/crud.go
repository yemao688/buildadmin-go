// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	adminmiddleware "buildadmin-go/internal/admin/middleware"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CrudRegistrar struct {
	handler *handler.CrudHandler
}

func NewCrudRegistrar(handler *handler.CrudHandler) *CrudRegistrar {
	return &CrudRegistrar{handler: handler}
}

func (r *CrudRegistrar) Group() string { return "admin" }

func (r *CrudRegistrar) Register(g gin.IRoutes) {
	adminmiddleware.RegisterHandlerExemptions("crud/crud", r.handler)
	g.GET("crud.Crud/databaseList", r.handler.DatabaseList)
	g.GET("crud.Crud/checkCrudLog", r.handler.CheckCrudLog)
	g.POST("crud.Crud/parseFieldData", r.handler.ParseFieldData)
	g.GET("crud.Crud/getFileData", r.handler.GetFileData)
	g.POST("crud.Crud/generateCheck", r.handler.GenerateCheck)
	g.POST("crud.Crud/generate", r.handler.Generate)
	g.POST("crud.Crud/logStart", r.handler.LogStart)
	g.POST("crud.Crud/delete", r.handler.Delete)
	g.POST("crud.Crud/uploadCompleted", r.handler.UploadCompleted)
}

func (r *CrudRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
