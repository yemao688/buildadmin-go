package handler

import (
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/common/model"

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
		"remark": h.GetRemark(ctx),
	})
}

type Attachment struct {
	Topic    string `gorm:"column:topic;not null;comment:细目" json:"topic"`           // 细目
	URL      string `gorm:"column:url;not null;comment:物理路径" json:"url"`             // 物理路径
	Width    int32  `gorm:"column:width;not null;comment:宽度" json:"width"`           // 宽度
	Height   int32  `gorm:"column:height;not null;comment:高度" json:"height"`         // 高度
	Name     string `gorm:"column:name;not null;comment:原始名称" json:"name"`           // 原始名称
	Size     int32  `gorm:"column:size;not null;comment:大小" json:"size"`             // 大小
	Mimetype string `gorm:"column:mimetype;not null;comment:mime类型" json:"mimetype"` // mime类型
	Quote    int32  `gorm:"column:quote;not null;comment:上传(引用)次数" json:"quote"`     // 上传(引用)次数
	Storage  string `gorm:"column:storage;not null;comment:存储方式" json:"storage"`     // 存储方式
	Sha1     string `gorm:"column:sha1;not null;comment:sha1编码" json:"sha1"`         // sha1编码
}

func (v Attachment) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *AttachmentHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	result, err := h.attachmentM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *AttachmentHandler) Edit(ctx *gin.Context) {
	var params = struct {
		IDS
		Attachment
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	attachment, err := h.attachmentM.GetOne(ctx, int32(params.ID))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	copier.Copy(&attachment, params)
	err = h.attachmentM.Edit(ctx, attachment)
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
	Success(ctx, "")
}
