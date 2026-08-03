package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"strconv"
	"strings"

	"buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	siteconfig "buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/utils"

	"github.com/go-mail/mail"
	"github.com/jinzhu/copier"
)

// ConfigService 承载站点配置的业务规则：新增时的字典/扩展属性装配与
// 测试邮件发送。编辑/删除保持绑定→repository→响应。
type ConfigService struct {
	configM *adminmodel.ConfigRepository
}

func NewConfigService(configM *adminmodel.ConfigRepository) *ConfigService {
	return &ConfigService{configM: configM}
}

// ConfigParams carries the plain (non-gin-bound) shape of a config create
// request.
type ConfigParams struct {
	Name        string
	Group       string
	Title       string
	Tip         string
	Type        string
	Content     string
	Rule        []string
	Extend      string
	InputExtend string
	Weigh       int32
}

// Add 编排配置新增：按组件类型装配 content/extend 字段后落库。
func (s *ConfigService) Add(ctx context.Context, p ConfigParams) error {
	var config = siteconfig.Config{}
	if err := copier.Copy(&config, p); err != nil {
		return err
	}
	if p.Type == "radio" || p.Type == "checkbox" || p.Type == "select" || p.Type == "selects" {
		contentBytes, _ := json.Marshal(utils.StrAttrToArray(p.Content))
		config.Content = string(contentBytes)
	} else {
		config.Content = ""
	}
	config.Rule = strings.Join(p.Rule, ",")

	if p.Extend != "" || p.InputExtend != "" {
		inputExtend := utils.StrAttrToArray(p.InputExtend)
		extend := utils.StrAttrToArray(p.Extend)
		if len(inputExtend) > 0 {
			extend["baInputExtend"] = inputExtend
		}
		if len(extend) > 0 {
			extendBytes, _ := json.Marshal(extend)
			config.Extend = string(extendBytes)
		}
		config.AllowDel = 1
	}

	return s.configM.Add(ctx, config)
}

// SendTestMail 发送测试邮件，按 SMTP 配置构建 dialer 并尝试真实投递。
func (s *ConfigService) SendTestMail(params dto.MailParam) error {
	message := mail.NewMessage()
	message.SetHeader("From", params.SmtpSenderMail)
	message.SetHeader("To", params.TestMail)
	message.SetHeader("Subject", "This is a test email")
	message.SetBody("text/plain", "congratulations, receiving this email means that your email service has been configured correctly")

	// 根据提供的加密类型设置 Dialer 的 TLSConfig
	port, err := strconv.Atoi(params.SmtpPort)
	if err != nil {
		return err
	}
	dialer := mail.NewDialer(params.SmtpServer, port, params.SmtpUser, params.SmtpPass)
	if strings.EqualFold(params.SmtpVerification, "SSL") {
		dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true, ServerName: params.SmtpServer}
	}
	return dialer.DialAndSend(message)
}
