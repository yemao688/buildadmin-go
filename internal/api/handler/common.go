package handler

import (
	"buildadmin-go/internal/api/service"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/captcha"
	"buildadmin-go/internal/pkg/clickcaptcha"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/token"
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
	token.RegisterBuiltinRefreshTypes()
	return &CommonHandler{log: log, clickCaptcha: clickCaptcha, captcha: captcha, tokenHelper: tokenHelper, authM: authM, config: config, uploadHelper: uploadHelper}
}

// SetAdminToken 实现 token.RefreshExecutor：为管理员落库 access token。
func (h *CommonHandler) SetAdminToken(newToken string, userID int32) error {
	return h.tokenHelper.Set(newToken, "admin", userID, h.config.App.AdminTokenKeepTime)
}

// RefreshUserAccessToken 实现 token.RefreshExecutor：用 refresh token 换发
// 会员 access token；会员服务不可用时返回内部错误。
func (h *CommonHandler) RefreshUserAccessToken(refreshToken string) (string, error) {
	if h.authM == nil {
		return "", cErr.InternalServer("token service unavailable")
	}
	return h.authM.RefreshUserAccessToken(refreshToken)
}

// 图形验证码
func (h *CommonHandler) Captcha(ctx *gin.Context) {
	var params struct {
		Id string `form:"id" json:"id" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&params); err != nil {
		response.FailByErr(ctx, err)
		return
	}

	img, err := h.captcha.Entry(params.Id)
	if err != nil {
		response.FailByErr(ctx, err)
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
		response.FailByErr(ctx, err)
		return
	}
	result, err := h.clickCaptcha.Create(ctx, params.Id)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, result)
}

// 点选验证码检查
func (h *CommonHandler) CheckClickCaptcha(ctx *gin.Context) {
	var params struct {
		Id    string `json:"id" binding:"required"`
		Info  string `json:"info" binding:"required"`
		Unset bool   `json:"unset"`
	}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	if !h.clickCaptcha.Check(params.Id, params.Info, params.Unset) {
		response.FailByErr(ctx, cErr.BadRequest("Captcha error"))
		return
	}
	response.Success(ctx, "")
}

func (h *CommonHandler) RefreshToken(ctx *gin.Context) {
	token.RegisterBuiltinRefreshTypes()
	var params struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	if params.RefreshToken == "" {
		response.FailByErr(ctx, cErr.BadRequest("Login expired, please login again."))
		return
	}
	result, err := h.tokenHelper.Get(params.RefreshToken)
	if err != nil {
		response.FailByErr(ctx, cErr.BadRequest("Login expired, please login again."))
		return
	}

	desc, ok := token.LookupRefreshType(result.Type)
	if !ok {
		response.FailByErr(ctx, cErr.BadRequest("Invalid token"))
		return
	}

	if ctx.GetHeader(desc.AccessHeader) == "" {
		response.FailByErr(ctx, cErr.BadRequest("Invalid token"))
		return
	}

	ctx.Set(token.HandlerContextKey, h)
	newToken, err := desc.Refresh(ctx, params.RefreshToken, result.UserID)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	response.Success(ctx, map[string]any{
		"type":  result.Type,
		"token": newToken,
	})
}
