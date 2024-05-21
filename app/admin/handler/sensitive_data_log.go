package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SensitiveDataLogHandler struct {
	Base
	log               *zap.Logger
	config            *conf.Configuration
	sensitiveDataLogM *model.SensitiveDataLogModel
}

func NewSensitiveDataLogHandler(log *zap.Logger, config *conf.Configuration, sensitiveDataLogM *model.SensitiveDataLogModel) *SensitiveDataLogHandler {
	return &SensitiveDataLogHandler{
		Base:              Base{currentM: sensitiveDataLogM},
		log:               log,
		config:            config,
		sensitiveDataLogM: sensitiveDataLogM,
	}
}

func (h *SensitiveDataLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	result, total, err := h.sensitiveDataLogM.List(ctx)
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

func (h *SensitiveDataLogHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	err := h.sensitiveDataLogM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
