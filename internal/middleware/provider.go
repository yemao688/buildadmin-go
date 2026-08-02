package middleware

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewLogin,
	NewAuthorization,
	NewRecord,
	NewUserLogin,
	NewSecurity,
)
