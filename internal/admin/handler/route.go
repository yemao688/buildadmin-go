package handler

import (
	"github.com/gin-gonic/gin"
)

var RegisteredRoutes = []Route{}

type Route struct {
	Method  string
	Path    string
	Handler string
}

type CRUDHandler interface {
	Index(*gin.Context)
	Add(*gin.Context)
	One(*gin.Context)
	Edit(*gin.Context)
	Del(*gin.Context)
	Sortable(*gin.Context)
}

func CRUDRoutes(r gin.IRoutes, name string, h CRUDHandler) {
	r.GET(name+"/index", h.Index)
	r.POST(name+"/add", h.Add)
	r.GET(name+"/edit", h.One)
	r.POST(name+"/edit", h.Edit)
	r.DELETE(name+"/del", h.Del)
	r.POST(name+"/sortable", h.Sortable)
}

// AtomicRoute capability declaration lives in internal/admin/router
// (CRUDCapabilities); route collection below stays here for the admin_rule
// coverage diagnostics driven from the composer root.
func CollectRoutes(router *gin.Engine) {
	routesInfo := router.Routes()
	routes := make([]Route, 0, len(routesInfo))
	for _, v := range routesInfo {
		routes = append(routes, Route{
			Method:  v.Method,
			Path:    v.Path,
			Handler: v.Handler,
		})
	}
	RegisteredRoutes = routes
}

func GetAllRoutes() []Route {
	return RegisteredRoutes
}
