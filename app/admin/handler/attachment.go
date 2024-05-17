package handler

import (
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/common/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AttachmentHandler struct {
	Base
	log         *zap.Logger
	attachmentM *model.AttachmentModel
}

func NewAttachmentHandler(log *zap.Logger, attachmentM *model.AttachmentModel) *AttachmentHandler {
	return &AttachmentHandler{
		Base:        Base{currentM: attachmentM},
		log:         log,
		attachmentM: attachmentM,
	}
}

func (h *AttachmentHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.attachmentM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//删除文件TODO:

	Success(ctx, "")
}
