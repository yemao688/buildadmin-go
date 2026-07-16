package model

import (
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScoreLogScopeOwnerBoundariesAndDelete(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	u := f.addUser(t, ctx, 20, "score")
	require.NoError(t, f.db.Model(&User{}).Where("id = ?", u.ID).Update("score", 10).Error)
	log := &UserScoreLog{AdminID: 40, UserID: u.ID, Score: 5, Memo: "five"}
	require.NoError(t, f.score.Add(ctx, log))
	require.Equal(t, int32(20), log.AdminID)
	require.Equal(t, int32(10), log.Before)
	require.Equal(t, int32(15), log.After)
	var got User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(15), got.Score)
	list, total, err := f.score.List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	_, err = f.score.GetOne(ctx, log.ID)
	require.NoError(t, err)
	require.NoError(t, f.score.Del(ctx, []int32{log.ID}))
	require.NoError(t, f.db.Model(&User{}).Where("id = ?", u.ID).Update("score", 0).Error)
	require.Error(t, f.score.Add(ctx, &UserScoreLog{UserID: u.ID, Score: -1, Memo: "low"}))
	require.NoError(t, f.db.Model(&User{}).Where("id = ?", u.ID).Update("score", math.MaxInt32-1).Error)
	require.Error(t, f.score.Add(ctx, &UserScoreLog{UserID: u.ID, Score: 2, Memo: "overflow"}))
}

func TestScoreLogConcurrentForUpdateFormsContinuousChain(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	u := f.addUser(t, ctx, 20, "score-chain")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.score.Add(ctx, &UserScoreLog{UserID: u.ID, Score: 7, Memo: "chain"})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var got User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(14), got.Score)
	var logs []UserScoreLog
	require.NoError(t, f.db.Where("user_id = ?", u.ID).Order("id").Find(&logs).Error)
	require.Len(t, logs, 2)
	require.Equal(t, int32(0), logs[0].Before)
	require.Equal(t, int32(7), logs[0].After)
	require.Equal(t, int32(7), logs[1].Before)
	require.Equal(t, int32(14), logs[1].After)
}

func TestScoreLogInsertFailureRollsBackAndOrphanHistoryIsVisibleToUnrestricted(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	u := f.addUser(t, ctx, 20, "score-rollback")
	duplicate := UserScoreLog{ID: 88, UserID: u.ID, AdminID: 20, Score: 1, Before: 0, After: 1, Memo: "existing"}
	require.NoError(t, f.db.Create(&duplicate).Error)
	require.Error(t, f.score.Add(ctx, &UserScoreLog{ID: 88, UserID: u.ID, Score: 9, Memo: "duplicate"}))
	var got User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, int32(0), got.Score)
	require.Error(t, f.root.Del(ctx, []int32{u.ID}))
	orphan := UserScoreLog{UserID: 999999, AdminID: 0, Score: 3, Memo: "legacy orphan"}
	require.NoError(t, f.db.Create(&orphan).Error)
	unrestricted := scopeCtx(t, 10, true)
	list, total, err := f.score.List(unrestricted)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	_, err = f.score.GetOne(unrestricted, orphan.ID)
	require.NoError(t, err)
}
