// Package route 持有 admin 渠道已注册路由的快照，供 security 模块等组件
// 获取路由列表用于功能配置。收集时点由组合根在 InitRouter 末尾触发。
package route

import (
	"github.com/gin-gonic/gin"
)

// Route 保存一条路由的方法、路径与处理器名。
type Route struct {
	Method  string
	Path    string
	Handler string
}

// registeredRoutes 是当前收集到的路由快照，只在包内被 CollectRoutes 写入。
var registeredRoutes = []Route{}

// CollectRoutes 从 gin 引擎读取全部已注册路由存入包内路由快照。
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
	registeredRoutes = routes
}

// GetAllRoutes 返回当前路由快照。
func GetAllRoutes() []Route {
	return registeredRoutes
}
