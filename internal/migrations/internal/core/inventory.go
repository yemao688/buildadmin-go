package core

import (
	"buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/captcha"
	"buildadmin-go/internal/pkg/token"
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
	NewModel    func() any
}

var coreTables = []CoreTable{
	{LogicalName: "admin_group_access", NewModel: func() any { return &model.AdminGroupAccess{} }},
	{LogicalName: "admin_group", NewModel: func() any { return &model.AdminGroup{} }},
	{LogicalName: "admin_log", NewModel: func() any { return &model.AdminLog{} }},
	{LogicalName: "admin_rule", NewModel: func() any { return &model.AdminRule{} }},
	{LogicalName: "admin", NewModel: func() any { return &model.Admin{} }},
	{LogicalName: "admin_closure", NewModel: func() any { return &model.AdminClosure{} }},
	{LogicalName: "area", NewModel: func() any { return &model.Area{} }},
	{LogicalName: "attachment", NewModel: func() any { return &upload.Attachment{} }},
	{LogicalName: "captcha", NewModel: func() any { return &captcha.Captcha{} }},
	{LogicalName: "config", NewModel: func() any { return &siteconfig.Config{} }},
	{LogicalName: "country_language", NewModel: func() any { return &model.Language{} }},
	{LogicalName: "country_language_content", NewModel: func() any { return &model.LanguageContent{} }},
	{LogicalName: "country_currency", NewModel: func() any { return &model.Currency{} }},
	{LogicalName: "crud_log", NewModel: func() any { return &model.Log{} }},
	{LogicalName: "security_data_recycle_log", NewModel: func() any { return &model.SecurityDataRecycleLog{} }},
	{LogicalName: "security_data_recycle", NewModel: func() any { return &model.SecurityDataRecycle{} }},
	{LogicalName: "security_sensitive_data_log", NewModel: func() any { return &model.SecuritySensitiveDataLog{} }},
	{LogicalName: "security_sensitive_data", NewModel: func() any { return &model.SecuritySensitiveData{} }},
	{LogicalName: "token", NewModel: func() any { return &token.Token{} }},
	{LogicalName: "user_money_log", NewModel: func() any { return &model.MoneyLog{} }},
	{LogicalName: "user", NewModel: func() any { return &model.User{} }},
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

func CoreLogicalNames() []string {
	names := make([]string, 0, len(coreTables))
	for _, table := range coreTables {
		names = append(names, table.LogicalName)
	}
	return names
}
