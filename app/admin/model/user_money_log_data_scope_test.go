package model

import (
	"errors"
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMoneyLogScopeOwnerCentsBoundariesAndDelete(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	u := f.addUser(t, ctx, 20, "money")
	u.Money = 100
	require.NoError(t, f.db.Model(&User{}).Where("id = ?", u.ID).Update("money", 100).Error)
	spoof := &UserMoneyLog{AdminID: 40, UserID: u.ID, Money: 125, MoneyCents: true, Memo: "one"}
	require.NoError(t, f.money.Add(ctx, spoof))
	require.Equal(t, int32(20), spoof.AdminID)
	require.Equal(t, int32(100), spoof.Before)
	require.Equal(t, int32(225), spoof.After)
	var got User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(225), got.Money)
	logs, total, err := f.money.List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	_, err = f.money.GetOne(ctx, spoof.ID)
	require.NoError(t, err)
	require.Error(t, f.money.Del(ctx, []int32{spoof.ID + 1}))
	require.NoError(t, f.money.Del(ctx, []int32{spoof.ID}))
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(225), got.Money)

	require.NoError(t, f.db.Model(&User{}).Where("id = ?", u.ID).Updates(map[string]any{"money": 0}).Error)
	require.Error(t, f.money.Add(ctx, &UserMoneyLog{UserID: u.ID, Money: -1, MoneyCents: true, Memo: "too low"}))
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(0), got.Money)
	require.NoError(t, f.db.Model(&User{}).Where("id = ?", u.ID).Update("money", math.MaxInt32-1).Error)
	require.Error(t, f.money.Add(ctx, &UserMoneyLog{UserID: u.ID, Money: 2, MoneyCents: true, Memo: "overflow"}))
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(math.MaxInt32-1), got.Money)
}

func TestMoneyLogConcurrentForUpdateFormsContinuousChain(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	u := f.addUser(t, ctx, 20, "money-chain")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.money.Add(ctx, &UserMoneyLog{UserID: u.ID, Money: 100, MoneyCents: true, Memo: "chain"})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var got User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(200), got.Money)
	var logs []UserMoneyLog
	require.NoError(t, f.db.Where("user_id = ?", u.ID).Order("id").Find(&logs).Error)
	require.Len(t, logs, 2)
	require.Equal(t, int32(0), logs[0].Before)
	require.Equal(t, int32(100), logs[0].After)
	require.Equal(t, int32(100), logs[1].Before)
	require.Equal(t, int32(200), logs[1].After)
}

func TestMoneyLogInsertFailureRollsBackBalanceAndDeleteCannotOrphan(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	u := f.addUser(t, ctx, 20, "money-rollback")
	duplicate := UserMoneyLog{ID: 77, UserID: u.ID, AdminID: 20, Money: 1, Before: 0, After: 1, Memo: "existing"}
	require.NoError(t, f.db.Create(&duplicate).Error)
	err := f.money.Add(ctx, &UserMoneyLog{ID: 77, UserID: u.ID, Money: 100, MoneyCents: true, Memo: "duplicate"})
	require.Error(t, err)
	var got User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(0), got.Money)
	require.Error(t, f.root.Del(ctx, []int32{u.ID}))
	var count int64
	require.NoError(t, f.db.Model(&User{}).Where("id = ?", u.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
	_ = errors.Is(err, gorm.ErrDuplicatedKey)
}
