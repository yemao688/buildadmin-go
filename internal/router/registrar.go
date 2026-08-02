package router

import (
	"go-build-admin/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RouteRegistrar 由各业务模块的 handler 实现，把本模块的路由与原子写能力
// 声明在模块自己的文件里。InitRouter 按 Group() 把 registrar 分派给
// admin/api 渠道注册器（internal/admin/router、internal/api/router），
// 两个渠道包用各自更窄的 Registrar 接口承接，避免渠道包反向依赖根装配件。
type RouteRegistrar interface {
	Group() string // admin=后台分组, api=会员分组, root=引擎根
	Register(r gin.IRoutes)
	Capabilities() []middleware.AtomicRoute
}
