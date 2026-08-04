package handler

import (
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AliossCallback 阿里云 OSS 直传回调：浏览器把文件 POST 到 OSS 后调用本接口
// 登记附件记录（storage=alioss，sha1 幂等去重）。
func (h *CommonHandler) AliossCallback(ctx *gin.Context) {
	var params upload.OSSCallback
	if err := ctx.ShouldBind(&params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	userAuth := header.GetUserAuth(ctx)
	result, err := h.uploadHelper.CompleteOSS(params, 0, userAuth.Id)
	if err != nil {
		h.log.Error("AliOSS callback failed", zap.Error(err))
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{"file": result})
}
