package handler

import (
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/common/model"
	cErr "go-build-admin/app/pkg/error"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
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

func (h *AttachmentHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	result, total, err := h.attachmentM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

type Attachment struct {
}

func (v Attachment) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *AttachmentHandler) Add(ctx *gin.Context) {
	var params SensitiveData
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var sensitiveData model.Attachment
	copier.Copy(&sensitiveData, params)
	err := h.attachmentM.Add(ctx, sensitiveData)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AttachmentHandler) Edit(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	admin, err := h.attachmentM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//校验数据权限
	if !h.CheckDataLimit(ctx, admin.ID) {
		FailByErr(ctx, cErr.BadRequest("You have no permission"))
		return
	}

	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]interface{}{
			"row": admin,
		})
		return
	}

	var params = struct {
		IDS
		Attachment
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	copier.Copy(&admin, params)
	err = h.attachmentM.Edit(ctx, admin)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AttachmentHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
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
