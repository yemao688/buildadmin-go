package service

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"buildadmin-go/internal/admin/dto"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/validator"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserMoneyLogTypeFor(t *testing.T) {
	// 超管可指定任意类型（含空 → 交给 money 链路的缺省 system）。
	require.Equal(t, "recharge", moneyLogTypeFor("recharge", true))
	require.Equal(t, "withdraw", moneyLogTypeFor("withdraw", true))
	require.Empty(t, moneyLogTypeFor("", true))
	// 非超管一律强制 system，忽略请求值。
	require.Equal(t, "system", moneyLogTypeFor("recharge", false))
	require.Equal(t, "system", moneyLogTypeFor("extend", false))
	require.Equal(t, "system", moneyLogTypeFor("", false))
}

func TestMoneyLogScopeOwnerBoundariesAndDelete(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	actor := scopeActor(t, ctx)
	u := f.addUser(t, ctx, 20, "money")
	u.Money = 1.00
	require.NoError(t, f.db.Model(&model.User{}).Where("id = ?", u.ID).Update("money", 1.00).Error)
	spoof := &model.MoneyLog{AdminID: 40, UserID: u.ID, Money: 1.25, Memo: "one"}
	require.NoError(t, f.moneySvc.add(ctx.Request.Context(), MoneyLogAddInput{UserID: spoof.UserID, Delta: spoof.Money, Memo: spoof.Memo, Log: spoof, Actor: actor}))
	require.Equal(t, int32(20), spoof.AdminID)
	require.InDelta(t, float64(1.00), spoof.Before, 1e-9)
	require.InDelta(t, float64(2.25), spoof.After, 1e-9)
	require.Equal(t, "system", spoof.Type)
	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(2.25), got.Money, 1e-9)
	logs, total, err := f.money.List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	_, err = f.money.GetOne(ctx, spoof.ID)
	require.NoError(t, err)
	require.Error(t, f.money.Del(ctx, []int32{spoof.ID + 1}))
	require.NoError(t, f.money.Del(ctx, []int32{spoof.ID}))
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(2.25), got.Money, 1e-9)

	require.NoError(t, f.db.Model(&model.User{}).Where("id = ?", u.ID).Updates(map[string]any{"money": 0}).Error)
	require.Error(t, f.moneySvc.add(ctx.Request.Context(), MoneyLogAddInput{UserID: u.ID, Delta: -1, Memo: "too low", Actor: actor}))
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(0), got.Money, 1e-9)
}

func TestMoneyLogConcurrentForUpdateFormsContinuousChain(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	actor := scopeActor(t, ctx)
	u := f.addUser(t, ctx, 20, "money-chain")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.moneySvc.Add(ctx.Request.Context(), dto.UserMoneyLogParam{UserID: validator.FlexInt32(u.ID), Money: json.RawMessage(`"1.00"`), Memo: "chain"}, actor, false)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(2.00), got.Money, 1e-9)
	var logs []model.MoneyLog
	require.NoError(t, f.db.Where("user_id = ?", u.ID).Order("id").Find(&logs).Error)
	require.Len(t, logs, 2)
	require.InDelta(t, float64(0), logs[0].Before, 1e-9)
	require.InDelta(t, float64(1.00), logs[0].After, 1e-9)
	require.InDelta(t, float64(1.00), logs[1].Before, 1e-9)
	require.InDelta(t, float64(2.00), logs[1].After, 1e-9)
}

func TestMoneyLogInsertFailureRollsBackBalanceAndDeleteCannotOrphan(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	actor := scopeActor(t, ctx)
	u := f.addUser(t, ctx, 20, "money-rollback")
	duplicate := model.MoneyLog{ID: 77, UserID: u.ID, AdminID: 20, Money: 1.00, Before: 0, After: 1.00, Memo: "existing"}
	require.NoError(t, f.db.Create(&duplicate).Error)
	carrier := &model.MoneyLog{ID: 77, UserID: u.ID, Money: 1.00, Memo: "duplicate"}
	err := f.moneySvc.add(ctx.Request.Context(), MoneyLogAddInput{UserID: carrier.UserID, Delta: carrier.Money, Memo: carrier.Memo, Log: carrier, Actor: actor})
	require.Error(t, err)
	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(0), got.Money, 1e-9)
	require.Error(t, f.userSvc.Del(ctx.Request.Context(), []int32{u.ID}, actor))
	var count int64
	require.NoError(t, f.db.Model(&model.User{}).Where("id = ?", u.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
	_ = errors.Is(err, gorm.ErrDuplicatedKey)
}
