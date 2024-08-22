package handler

import (
	"go-build-admin/app/pkg/captcha"
	"go-build-admin/app/pkg/clickcaptcha"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/random"
	"go-build-admin/app/pkg/token"
	"image/png"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CommonHandler struct {
	log          *zap.Logger
	clickCaptcha *clickcaptcha.ClickCaptcha
	captcha      *captcha.Captcha
	tokenHelper  *token.TokenHelper
}

func NewCommonHandler(log *zap.Logger, clickCaptcha *clickcaptcha.ClickCaptcha, captcha *captcha.Captcha, tokenHelper *token.TokenHelper) *CommonHandler {
	return &CommonHandler{log: log, clickCaptcha: clickCaptcha, captcha: captcha, tokenHelper: tokenHelper}
}

// 图形验证码
func (h *CommonHandler) Captcha(ctx *gin.Context) {
	var params struct {
		Id string `form:"id" json:"id" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, err)
		return
	}

	img, err := h.captcha.Entry(params.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	// 将图像写入 HTTP 响应
	ctx.Writer.Header().Set("Content-Type", "image/png")
	err = png.Encode(ctx.Writer, img)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
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
	var params struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, err)
		return
	}
	if params.RefreshToken == "" {
		FailByErr(ctx, cErr.BadRequest("Login expired, please login again."))
		return
	}
	result, err := h.tokenHelper.Get(params.RefreshToken)
	if err != nil {
		FailByErr(ctx, cErr.BadRequest("Login expired, please login again."))
		return
	}

	newToken := random.Uuid()
	if result.Type == "admin-refresh" {
		batoken := ctx.GetHeader("batoken")
		if batoken == "" {
			FailByErr(ctx, cErr.BadRequest("Invalid token"))
			return
		}
		h.tokenHelper.Delete(batoken)
		h.tokenHelper.Set(newToken, "admin", result.UserID, 86400)

	} else if result.Type == "user-refresh" {
		baUserToken := ctx.GetHeader("ba-user-token")
		if baUserToken == "" {
			FailByErr(ctx, cErr.BadRequest("Invalid token"))
			return
		}
		h.tokenHelper.Delete(baUserToken)
		h.tokenHelper.Set(newToken, "user", result.UserID, 86400)
	}

	Success(ctx, map[string]any{
		"type":  result.Type,
		"token": newToken,
	})
}
