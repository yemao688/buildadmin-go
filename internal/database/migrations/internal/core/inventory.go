package core

import "go-build-admin/internal/database/migrations/model"

// CoreTable describes one framework table in fresh-snapshot order.
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
	{LogicalName: "attachment", NewModel: func() any { return &model.Attachment{} }},
	{LogicalName: "captcha", NewModel: func() any { return &model.Captcha{} }},
	{LogicalName: "config", NewModel: func() any { return &model.Config{} }},
	{LogicalName: "country_language", NewModel: func() any { return &model.CountryLanguage{} }},
	{LogicalName: "country_language_content", NewModel: func() any { return &model.CountryLanguageContent{} }},
	{LogicalName: "country_currency", NewModel: func() any { return &model.CountryCurrency{} }},
	{LogicalName: "crud_log", NewModel: func() any { return &model.CrudLog{} }},
	{LogicalName: "migrations", NewModel: func() any { return &model.Migrations{} }},
	{LogicalName: "security_data_recycle_log", NewModel: func() any { return &model.SecurityDataRecycleLog{} }},
	{LogicalName: "security_data_recycle", NewModel: func() any { return &model.SecurityDataRecycle{} }},
	{LogicalName: "security_sensitive_data_log", NewModel: func() any { return &model.SecuritySensitiveDataLog{} }},
	{LogicalName: "security_sensitive_data", NewModel: func() any { return &model.SecuritySensitiveData{} }},
	{LogicalName: "token", NewModel: func() any { return &model.Token{} }},
	{LogicalName: "user_money_log", NewModel: func() any { return &model.UserMoneyLog{} }},
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
