package router

import (
	"go-build-admin/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RouteRegistrar 由各业务模块的 handler 实现，把本模块的路由与原子写能力
// 声明在模块自己的文件里，避免 InitRouter 成为唯一注册点。
type RouteRegistrar interface {
	Group() string // admin=后台分组, api=会员分组, root=引擎根
	Register(r gin.IRoutes)
	Capabilities() []middleware.AtomicRoute
}
