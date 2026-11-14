// Package service 承载后台渠道需要事务链与业务规则编排的场景；纯 CRUD 模块保持 handler 直连 repository。
package service

import (
	"github.com/google/wire"
)

// ProviderSet 聚合 admin 渠道业务服务构造器。仅当模块存在真实业务编排时
// 才在此登记（纯 CRUD 模块保持 handler→repository 直连，严禁透传 service）。
var ProviderSet = wire.NewSet(
	NewAuthService,
	NewAdminService,
	NewAdminGroupService,
	NewAdminRuleService,
	NewUserService,
	NewUserMoneyLogService,
	NewConfigService,
	NewSensitiveDataService,
	NewRoutineAttachmentService,
	NewSecuritySensitiveDataLogService,
	NewSecurityDataRecycleService,
	NewSecurityDataRecycleLogService,
	NewCrudService,
	NewAuditLogService,
)
