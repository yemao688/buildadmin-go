package repository

import "github.com/google/wire"

// ProviderSet 提供 api 渠道的仓库构造器，由 service.ProviderSet 纳入
// Wire 依赖图。
var ProviderSet = wire.NewSet(
	NewUserRepository,
)
