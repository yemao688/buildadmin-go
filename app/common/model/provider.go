package model

import (
	"go-build-admin/app/common/model/country"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAreaModel,
	country.NewService,
)
