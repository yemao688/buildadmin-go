package handler

import (
	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type SensitiveDataLogHandler struct {
	log               *zap.Logger
	config            *conf.Configuration
	sensitiveDataLogM *securitymodel.SecuritySensitiveDataLogRepository
	svc               *service.SecuritySensitiveDataLogService
}

func NewSensitiveDataLogHandler(log *zap.Logger, config *conf.Configuration, sensitiveDataLogM *securitymodel.SecuritySensitiveDataLogRepository, svc *service.SecuritySensitiveDataLogService) *SensitiveDataLogHandler {
	return &SensitiveDataLogHandler{
		log:               log,
		config:            config,
		sensitiveDataLogM: sensitiveDataLogM,
		svc:               svc,
	}
}

func (h *SensitiveDataLogHandler) Index(ctx *gin.Context) {
	result, total, err := h.sensitiveDataLogM.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

func (h *SensitiveDataLogHandler) Info(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	result, err := h.sensitiveDataLogM.GetOne(ctx, int32(id)) //nolint:gosec // G115: 后台主键参数由 com.StrTo(...).MustInt() 解析，对应数据库主键列 int(11)，业务 ID 不超过 int32 上界（敏感数据日志 id）
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	response.Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *SensitiveDataLogHandler) Del(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.sensitiveDataLogM.Del(ctx, params.Ids); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *SensitiveDataLogHandler) Rollback(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	if err := h.svc.Rollback(ctx.Request.Context(), params.Ids); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}
