package router

import (
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

// TableRegistrar 是标准 CRUD 模块的数据驱动注册器：路由名 + CRUDHandler，
// 固定注册 6 条标准路由并声明 add/edit/del 三个原子写能力。
// 需要额外端点或不同能力面的模块，继续使用独立 registrar 文件。
type TableRegistrar struct {
	route string
	h     CRUDHandler
}

// NewTableRegistrar 由生成器的每表构造器调用，route 使用模块路由常量。
func NewTableRegistrar(route string, h CRUDHandler) *TableRegistrar {
	return &TableRegistrar{route: route, h: h}
}

// Register 注册标准 CRUD 路由。
func (r *TableRegistrar) Register(g gin.IRoutes) {
	CRUDRoutes(g, r.route, r.h)
}

// Capabilities 声明标准 CRUD 的原子写能力。
func (r *TableRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(r.route)
}
