package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type TestHandler struct {
	Base
	log   *zap.Logger
	testM *model.TestModel
}

func NewTestHandler(log *zap.Logger, testM *model.TestModel) *TestHandler {
	return &TestHandler{Base: Base{currentM: testM}, log: log, testM: testM}
}

func (h *TestHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	list, total, err := h.testM.List(ctx)
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

type TestParam struct {
	Editor     string             `json:"editor"`      // 富文本
	Status     validate.FlexInt32 `json:"status"`      // 状态:0=禁用,1=启用
	Weigh      validate.FlexInt32 `json:"weigh"`       // 权重
	UpdateTime validate.FlexInt64 `json:"update_time"` // 修改时间
	CreateTime validate.FlexInt64 `json:"create_time"` // 创建时间
}

func (h *TestHandler) Add(ctx *gin.Context) {
	var params TestParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.Test
	copier.Copy(&data, params)
	err := h.testM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *TestHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	type TestIDs struct {
		ID int32 `json:"id" binding:"required"`
	}
	var params = struct {
		TestIDs
		TestParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.testM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.testM.Edit(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *TestHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int32 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.testM.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
