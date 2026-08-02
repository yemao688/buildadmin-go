package handler

import (
	"github.com/google/wire"
)

// ProviderSet 聚合 admin 渠道全部 handler 构造器。路由注册器（registrar）
// 已随路由中心化迁至 internal/admin/router 包。CRUD 生成器生成的
// handler 构造器在本集合中追加。
var ProviderSet = wire.NewSet(
	NewAjaxHandler,
	NewDashboardHandler,
	NewIndexHandler,
	NewModuleHandler,
	NewAdminHandler,
	NewAdminGroupHandler,
	NewAdminRuleHandler,
	NewAdminLogHandler,
	NewCurrencyHandler,
	NewLanguageHandler,
	NewLanguageContentHandler,
	NewCrudHandler,
	NewLogHandler,
	NewAdminInfoHandler,
	NewAttachmentHandler,
	NewConfigHandler,
	NewDataRecycleHandler,
	NewDataRecycleLogHandler,
	NewSensitiveDataHandler,
	NewSensitiveDataLogHandler,
	NewUserHandler,
	NewMoneyLogHandler,
)
