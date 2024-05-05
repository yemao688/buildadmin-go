package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type EmsHandler struct {
	log *zap.Logger
}

func NewEmsHandler(log *zap.Logger) *EmsHandler {
	return &EmsHandler{log: log}
}

/**
 * 发送邮件
 * event 事件:user_register=用户注册,user_change_email=用户修改邮箱,user_retrieve_pwd=用户找回密码,user_email_verify=验证账户
 * 不同的事件，会自动做各种必要检查，其中 验证账户 要求用户输入当前密码才能发送验证码邮件
 */
func (h *EmsHandler) Send(ctx *gin.Context) {

	Success(ctx, "")
}
