package utils

import (
	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// 翻译内容
func Lang(c *gin.Context, messageID string, templateData map[string]interface{}) string {
	msg := ginI18n.MustGetMessage(c, &i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})

	if msg == "" {
		return messageID
	}
	return msg
}
