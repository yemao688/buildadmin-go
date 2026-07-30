package crud

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewCrudLogModel,
)
