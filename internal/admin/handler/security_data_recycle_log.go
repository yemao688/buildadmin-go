package handler

import (
	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type DataRecycleLogHandler struct {
	Base
	log             *zap.Logger
	config          *conf.Configuration
	dataRecycleLogM *securitymodel.SecurityDataRecycleLogRepository
	svc             *service.SecurityDataRecycleLogService
}

func NewDataRecycleLogHandler(log *zap.Logger, config *conf.Configuration, dataRecycleLogM *securitymodel.SecurityDataRecycleLogRepository, svc *service.SecurityDataRecycleLogService) *DataRecycleLogHandler {
	return &DataRecycleLogHandler{
		Base:            NewBase(dataRecycleLogM),
		log:             log,
		config:          config,
		dataRecycleLogM: dataRecycleLogM,
		svc:             svc,
	}
}

func (h *DataRecycleLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		response.Success(ctx, data)
	}
	result, total, err := h.dataRecycleLogM.List(ctx)
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

func (h *DataRecycleLogHandler) Info(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	dataRecycleLog, err := h.dataRecycleLogM.GetOne(ctx, int32(id))
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	type Result struct {
		model.SecurityDataRecycleLog
		Data map[string]any `json:"data"`
	}

	result := Result{}
	copier.Copy(&result, dataRecycleLog)
	if err := json.Unmarshal([]byte(dataRecycleLog.Data), &result.Data); err != nil {
		response.FailByErr(ctx, err)
		return
	}

	response.Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *DataRecycleLogHandler) Del(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.dataRecycleLogM.Del(ctx, params.Ids); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *DataRecycleLogHandler) Restore(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	if err := h.svc.Restore(ctx.Request.Context(), params.Ids); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}
