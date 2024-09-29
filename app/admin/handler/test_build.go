package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type TestBuildHandler struct {
	Base
	log        *zap.Logger
	testBuildM *model.TestBuildModel
}

func NewTestBuildHandler(log *zap.Logger, testBuildM *model.TestBuildModel) *TestBuildHandler {
	return &TestBuildHandler{Base: Base{currentM: testBuildM}, log: log, testBuildM: testBuildM}
}

func (h *TestBuildHandler) Index(ctx *gin.Context) {
	list, total, err := h.testBuildM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   list,
		"total":  total,
		"remark": "",
	})
}

type TestBuildParam struct {
}

func (h *TestBuildHandler) Add(ctx *gin.Context) {
	var params TestBuildParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	var data model.TestBuild
	copier.Copy(&data, params)
	err := h.testBuildM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *TestBuildHandler) Edit(ctx *gin.Context) {
	var params TestBuildParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var data model.TestBuild
	copier.Copy(&data, params)
	err := h.testBuildM.Edit(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *TestBuildHandler) Del(ctx *gin.Context) {
	var param validate.Ids
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.testBuildM.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
