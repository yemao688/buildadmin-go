package handler

import (
	"buildadmin-go/internal/api/service"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/captcha"
	"buildadmin-go/internal/pkg/clickcaptcha"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/token"
	"buildadmin-go/internal/pkg/util"
	"image/png"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CommonHandler struct {
	log          *zap.Logger
	clickCaptcha *clickcaptcha.ClickCaptcha
	captcha      *captcha.CaptchaService
	tokenHelper  *token.TokenHelper
	authM        *service.MemberService
	config       *conf.Configuration
	uploadHelper *upload.UploadHelper
}

func NewCommonHandler(log *zap.Logger, clickCaptcha *clickcaptcha.ClickCaptcha, captcha *captcha.CaptchaService, tokenHelper *token.TokenHelper, authM *service.MemberService, config *conf.Configuration, uploadHelper *upload.UploadHelper) *CommonHandler {
	registerBuiltinRefreshTypes()
	return &CommonHandler{log: log, clickCaptcha: clickCaptcha, captcha: captcha, tokenHelper: tokenHelper, authM: authM, config: config, uploadHelper: uploadHelper}
}

// Upload 会员文件上传（本地存储），与 admin ajax/upload 对齐，属主为当前会员。
func (h *CommonHandler) Upload(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	userAuth := header.GetUserAuth(ctx)

	result, err := h.uploadHelper.Upload(ctx, upload.UploadParams{File: file}, 0, userAuth.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"file": result,
	})
}

// AliossCallback 阿里云 OSS 直传回调：浏览器把文件 POST 到 OSS 后调用本接口
// 登记附件记录（storage=alioss，sha1 幂等去重）。
func (h *CommonHandler) AliossCallback(ctx *gin.Context) {
	var params upload.OSSCallback
	if err := ctx.ShouldBind(&params); err != nil {
		FailByErr(ctx, err)
		return
	}
	userAuth := header.GetUserAuth(ctx)
	result, err := h.uploadHelper.CompleteOSS(params, 0, userAuth.Id)
	if err != nil {
		h.log.Error("AliOSS callback failed", zap.Error(err))
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{"file": result})
}

func FailByErrWithData(c *gin.Context, err error, data interface{}) {
	v, ok := err.(*cErr.Error)
	if !ok {
		FailByErr(c, err)
		return
	}

	msg := util.Lang(c, v.Error(), nil)
	if requesttx.Stage(c, requesttx.Outcome{
		HTTPCode:     v.HttpCode(),
		BusinessCode: v.ErrorCode(),
		Message:      msg,
		Data:         data,
	}) {
		return
	}
	c.JSON(v.HttpCode(), Response{v.ErrorCode(), data, msg, 0})
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
		FailByErr(ctx, cErr.BadRequest("Captcha error"))
		return
	}
	Success(ctx, "")
}

func (h *CommonHandler) RefreshToken(ctx *gin.Context) {
	registerBuiltinRefreshTypes()
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

	desc, ok := lookupRefreshType(result.Type)
	if !ok {
		FailByErr(ctx, cErr.BadRequest("Invalid token"))
		return
	}

	if ctx.GetHeader(desc.AccessHeader) == "" {
		FailByErr(ctx, cErr.BadRequest("Invalid token"))
		return
	}

	ctx.Set(refreshHandlerContextKey, h)
	newToken, err := desc.Refresh(ctx, params.RefreshToken, result.UserID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	Success(ctx, map[string]any{
		"type":  result.Type,
		"token": newToken,
	})
}
