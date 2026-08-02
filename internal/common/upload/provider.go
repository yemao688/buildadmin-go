package upload

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAliossStorage,
	NewUploadHelper,
)
