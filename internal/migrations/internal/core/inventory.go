package core

import (
	"fmt"
	"strings"

	"buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/captcha"
	"buildadmin-go/internal/pkg/token"

	"gorm.io/gorm"
)

// CoreTable describes one framework table in fresh-snapshot order.
// Models are collected from their owning packages: the shared entity layer
// (internal/model) plus the per-table owners (upload.Attachment,
// siteconfig.Config, token.Token, captcha.Captcha, model.Log). The three
// migrations ledgers are created by the ledger bootstrap (BootstrapOfficial/
// Framework/BusinessLedger) and are intentionally not part of the
// AutoMigrate snapshot.
type CoreTable struct {
	LogicalName string
	Comment     string // table comment aligned with PHP upstream install migration
	NewModel    func() any
}

var coreTables = []CoreTable{
	{LogicalName: "admin_group_access", Comment: "管理分组映射表", NewModel: func() any { return &model.AdminGroupAccess{} }},
	{LogicalName: "admin_group", Comment: "管理分组表", NewModel: func() any { return &model.AdminGroup{} }},
	{LogicalName: "admin_log", Comment: "管理员日志表", NewModel: func() any { return &model.AdminLog{} }},
	{LogicalName: "admin_rule", Comment: "菜单和权限规则表", NewModel: func() any { return &model.AdminRule{} }},
	{LogicalName: "admin", Comment: "管理员表", NewModel: func() any { return &model.Admin{} }},
	{LogicalName: "admin_closure", Comment: "管理员层级闭包表", NewModel: func() any { return &model.AdminClosure{} }},
	{LogicalName: "area", Comment: "省份地区表", NewModel: func() any { return &model.Area{} }},
	{LogicalName: "attachment", Comment: "附件表", NewModel: func() any { return &upload.Attachment{} }},
	{LogicalName: "captcha", Comment: "验证码表", NewModel: func() any { return &captcha.Captcha{} }},
	{LogicalName: "config", Comment: "系统配置", NewModel: func() any { return &siteconfig.Config{} }},
	{LogicalName: "country_language", Comment: "国家语言表", NewModel: func() any { return &model.CountryLanguage{} }},
	{LogicalName: "country_language_content", Comment: "国家语言内容表", NewModel: func() any { return &model.CountryLanguageContent{} }},
	{LogicalName: "country_currency", Comment: "国家货币表", NewModel: func() any { return &model.CountryCurrency{} }},
	{LogicalName: "crud_log", Comment: "CRUD记录表", NewModel: func() any { return &model.Log{} }},
	{LogicalName: "security_data_recycle_log", Comment: "数据回收记录表", NewModel: func() any { return &model.SecurityDataRecycleLog{} }},
	{LogicalName: "security_data_recycle", Comment: "回收规则表", NewModel: func() any { return &model.SecurityDataRecycle{} }},
	{LogicalName: "security_sensitive_data_log", Comment: "敏感数据修改记录", NewModel: func() any { return &model.SecuritySensitiveDataLog{} }},
	{LogicalName: "security_sensitive_data", Comment: "敏感数据规则表", NewModel: func() any { return &model.SecuritySensitiveData{} }},
	{LogicalName: "token", Comment: "用户Token表", NewModel: func() any { return &token.Token{} }},
	{LogicalName: "user_money_log", Comment: "会员余额变动表", NewModel: func() any { return &model.MoneyLog{} }},
	{LogicalName: "user", Comment: "会员表", NewModel: func() any { return &model.User{} }},
}

func CoreTables() []CoreTable {
	return append([]CoreTable(nil), coreTables...)
}

func CoreModels() []any {
	models := make([]any, 0, len(coreTables))
	for _, table := range coreTables {
		models = append(models, table.NewModel())
	}
	return models
}

// ApplyTableComments writes the table comments after the fresh snapshot,
// since GORM AutoMigrate cannot express table-level comments from struct
// tags. The values mirror the PHP upstream install migration.
func ApplyTableComments(db *gorm.DB, prefix string) error {
	for _, table := range coreTables {
		if table.Comment == "" {
			continue
		}
		name := prefix + table.LogicalName
		comment := strings.ReplaceAll(table.Comment, "'", "''")
		if err := db.Exec("ALTER TABLE `" + name + "` COMMENT = '" + comment + "'").Error; err != nil {
			return fmt.Errorf("set table comment %s: %w", name, err)
		}
	}
	return nil
}

func CoreLogicalNames() []string {
	names := make([]string, 0, len(coreTables))
	for _, table := range coreTables {
		names = append(names, table.LogicalName)
	}
	return names
}
