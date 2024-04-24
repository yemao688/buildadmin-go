package handler

import (
	"github.com/google/wire"
)

// ProviderSet is handler providers.
var ProviderSet = wire.NewSet(NewExampleHandler, NewMigrateHandler)
