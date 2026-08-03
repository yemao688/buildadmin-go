package install

import (
	"github.com/google/wire"
)

// ProviderSet 聚合安装渠道的 handler 与路由注册器构造器。
var ProviderSet = wire.NewSet(
	NewInstallHandler,
	NewInstallRouter,
)
