package handler

import (
	"encoding/json"
	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/pkg/validator"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type DataRecycleLogHandler struct {
	Base
	log             *zap.Logger
	config          *conf.Configuration
	dataRecycleLogM *securitymodel.DataRecycleLogRepository
}

func NewDataRecycleLogHandler(log *zap.Logger, config *conf.Configuration, dataRecycleLogM *securitymodel.DataRecycleLogRepository) *DataRecycleLogHandler {
	return &DataRecycleLogHandler{
		Base:            NewBase(dataRecycleLogM),
		log:             log,
		config:          config,
		dataRecycleLogM: dataRecycleLogM,
	}
}

func (h *DataRecycleLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	result, total, err := h.dataRecycleLogM.List(ctx)
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

func (h *DataRecycleLogHandler) Info(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	dataRecycleLog, err := h.dataRecycleLogM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	type Result struct {
		model.SecurityDataRecycleLog
		Data map[string]any `json:"data"`
	}

	result := Result{}
	copier.Copy(&result, dataRecycleLog)
	if err := json.Unmarshal([]byte(dataRecycleLog.Data), &result.Data); err != nil {
		FailByErr(ctx, err)
		return
	}

	Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *DataRecycleLogHandler) Del(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if err := h.dataRecycleLogM.Del(ctx, params.Ids); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleLogHandler) Restore(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	if err := h.dataRecycleLogM.Restore(ctx, params.Ids); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
