// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type AttachmentRegistrar struct {
	handler *AttachmentHandler
}

func NewAttachmentRegistrar(handler *AttachmentHandler) *AttachmentRegistrar {
	return &AttachmentRegistrar{handler: handler}
}

const attachmentRoute = "routine.Attachment"

func (r *AttachmentRegistrar) Group() string { return "admin" }

func (r *AttachmentRegistrar) Register(g gin.IRoutes) {
	g.GET(attachmentRoute+"/index", r.handler.Index)
	g.GET(attachmentRoute+"/edit", r.handler.One)
	g.POST(attachmentRoute+"/edit", r.handler.Edit)
	g.DELETE(attachmentRoute+"/del", r.handler.Del)
}

func (r *AttachmentRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
