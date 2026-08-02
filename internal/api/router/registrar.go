package router

import (
	"github.com/gin-gonic/gin"
)

// Registrar 是 api 渠道模块注册器的最小契约（与根 router.RouteRegistrar
// 的 api 分支结构一致，独立声明以避免渠道包反向依赖根装配件）。
// api 渠道不参与 AtomicRoute 能力注册，故只要求 Register。
type Registrar interface {
	Register(r gin.IRoutes)
}
