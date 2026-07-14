package data_scope

import "github.com/google/wire"

// ProviderSet exposes the temporary fail-closed enforcer binding to the
// application graph. Phase 2A can replace the concrete provider without
// changing consumers of the Enforcer interface.
var ProviderSet = wire.NewSet(
	NewDenyAllEnforcer,
	wire.Bind(new(Enforcer), new(*DenyAllEnforcer)),
)
