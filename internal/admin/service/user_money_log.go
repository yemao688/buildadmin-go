package service

import (
	"context"

	"buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/common/money"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"

	"gorm.io/gorm"
)

// UserMoneyLogService 承载会员余额日志的业务编排：actor 校验、变动类型收敛、
// 事务与共享余额原语（money.UserBalanceService）的调用。仓库只保留 scoped
// 原子原语（GetOne/List/Del 与 UserScope）；余额领域错误（ErrInsufficientBalance、
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
// After, Type) is written back into it when non-nil; a nil carrier is built
// from the remaining fields.
type MoneyLogAddInput struct {
	UserID int32
	Delta  float64
	Type   string
	Memo   string
	Log    *model.MoneyLog
	Actor  data_scope.Actor
}

// Add 编排余额变动：金额由 DTO 解析校验，变动类型在此收敛（仅超管可指定，
// 其余管理员一律 system），随后进入共享余额事务链。
func (s *UserMoneyLogService) Add(ctx context.Context, p dto.UserMoneyLogParam, actor data_scope.Actor, isSuperAdmin bool) error {
	amount, err := p.Amount()
	if err != nil {
		return err
	}
	return s.add(ctx, MoneyLogAddInput{
		UserID: int32(p.UserID),
		Delta:  amount,
		Type:   moneyLogTypeFor(p.Type, isSuperAdmin),
		Memo:   p.Memo,
		Actor:  actor,
	})
}

// moneyLogTypeFor 决定余额流水的变动类型：仅超管可指定任意类型，其余
// 管理员一律强制 system（资金语义不允许非超管写入非系统类型）。
func moneyLogTypeFor(requested string, isSuperAdmin bool) string {
	if !isSuperAdmin {
		return "system"
	}
	return requested
}

// add runs the whole balance-change flow: actor validation, the transaction
// around money.UserBalanceService.ApplyDelta (FOR UPDATE read, ownership
// check, negative-balance rejection, atomic update and log write) and the
// domain-error propagation. Any failure rolls back both the balance change
// and the log insert.
func (s *UserMoneyLogService) add(ctx context.Context, in MoneyLogAddInput) error {
	if data_scope.ValidateActor(in.Actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	log := in.Log
	if log == nil {
		log = &model.MoneyLog{UserID: in.UserID, Money: in.Delta, Type: in.Type, Memo: in.Memo}
	} else {
		log.Type = in.Type
	}
	return s.moneyLogM.Transaction(ctx, func(tx *gorm.DB) error {
		created, err := s.balance.ApplyDelta(tx, money.ApplyInput{
			UserID: in.UserID,
			Delta:  in.Delta,
			Type:   in.Type,
			Memo:   in.Memo,
			Log:    log,
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
