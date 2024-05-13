package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"github.com/go-mail/mail"
	"go.uber.org/zap"
)

type ConfigHandler struct {
	Base
	log     *zap.Logger
	configM *model.ConfigModel
}

func NewConfigHandler(log *zap.Logger, configM *model.ConfigModel) *ConfigHandler {
	return &ConfigHandler{
		Base: Base{currentM: configM},
		log:  log, configM: configM}
}

func (h *ConfigHandler) Index(ctx *gin.Context) {
	result, err := h.configM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"list":          result,
		"remark":        "",
		"configGroup":   "",
		"quickEntrance": "",
	})
}

func (h *ConfigHandler) Add(ctx *gin.Context) {
	result, err := h.configM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *ConfigHandler) Edit(ctx *gin.Context) {
	result, err := h.configM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *ConfigHandler) Del(ctx *gin.Context) {
	result, err := h.configM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *ConfigHandler) SendTestMail(ctx *gin.Context) {

	from := "sender@example.com"
	to := "sender@example.com"
	password := "your_password"
	smtpHost := "smtp.example.com"
	smtpPort := 587

	message := mail.NewMessage()
	message.SetHeader("From", from)
	message.SetHeader("To", to)
	message.SetHeader("Subject", "This is a test email-")
	message.SetBody("text/plain", "Congratulations, receiving this email means that your email service has been configured correctly")

	dialer := mail.NewDialer(smtpHost, smtpPort, from, password)

	dialer.DialAndSend(message)
	Success(ctx, "Test mail sent successfully~")
}
