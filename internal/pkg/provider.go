package pkg

import (
	"go-build-admin/internal/pkg/captcha"
	"go-build-admin/internal/pkg/clickcaptcha"
	"go-build-admin/internal/pkg/data_scope"
	"go-build-admin/internal/pkg/terminal"
	"go-build-admin/internal/pkg/token"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	terminal.NewTerminal,
	token.NewTokenHelper,
	clickcaptcha.NewClickCaptcha,
	captcha.NewCaptcha,
	data_scope.ProviderSet,
)
