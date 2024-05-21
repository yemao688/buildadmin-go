package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CrudLogHandler struct {
	Base
	log     *zap.Logger
	crudLog *model.CrudLogModel
	authM   *model.AuthModel
}

func NewCrudLogHandler(log *zap.Logger, crudLog *model.CrudLogModel, authM *model.AuthModel) *CrudLogHandler {
	return &CrudLogHandler{
		Base:    Base{currentM: crudLog},
		log:     log,
		crudLog: crudLog,
		authM:   authM,
	}
}

func (h *CrudLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}

	result, total, err := h.crudLog.List(ctx)
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
