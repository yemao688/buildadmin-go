package security

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewDataRecycleRepository,
	NewDataRecycleLogRepository,
	NewSensitiveDataRepository,
	NewSensitiveDataLogRepository,
)
