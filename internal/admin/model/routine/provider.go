package routine

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewConfigModel,
	NewAttachmentModel,
)
