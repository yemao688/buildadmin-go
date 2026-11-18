package handler

import (
	"buildadmin-go/internal/common/country"
	"buildadmin-go/internal/pkg/crudhooks"
	"buildadmin-go/internal/pkg/requesttx"

	"github.com/gin-gonic/gin"
)

// ProvideCrudHooks 在装配期注册需要依赖的 CRUD 钩子：spec 里写钩子名，
// 这里把实现绑定到具体服务实例。本文件手写维护，生成器不会覆盖。
//
// 新增钩子时：在 spec 的 hooks.afterMutation 写名字，在下面补一条
// registerMutation，并在 internal/pkg/crudhooks 里保持名字唯一。
func ProvideCrudHooks(countrySvc *country.Service) crudhooks.Registrations {
	mustRegisterMutation("currency-cache", func(ctx *gin.Context) {
		// 货币列表缓存失效：响应暂存成功后登记，请求事务提交后执行；非事务请求立即执行。
		requesttx.InvalidateAfterMutation(ctx, countrySvc.InvalidateCurrencyCache)
	})
	mustRegisterMutation("language-cache", func(ctx *gin.Context) {
		// 语言列表缓存失效：同上。
		requesttx.InvalidateAfterMutation(ctx, countrySvc.InvalidateLanguageCache)
	})
	return crudhooks.Registrations{}
}

func mustRegisterMutation(name string, hook crudhooks.MutationHook) {
	if err := crudhooks.RegisterMutation(name, hook); err != nil {
		panic("register CRUD mutation hook: " + err.Error())
	}
}
