package handler

import (
	"go-build-admin/app/pkg/clickcaptcha"
	cErr "go-build-admin/app/pkg/error"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CommonHandler struct {
	log          *zap.Logger
	clickCaptcha *clickcaptcha.ClickCaptcha
}

func NewCommonHandler(log *zap.Logger, clickCaptcha *clickcaptcha.ClickCaptcha) *CommonHandler {
	return &CommonHandler{log: log, clickCaptcha: clickCaptcha}
}

// 图形验证码
func (h *CommonHandler) Captcha(ctx *gin.Context) {

	Success(ctx, "")
}

// 点选验证码
func (h *CommonHandler) ClickCaptcha(ctx *gin.Context) {
	var params struct {
		Id string `form:"id" json:"id" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, err)
		return
	}
	result, err := h.clickCaptcha.Create(ctx, params.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

// 点选验证码检查
func (h *CommonHandler) CheckClickCaptcha(ctx *gin.Context) {
	var params struct {
		Id    string `json:"id" binding:"required"`
		Info  string `json:"info" binding:"required"`
		Unset bool   `json:"unset"`
	}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, err)
		return
	}
	if !h.clickCaptcha.Check(params.Id, params.Info, params.Unset) {
		FailByErr(ctx, cErr.BadRequest("Validate fail"))
		return
	}
	Success(ctx, "")
}

func (h *CommonHandler) RefreshToken(ctx *gin.Context) {

	Success(ctx, "")
}
