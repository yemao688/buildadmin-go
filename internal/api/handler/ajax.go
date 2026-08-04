package handler

import (
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// Upload 会员文件上传（本地存储），与 admin ajax/upload 对齐，属主为当前会员。
func (h *CommonHandler) Upload(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	userAuth := header.GetUserAuth(ctx)

	result, err := h.uploadHelper.Upload(ctx, upload.UploadParams{File: file}, 0, userAuth.Id)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{
		"file": result,
	})
}
