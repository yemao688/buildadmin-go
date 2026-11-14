// Package pkg 汇聚技术基建包的 Wire ProviderSet，供上层依赖图统一注入。
package pkg

import (
	"buildadmin-go/internal/pkg/captcha"
	"buildadmin-go/internal/pkg/clickcaptcha"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/terminal"
	"buildadmin-go/internal/pkg/token"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	terminal.NewTerminal,
	token.NewTokenHelper,
	clickcaptcha.NewClickCaptcha,
	captcha.NewCaptchaService,
	data_scope.ProviderSet,
)
