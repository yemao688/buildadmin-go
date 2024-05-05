package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AccountHandler struct {
	log *zap.Logger
}

func NewAccountHandler(log *zap.Logger) *AccountHandler {
	return &AccountHandler{log: log}
}

func (h *AccountHandler) Overview(ctx *gin.Context) {

	Success(ctx, "")
}

/**
 * 会员资料
 */
func (h *AccountHandler) Profile(ctx *gin.Context) {

	Success(ctx, "")
}

/**
 * 通过手机号或邮箱验证账户
 * 此处检查的验证码是通过 api/Ems或api/Sms发送的
 * 验证成功后，向前端返回一个 email-pass Token或着 mobile-pass Token
 * 在 changBind 方法中，通过 pass Token来确定用户已经通过了账户验证（用户未绑定邮箱/手机时通过账户密码验证）
 */
func (h *AccountHandler) Verification(ctx *gin.Context) {

	Success(ctx, "")
}

/**
 * 修改绑定信息（手机号、邮箱）
 * 通过 pass Token来确定用户已经通过了账户验证，也就是以上的 verification 方法，同时用户未绑定邮箱/手机时通过账户密码验证
 */
func (h *AccountHandler) ChangeBind(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *AccountHandler) ChangePassword(ctx *gin.Context) {

	Success(ctx, "")
}

// 积分日志
func (h *AccountHandler) Integral(ctx *gin.Context) {

	Success(ctx, "")
}

// 余额日志
func (h *AccountHandler) Balance(ctx *gin.Context) {

	Success(ctx, "")
}

// 找回密码
func (h *AccountHandler) RetrievePassword(ctx *gin.Context) {

	Success(ctx, "")
}
