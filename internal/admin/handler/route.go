package handler

import (
	"net/http"
	"strings"

	"go-build-admin/internal/middleware"

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

const (
	actionAdd  = "add"
	actionEdit = "edit"
	actionDel  = "del"
)

func CRUDRoutes(r gin.IRoutes, name string, h CRUDHandler) {
	r.GET(name+"/index", h.Index)
	r.POST(name+"/add", h.Add)
	r.GET(name+"/edit", h.One)
	r.POST(name+"/edit", h.Edit)
	r.DELETE(name+"/del", h.Del)
	r.POST(name+"/sortable", h.Sortable)
}

func CRUDCapabilities(name string) []middleware.AtomicRoute {
	route := capabilityRoute(name)
	return []middleware.AtomicRoute{
		{Route: route, Action: actionAdd, Method: http.MethodPost},
		{Route: route, Action: actionEdit, Method: http.MethodPost},
		{Route: route, Action: actionDel, Method: http.MethodDelete},
	}
}

func capabilityRoute(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, ".", "/"))
}

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
