package money

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewBalanceService,
)
