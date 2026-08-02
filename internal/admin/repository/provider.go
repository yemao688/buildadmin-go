package repository

import (
	"github.com/google/wire"
)

// ProviderSet 聚合 admin 渠道全部仓储构造器。CRUD 生成器生成的新仓储
// 构造器在本集合中追加。
var ProviderSet = wire.NewSet(
	NewTableRepository,
	NewAdminRepository,
	NewAdminGroupRepository,
	NewAdminRuleRepository,
	NewAdminLogRepository,
	NewAuthRepository,
	NewCurrencyRepository,
	NewLanguageRepository,
	NewLanguageContentRepository,
	NewConfigRepository,
	NewAttachmentRepository,
	NewDataRecycleRepository,
	NewDataRecycleLogRepository,
	NewSensitiveDataRepository,
	NewSensitiveDataLogRepository,
	NewUserRepository,
	NewMoneyLogRepository,
)
