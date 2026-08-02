package security

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewDataRecycleModel,
	NewDataRecycleLogModel,
	NewSensitiveDataModel,
	NewSensitiveDataLogModel,
)
