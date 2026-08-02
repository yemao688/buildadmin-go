package router

import (
	"go-build-admin/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Registrar 是 admin 渠道模块注册器的最小契约（与根 router.RouteRegistrar
// 的 admin 分支结构一致，独立声明以避免渠道包反向依赖根装配件）。
type Registrar interface {
	Register(r gin.IRoutes)
	Capabilities() []middleware.AtomicRoute
}
