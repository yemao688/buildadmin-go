package security

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewDataRecycleHandler,
	NewDataRecycleRegistrar,
	NewDataRecycleLogHandler,
	NewDataRecycleLogRegistrar,
	NewSensitiveDataHandler,
	NewSensitiveDataRegistrar,
	NewSensitiveDataLogHandler,
	NewSensitiveDataLogRegistrar,
)
