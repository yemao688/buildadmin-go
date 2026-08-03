package service

import (
	"context"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/common/money"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"

	"gorm.io/gorm"
)

// UserMoneyLogService 承载会员余额日志的业务编排：actor 校验、事务与共享
// 余额原语（money.UserBalanceService）的调用。仓库只保留 scoped 原子原语
// （GetOne/List/Del 与 UserScope）；余额领域错误（ErrInsufficientBalance、
// ErrUserNotFound、ErrNoOwner 等）原样上抛，由 handler 负责 HTTP 映射。
type UserMoneyLogService struct {
	moneyLogM *adminmodel.UserMoneyLogRepository
	balance   *money.UserBalanceService
}

func NewUserMoneyLogService(moneyLogM *adminmodel.UserMoneyLogRepository, balance *money.UserBalanceService) *UserMoneyLogService {
	return &UserMoneyLogService{moneyLogM: moneyLogM, balance: balance}
}

// MoneyLogAddInput carries the transport-free shape of a balance-change
// request. Log is the insert carrier: the generated entry (ID, owner, Before,
// After, Type) is written back into it when non-nil.
type MoneyLogAddInput struct {
	UserID int32
	Delta  float64
	Type   string
	Memo   string
	Log    *model.MoneyLog
	Actor  data_scope.Actor
}

// Add runs the whole balance-change flow: actor validation, the transaction
// around money.UserBalanceService.ApplyDelta (FOR UPDATE read, ownership
// check, negative-balance rejection, atomic update and log write) and the
// domain-error propagation. Any failure rolls back both the balance change
// and the log insert.
func (s *UserMoneyLogService) Add(ctx context.Context, in MoneyLogAddInput) error {
	if data_scope.ValidateActor(in.Actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	return s.moneyLogM.Transaction(ctx, func(tx *gorm.DB) error {
		created, err := s.balance.ApplyDelta(tx, money.ApplyInput{
			UserID: in.UserID,
			Delta:  in.Delta,
			Type:   in.Type,
			Memo:   in.Memo,
			Log:    in.Log,
			Scope:  s.moneyLogM.UserScope(ctx, in.Actor),
		})
		if err != nil {
			return err
		}
		if in.Log != nil {
			*in.Log = *created
		}
		return nil
	})
}
