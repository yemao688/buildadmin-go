package country

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewCurrencyModel,
	NewLanguageModel,
	NewLanguageContentModel,
)
