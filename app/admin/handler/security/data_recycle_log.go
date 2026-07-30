package security

import (
	"encoding/json"
	adminhandler "go-build-admin/app/admin/handler"
	securitymodel "go-build-admin/app/admin/model/security"
	"go-build-admin/app/admin/validate"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type DataRecycleLogHandler struct {
	adminhandler.Base
	log             *zap.Logger
	config          *conf.Configuration
	dataRecycleLogM *securitymodel.DataRecycleLogModel
}

func NewDataRecycleLogHandler(log *zap.Logger, config *conf.Configuration, dataRecycleLogM *securitymodel.DataRecycleLogModel) *DataRecycleLogHandler {
	return &DataRecycleLogHandler{
		Base:            adminhandler.NewBase(dataRecycleLogM),
		log:             log,
		config:          config,
		dataRecycleLogM: dataRecycleLogM,
	}
}

func (h *DataRecycleLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		adminhandler.Success(ctx, data)
	}
	result, total, err := h.dataRecycleLogM.List(ctx)
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

func (h *DataRecycleLogHandler) Info(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	dataRecycleLog, err := h.dataRecycleLogM.GetOne(ctx, int32(id))
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	type Result struct {
		securitymodel.SecurityDataRecycleLog
		Data map[string]any `json:"data"`
	}

	result := Result{}
	copier.Copy(&result, dataRecycleLog)
	if err := json.Unmarshal([]byte(dataRecycleLog.Data), &result.Data); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	adminhandler.Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *DataRecycleLogHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(params, err))
		return
	}
	if err := h.dataRecycleLogM.Del(ctx, params.Ids); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *DataRecycleLogHandler) Restore(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if err := h.dataRecycleLogM.Restore(ctx, params.Ids); err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}
