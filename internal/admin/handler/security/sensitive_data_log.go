package security

import (
	adminhandler "go-build-admin/internal/admin/handler"
	securitymodel "go-build-admin/internal/admin/model/security"
	"go-build-admin/internal/admin/validate"
	"go-build-admin/internal/conf"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type SensitiveDataLogHandler struct {
	adminhandler.Base
	log               *zap.Logger
	config            *conf.Configuration
	sensitiveDataLogM *securitymodel.SensitiveDataLogModel
}

func NewSensitiveDataLogHandler(log *zap.Logger, config *conf.Configuration, sensitiveDataLogM *securitymodel.SensitiveDataLogModel) *SensitiveDataLogHandler {
	return &SensitiveDataLogHandler{
		Base:              adminhandler.NewBase(sensitiveDataLogM),
		log:               log,
		config:            config,
		sensitiveDataLogM: sensitiveDataLogM,
	}
}

func (h *SensitiveDataLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		adminhandler.Success(ctx, data)
	}
	result, total, err := h.sensitiveDataLogM.List(ctx)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

func (h *SensitiveDataLogHandler) Info(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	result, err := h.sensitiveDataLogM.GetOne(ctx, int32(id))
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	adminhandler.Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *SensitiveDataLogHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(params, err))
		return
	}
	if err := h.sensitiveDataLogM.Del(ctx, params.Ids); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *SensitiveDataLogHandler) Rollback(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if err := h.sensitiveDataLogM.Rollback(ctx, params.Ids); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}
