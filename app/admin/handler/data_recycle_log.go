package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DataRecycleLogHandler struct {
	Base
	log             *zap.Logger
	config          *conf.Configuration
	dataRecycleLogM *model.DataRecycleLogModel
}

func NewDataRecycleLogHandler(log *zap.Logger, config *conf.Configuration, dataRecycleLogM *model.DataRecycleLogModel) *DataRecycleLogHandler {
	return &DataRecycleLogHandler{
		Base:            Base{currentM: dataRecycleLogM},
		log:             log,
		config:          config,
		dataRecycleLogM: dataRecycleLogM,
	}
}

// 还原
func (h *DataRecycleLogHandler) Restore(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.dataRecycleLogM.Restore(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
}

// 详情
func (h *DataRecycleLogHandler) Info(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *DataRecycleLogHandler) jsonToArray(ctx *gin.Context) {
	Success(ctx, "")
}
