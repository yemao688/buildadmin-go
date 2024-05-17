package handler

import "github.com/gin-gonic/gin"

var RegisteredRoutes = []Route{}

type Route struct {
	Method  string
	Path    string
	Handler string
}

func CollectRoutes(router *gin.Engine) {
	routesInfo := router.Routes()
	for _, v := range routesInfo {
		RegisteredRoutes = append(RegisteredRoutes, Route{
			Method:  v.Method,
			Path:    v.Path,
			Handler: v.Handler,
		})
	}
}

func GetAllRoutes() []Route {
	return RegisteredRoutes
}
