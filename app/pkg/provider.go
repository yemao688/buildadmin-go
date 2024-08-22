package pkg

import (
	"go-build-admin/app/pkg/captcha"
	"go-build-admin/app/pkg/clickcaptcha"
	"go-build-admin/app/pkg/token"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	token.NewTokenHelper,
	clickcaptcha.NewClickCaptcha,
	captcha.NewCaptcha,
)
