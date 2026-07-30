package routine

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewConfigHandler,
	NewConfigRegistrar,
	NewAttachmentHandler,
	NewAttachmentRegistrar,
	NewAdminInfoHandler,
	NewAdminInfoRegistrar,
)
