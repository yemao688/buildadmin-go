package pkg

import (
	"go-build-admin/app/pkg/captcha"
	"go-build-admin/app/pkg/clickcaptcha"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/app/pkg/terminal"
	"go-build-admin/app/pkg/token"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	terminal.NewTerminal,
	token.NewTokenHelper,
	clickcaptcha.NewClickCaptcha,
	captcha.NewCaptcha,
	data_scope.ProviderSet,
)
