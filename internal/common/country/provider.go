// Package country 提供国家/货币/语言等公共领域服务。
package country

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewService,
)
