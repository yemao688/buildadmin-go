package model

import (
	"go-build-admin/app/common/model/country"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAreaModel,
	NewAttachmentModel,
	NewUploadHelper,
	NewAliossStorage,
	NewUserMoneyLogModel,
	NewUserScoreLogModel,
	NewUserModel,
	NewAuthModel,
	country.NewService,
)
