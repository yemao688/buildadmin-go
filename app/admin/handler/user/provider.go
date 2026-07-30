package user

import (
	"github.com/google/wire"
	"go-build-admin/app/common/member"
)

func NewMemberPermissionInvalidator(s *member.Service) MemberPermissionInvalidator {
	return s
}

var ProviderSet = wire.NewSet(
	NewMemberPermissionInvalidator,
	NewUserHandlerWithAuth,
	NewUserRegistrar,
	NewGroupHandlerWithAuth,
	NewGroupRegistrar,
	NewRuleHandlerWithAuth,
	NewRuleRegistrar,
	NewMoneyLogHandler,
	NewScoreLogHandler,
	NewUserLogRegistrar,
)
