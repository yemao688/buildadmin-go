// Package middleware 提供门户渠道的会员登录态校验中间件。
package middleware

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewUserLogin,
)
