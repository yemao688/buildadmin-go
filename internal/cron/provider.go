package cron

import "github.com/google/wire"

// ProviderSet is cron providers. NewCron 不再需要任务依赖：业务定时任务
// 通过 cron.Register 自注册，由 Run() 统一挂载。
var ProviderSet = wire.NewSet(NewCron)
