package country

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewCurrencyHandler,
	NewCurrencyRegistrar,
	NewLanguageHandler,
	NewLanguageRegistrar,
	NewLanguageContentHandler,
	NewLanguageContentRegistrar,
)
