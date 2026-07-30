package area

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAreaModel,
)
