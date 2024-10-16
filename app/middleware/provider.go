package middleware

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewLogin,
	NewDataLimit,
	NewRecord,
	NewUserLogin,
	NewSecurity,
)
