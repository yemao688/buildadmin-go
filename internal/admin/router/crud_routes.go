package router

import (
	"github.com/gin-gonic/gin"
)

// CRUDHandler 是 CRUD 模块路由注册器约定的控制器接口。
// 生成器产出的 handler 通过该接口自动注册标准 CRUD 路由。
type CRUDHandler interface {
	Index(*gin.Context)
	Add(*gin.Context)
	One(*gin.Context)
	Edit(*gin.Context)
	Del(*gin.Context)
	Sortable(*gin.Context)
}

// CRUDRoutes 为标准 CRUD 模块注册 <name>/index, <name>/add, <name>/one,
// <name>/edit, <name>/del, <name>/sortable 六条路由。
func CRUDRoutes(r gin.IRoutes, name string, h CRUDHandler) {
	r.GET(name+"/index", h.Index)
	r.POST(name+"/add", h.Add)
	r.GET(name+"/edit", h.One)
	r.POST(name+"/edit", h.Edit)
	r.DELETE(name+"/del", h.Del)
	r.POST(name+"/sortable", h.Sortable)
}