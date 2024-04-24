package cron

import "github.com/google/wire"

// ProviderSet is cron providers.
var ProviderSet = wire.NewSet(NewCron, NewExampleJob)
