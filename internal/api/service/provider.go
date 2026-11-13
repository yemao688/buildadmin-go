package service

import (
	"buildadmin-go/internal/api/repository"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	repository.ProviderSet,
	NewMemberService,
)
