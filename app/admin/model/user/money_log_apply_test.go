package user

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplyMoneyDeltaSystemFlow(t *testing.T) {
	f := newScopeFixture(t)
	u := f.addUser(t, scopeCtx(t, 20, false), 20, "money-system")
	log := &MoneyLog{AdminID: 40, UserID: u.ID, Money: 99, Memo: "system"}

	err := f.db.Transaction(func(tx *gorm.DB) error {
		return f.money.ApplyMoneyDelta(tx, ApplyMoneyDeltaInput{
			UserID:          u.ID,
			Delta:           1.25,
			Log:             log,
			OperatorAdminID: 0,
		})
	})
	require.NoError(t, err)
	require.Equal(t, int32(20), log.AdminID)
	require.Equal(t, u.ID, log.UserID)
	require.Equal(t, float64(0), log.Before)
	require.Equal(t, float64(1.25), log.Money)
	require.Equal(t, float64(1.25), log.After)

	var got User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, float64(1.25), got.Money)
}

func TestApplyMoneyDeltaRejectsInsufficientBalance(t *testing.T) {
	f := newScopeFixture(t)
	u := f.addUser(t, scopeCtx(t, 20, false), 20, "money-insufficient")
	log := &MoneyLog{Memo: "too low"}

	err := f.db.Transaction(func(tx *gorm.DB) error {
		return f.money.ApplyMoneyDelta(tx, ApplyMoneyDeltaInput{
			UserID: u.ID,
			Delta:  -1,
			Log:    log,
		})
	})
	require.EqualError(t, err, "insufficient balance")

	var got User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, float64(0), got.Money)
}

func TestApplyMoneyDeltaReturnsNotFoundForUnknownUser(t *testing.T) {
	f := newScopeFixture(t)
	log := &MoneyLog{Memo: "missing"}

	err := f.db.Transaction(func(tx *gorm.DB) error {
		return f.money.ApplyMoneyDelta(tx, ApplyMoneyDeltaInput{
			UserID: 999999,
			Delta:  1,
			Log:    log,
		})
	})
	require.True(t, errors.Is(err, gorm.ErrRecordNotFound), "error = %v", err)
}
