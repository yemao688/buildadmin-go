package country

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewCurrencyRepository,
	NewLanguageRepository,
	NewLanguageContentRepository,
)
