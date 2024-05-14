package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CurdLogHandler struct {
	Base
	log     *zap.Logger
	curdLog *model.CrudLogModel
	authM   *model.AuthModel
}

func NewCurdLogHandler(log *zap.Logger, curdLog *model.CrudLogModel, authM *model.AuthModel) *CurdLogHandler {
	return &CurdLogHandler{
		Base:    Base{currentM: curdLog},
		log:     log,
		curdLog: curdLog,
		authM:   authM,
	}
}

func (h *CurdLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}

	result, total, err := h.curdLog.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}
