package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type TestBuildHandler struct {
	log        *zap.Logger
	testBuildM *model.TestBuildModel
}

func NewTestBuildHandler(log *zap.Logger, testBuildM *model.TestBuildModel) *TestBuildHandler {
	return &TestBuildHandler{log: log, testBuildM: testBuildM}
}

func (h *TestBuildHandler) Index(ctx *gin.Context) {
	result, err := h.testBuildM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

type ParamTestBuild struct {
}

func (h *TestBuildHandler) Add(ctx *gin.Context) {
	var param ParamTestBuild
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validator.GetError(param, err))
		return
	}
	data := model.TestBuild{
		ID:        0,
		Image:     "",
		File:      "",
		Radio:     "",
		Checkbox:  "",
		Select:    "",
		Switch:    false,
		Editor:    "",
		Textarea:  "",
		Float:     0,
		Password:  "",
		Array:     "",
		Icon:      "",
		BannerIds: "",
	}
	err := h.testBuildM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

type TestBuild struct {
}

func (h *TestBuildHandler) Edit(ctx *gin.Context) {
	var params TestBuild
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
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
	if err := ctx.ShouldBindJSON(&param); err != nil {
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
