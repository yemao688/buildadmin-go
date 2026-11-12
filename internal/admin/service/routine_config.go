package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	siteconfig "buildadmin-go/internal/common/siteconfig"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/util"

	"github.com/go-mail/mail"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

// ConfigService 承载站点配置的业务规则：变量名唯一性预检、字典/扩展属性
// 装配（新增与编辑共用）与测试邮件发送。删除保持绑定→repository→响应。
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

// Add 编排配置新增：按组件类型装配 content/extend 字段、校验变量名唯一后落库。
func (s *ConfigService) Add(ctx context.Context, p ConfigParams) error {
	config, err := prepareConfig(p)
	if err != nil {
		return err
	}
	exists, err := s.configM.NameExists(ctx, config.Name, 0)
	if err != nil {
		return err
	}
	if exists {
		return cErr.BadRequest("config name already exists")
	}
	return s.configM.Add(ctx, config)
}

// Edit 编排配置编辑：与 Add 共用同一套 content/extend 装配，并在落库前做
// 变量名唯一性预检（排除自身）。并发插入仍由 repo 的 duplicate-key 兜底。
func (s *ConfigService) Edit(ctx context.Context, id int32, p ConfigParams) error {
	config, err := prepareConfig(p)
	if err != nil {
		return err
	}
	config.ID = id
	exists, err := s.configM.NameExists(ctx, config.Name, id)
	if err != nil {
		return err
	}
	if exists {
		return cErr.BadRequest("config name already exists")
	}
	return s.configM.Edit(ctx, config)
}

// prepareConfig 把请求参数装配为待落库的配置行：字典型组件把 content 序列化
// 为 JSON，扩展属性序列化进 extend 并标记允许删除。Add/Edit 共用，避免两条
// 写入路径的字段形态漂移。
func prepareConfig(p ConfigParams) (siteconfig.Config, error) {
	var config = siteconfig.Config{}
	if err := copier.Copy(&config, p); err != nil {
		return config, err
	}
	if p.Type == "radio" || p.Type == "checkbox" || p.Type == "select" || p.Type == "selects" {
		contentBytes, _ := json.Marshal(util.StrAttrToArray(p.Content))
		config.Content = string(contentBytes)
	} else {
		config.Content = ""
	}
	config.Rule = strings.Join(p.Rule, ",")

	if p.Extend != "" || p.InputExtend != "" {
		inputExtend := util.StrAttrToArray(p.InputExtend)
		extend := util.StrAttrToArray(p.Extend)
		if len(inputExtend) > 0 {
			extend["baInputExtend"] = inputExtend
		}
		if len(extend) > 0 {
			extendBytes, _ := json.Marshal(extend)
			config.Extend = string(extendBytes)
		}
		config.AllowDel = 1
	}
	return config, nil
}

// SaveAll 编排站点配置整体保存：事务内逐行装配类型化值，跳过空白的
// upload_secret_key（表单回填不能清空已存密钥），并拒绝静默空更新。
func (s *ConfigService) SaveAll(ctx context.Context, params map[string]any) error {
	return s.configM.Transaction(ctx, func(tx *gorm.DB) error {
		all, err := s.configM.AllTx(tx)
		if err != nil {
			return err
		}
		for _, v := range all {
			value, ok := params[v.Name]
			if !ok {
				continue
			}
			if v.Name == "upload_secret_key" && fmt.Sprintf("%v", value) == "" {
				continue
			}
			newValue := v.SetValueAttr(value, v.Type)
			if err := updateConfigValue(v.Value, newValue, func() (int64, error) {
				return s.configM.UpdateValueTx(tx, v.ID, newValue)
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// updateConfigValue applies one config value change, rejecting silent no-ops
// so a lost row surfaces instead of passing as a successful save.
func updateConfigValue(currentValue, newValue string, update func() (int64, error)) error {
	if newValue == currentValue {
		return nil
	}

	rowsAffected, err := update()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return fmt.Errorf("config update failed: rows affected mismatch")
	}
	return nil
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
