package handler

import (
	"crypto/tls"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	commonModel "go-build-admin/app/common/model"
	"go-build-admin/app/pkg/captcha"
	"go-build-admin/app/pkg/clickcaptcha"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-mail/mail"
	"go.uber.org/zap"
)

type EmsHandler struct {
	log          *zap.Logger
	configM      *model.ConfigModel
	captcha      *captcha.Captcha
	clickCaptcha *clickcaptcha.ClickCaptcha
	userM        *commonModel.UserModel
	authM        *commonModel.AuthModel
}

func NewEmsHandler(log *zap.Logger, configM *model.ConfigModel, captcha *captcha.Captcha, clickCaptcha *clickcaptcha.ClickCaptcha, userM *commonModel.UserModel, authM *commonModel.AuthModel) *EmsHandler {
	return &EmsHandler{
		log:          log,
		configM:      configM,
		captcha:      captcha,
		clickCaptcha: clickCaptcha,
		userM:        userM,
		authM:        authM,
	}
}

type SendEmail struct {
	Email       string `json:"email" binding:"required,email"`
	Event       string `json:"event" binding:"required"`
	CaptchaId   string `json:"captchaId" binding:"required"`
	CaptchaInfo string `json:"captchaInfo" binding:"required"`
	Password    string `json:"password"`
}

func (v SendEmail) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"email.required": "email required",
		"event.required": "event required",
	}
}

/**
 * 发送邮件
 * event 事件:user_register=用户注册,user_change_email=用户修改邮箱,user_retrieve_pwd=用户找回密码,user_email_verify=验证账户
 * 不同的事件，会自动做各种必要检查，其中 验证账户 要求用户输入当前密码才能发送验证码邮件
 */
func (h *EmsHandler) Send(ctx *gin.Context) {
	params := SendEmail{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	mailConfig, err := h.configM.GetKVByGroup(ctx, "mail")
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	// 检查验证码
	if !h.clickCaptcha.Check(params.CaptchaId, params.CaptchaInfo, true) {
		FailByErr(ctx, cErr.BadRequest("Captcha error"))
		return
	}

	// 检查频繁发送
	captchaData, err := h.captcha.GetCaptchaData(params.Email + params.Event)
	if err == nil && time.Now().Unix()-captchaData.CreateTime < 60 {
		FailByErr(ctx, cErr.BadRequest("Frequent email sending"))
		return
	}

	// 检查邮箱
	_, err = h.userM.GetOneByEmail(ctx, params.Email)
	if err == nil && params.Event == "user_register" {
		FailByErr(ctx, cErr.BadRequest("Email has been registered, please log in directly"))
		return
	}

	if err == nil && params.Event == "user_change_email" {
		FailByErr(ctx, cErr.BadRequest("The email has been occupied"))
		return
	}

	if err != nil && slices.Contains([]string{"user_retrieve_pwd", "user_email_verify"}, params.Event) {
		FailByErr(ctx, cErr.BadRequest("Email not registered"))
		return
	}

	// 通过邮箱验证账户
	if params.Event == "user_email_verify" {
		token, isLogin := h.authM.IsLogin(ctx)
		if !isLogin {
			FailByErr(ctx, cErr.BadRequest("Please login first"))
			return
		}

		user, _ := h.userM.GetOne(ctx, token.UserID)
		if params.Email != user.Email {
			FailByErr(ctx, cErr.BadRequest("Please use the account registration email to send the verification code"))
			return
		}

		// 验证账户密码
		if user.Password != utils.EncryptPassword(params.Password, user.Salt) {
			FailByErr(ctx, cErr.BadRequest("Password error"))
			return
		}
	}

	// 生成一个验证码
	code, err := h.captcha.Create(params.Email + params.Event)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	site_name, _ := h.configM.GetValueByName(ctx, "mail")
	subject := utils.Lang(ctx, params.Event+"-"+site_name, map[string]interface{}{
		"code": code,
	})
	body := utils.Lang(ctx, "Your verification code is: {code}", map[string]interface{}{
		"code": code,
	})

	message := mail.NewMessage()
	message.SetHeader("From", mailConfig["smtp_sender_mail"])
	message.SetHeader("To", params.Email)
	message.SetHeader("Subject", subject)
	message.SetBody("text/plain", body)

	// 根据提供的加密类型设置 Dialer 的 TLSConfig
	port, _ := strconv.Atoi(mailConfig["smtp_port"])
	dialer := mail.NewDialer(mailConfig["smtp_server"], port, mailConfig["smtp_user"], mailConfig["smtp_pass"])
	if strings.EqualFold(mailConfig["smtp_verification"], "SSL") {
		dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true, ServerName: mailConfig["smtp_server"]}
	}
	if err := dialer.DialAndSend(message); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
