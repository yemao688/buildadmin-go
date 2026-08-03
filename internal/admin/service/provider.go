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
	NewUserService,
	NewConfigService,
	NewSensitiveDataService,
)
