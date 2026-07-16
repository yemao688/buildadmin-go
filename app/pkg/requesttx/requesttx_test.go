package requesttx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestActiveTransactionIsReusedAndNotCommitted(t *testing.T) {
	tx := &gorm.DB{}
	ctx := Bind(context.Background(), tx)
	var got *gorm.DB
	require.NoError(t, Transaction(ctx, func(db *gorm.DB) error {
		got = db
		return Transaction(ctx, func(nested *gorm.DB) error {
			require.Same(t, tx, nested)
			return nil
		})
	}))
	require.Same(t, tx, got)
	require.True(t, Active(ctx))
}

func TestFallbackWithoutDatabaseReturnsError(t *testing.T) {
	err := Transaction(WithDB(context.Background(), nil), func(*gorm.DB) error { return nil })
	require.EqualError(t, err, "requesttx: no database bound to context")
}

func TestOutcomeStagesOnceAndCanBeDiscarded(t *testing.T) {
	ctx := Bind(context.Background(), &gorm.DB{})
	out := Outcome{HTTPCode: 200, BusinessCode: 1, Message: "ok", Data: "value"}
	require.True(t, Stage(ctx, out))
	require.False(t, Stage(ctx, Outcome{Message: "duplicate"}))
	got, ok := TakeOutcome(ctx)
	require.True(t, ok)
	require.Equal(t, out, got)
	require.False(t, Stage(ctx, out))

	ctx = Bind(context.Background(), &gorm.DB{})
	require.True(t, Stage(ctx, out))
	DiscardOutcome(ctx)
	_, ok = TakeOutcome(ctx)
	require.False(t, ok)
}

func TestFinishUnbindsAndClearsTransaction(t *testing.T) {
	parent := context.WithValue(context.Background(), "request", "original")
	bound := Bind(parent, &gorm.DB{})
	require.True(t, Active(bound))
	Finish(bound)
	require.False(t, Active(bound))
	require.Nil(t, DB(bound))
	require.Equal(t, parent, Unbind(bound))
}
