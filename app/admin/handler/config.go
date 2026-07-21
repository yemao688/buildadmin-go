package handler

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-mail/mail"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ConfigHandler struct {
	Base
	log     *zap.Logger
	config  *conf.Configuration
	configM *model.ConfigModel
}

type configJSONItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

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

func decodeConfigJSON(field, value string) ([]configJSONItem, error) {
	if strings.TrimSpace(value) == "" {
		return []configJSONItem{}, nil
	}
	items := []configJSONItem{}
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return nil, fmt.Errorf("%s: %w", field, err)
	}
	if items == nil {
		items = []configJSONItem{}
	}
	return items, nil
}

func (h *ConfigHandler) configJSON(ctx *gin.Context, name string) ([]configJSONItem, error) {
	value, err := h.configM.GetValueByName(ctx, name)
	return decodeConfigValue(name, value, err)
}

func decodeConfigValue(field, value string, err error) ([]configJSONItem, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []configJSONItem{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", field, err)
	}
	return decodeConfigJSON(field, value)
}

func NewConfigHandler(log *zap.Logger, config *conf.Configuration, configM *model.ConfigModel) *ConfigHandler {
	return &ConfigHandler{
		Base: Base{currentM: configM},
		log:  log, config: config, configM: configM}
}

func (h *ConfigHandler) Index(ctx *gin.Context) {
	configGroupItems, err := h.configJSON(ctx, "config_group")
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	type Group struct {
		Name  string        `json:"name"`
		Title string        `json:"title"`
		List  []interface{} `json:"list"`
	}

	list := map[string]*Group{}
	newConfigGroup := map[string]any{}
	for _, v := range configGroupItems {
		title := utils.Lang(ctx, v.Value, nil)
		newConfigGroup[v.Key] = title
		list[v.Key] = &Group{
			Name:  v.Key,
			Title: title,
			List:  []any{},
		}
	}

	all, err := h.configM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	for _, v := range all {
		if _, ok := list[v.Group]; ok {
			title := utils.Lang(ctx, v.Title, nil)
			value := v.GetValueAttr()
			if v.Name == "upload_secret_key" {
				value = ""
			}
			list[v.Group].List = append(list[v.Group].List, map[string]any{
				"id":           v.ID,
				"name":         v.Name,
				"group":        v.Group,
				"title":        title,
				"tip":          v.Tip,
				"type":         v.Type,
				"value":        value,
				"content":      v.GetContentAttr(),
				"rule":         v.Rule,
				"extend":       v.GetExtendAttr(),
				"allow_del":    v.AllowDel,
				"weigh":        v.Weigh,
				"input_extend": v.GetInputExtendAttr(),
			})
		}
	}

	quickEntranceItems, err := h.configJSON(ctx, "config_quick_entrance")
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"list":          list,
		"remark":        "",
		"configGroup":   newConfigGroup,
		"quickEntrance": quickEntranceItems,
	})
}

type Config struct {
	Name        string   `json:"name"`
	Group       string   `json:"group"`
	Title       string   `json:"title"`
	Tip         string   `json:"tip"`
	Type        string   `json:"type"`
	Content     string   `json:"content"`
	Rule        []string `json:"rule"`
	Extend      string   `json:"extend"`
	InputExtend string   `json:"input_extend"`
	Weigh       int32    `json:"weigh"`
}

func (v Config) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *ConfigHandler) Add(ctx *gin.Context) {
	var params Config
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var config = model.Config{}
	copier.Copy(&config, params)
	if params.Type == "radio" || params.Type == "checkbox" || params.Type == "select" || params.Type == "selects" {
		contentBytes, _ := json.Marshal(utils.StrAttrToArray(params.Content))
		config.Content = string(contentBytes)
	} else {
		config.Content = ""
	}
	config.Rule = strings.Join(params.Rule, ",")

	if params.Extend != "" || params.InputExtend != "" {
		inputExtend := utils.StrAttrToArray(params.InputExtend)
		extend := utils.StrAttrToArray(params.Extend)
		if len(inputExtend) > 0 {
			extend["baInputExtend"] = inputExtend
		}
		if len(extend) > 0 {
			extendBytes, _ := json.Marshal(extend)
			config.Extend = string(extendBytes)
		}
		config.AllowDel = 1
	}

	err := h.configM.Add(ctx, config)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *ConfigHandler) Edit(ctx *gin.Context) {
	var params map[string]interface{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, err)
		return
	}

	if err := h.configM.Transaction(ctx, func(tx *gorm.DB) error {
		all := []model.Config{}
		if err := tx.Model(&model.Config{}).Order("`weigh` desc").Find(&all).Error; err != nil {
			return err
		}
		for _, v := range all {
			if value, ok := params[v.Name]; ok {
				if v.Name == "upload_secret_key" && fmt.Sprintf("%v", value) == "" {
					continue
				}
				newValue := v.SetValueAttr(value, v.Type)
				if err := updateConfigValue(v.Value, newValue, func() (int64, error) {
					result := tx.Table(h.configM.TableName).Where("id=?", v.ID).Update("value", newValue)
					return result.RowsAffected, result.Error
				}); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *ConfigHandler) Del(ctx *gin.Context) {
	var param validate.Ids
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.configM.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

type MailParam struct {
	SmtpServer       string `json:"smtp_server" binding:"required"`
	SmtpPort         string `json:"smtp_port" binding:"required"`
	SmtpUser         string `json:"smtp_user" binding:"required"`
	SmtpPass         string `json:"smtp_pass" binding:"required"`
	SmtpVerification string `json:"smtp_verification" binding:"required"`
	SmtpSenderMail   string `json:"smtp_sender_mail" binding:"required"`
	TestMail         string `json:"testMail" binding:"required"`
}

func (v MailParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *ConfigHandler) SendTestMail(ctx *gin.Context) {
	params := MailParam{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	message := mail.NewMessage()
	message.SetHeader("From", params.SmtpSenderMail)
	message.SetHeader("To", params.TestMail)
	message.SetHeader("Subject", "This is a test email-"+h.config.App.AppName)
	message.SetBody("text/plain", "congratulations, receiving this email means that your email service has been configured correctly")

	// 根据提供的加密类型设置 Dialer 的 TLSConfig
	port, err := strconv.Atoi(params.SmtpPort)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	dialer := mail.NewDialer(params.SmtpServer, port, params.SmtpUser, params.SmtpPass)
	if strings.EqualFold(params.SmtpVerification, "SSL") {
		dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true, ServerName: params.SmtpServer}
	}

	if err := dialer.DialAndSend(message); err != nil {
		JsonReturn(ctx, http.StatusOK, 0, "Mail sending service unavailable", err.Error())
		return
	}
	Success(ctx, "test mail sent successfully~")
}
