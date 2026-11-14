package money

import (
	"fmt"
	"sync"
	"testing"
	"time"

	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// fixture 使用独立的每测试表前缀打开共享的 mysql_test 数据库
// （由 testutil.OpenMySQL 门禁；mysql_test 禁用时跳过）。
type fixture struct {
	t   *testing.T
	db  *gorm.DB
	svc *UserBalanceService
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	prefix := fmt.Sprintf("it_%d_", time.Now().UnixNano())
	db, _ := testutil.OpenMySQL(t)
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.MoneyLog{}))
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "user_money_log`")
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "user`")
	})
	return &fixture{t: t, db: db, svc: NewUserBalanceService()}
}

func (f *fixture) addUser(t *testing.T, owner int32, name string) model.User {
	t.Helper()
	u := model.User{AdminID: owner, Username: name, Nickname: name, Password: "p", Status: "enable"}
	require.NoError(t, f.db.Create(&u).Error)
	return u
}

// apply 在真实事务中执行一次余额变动，模拟适配器的调用方式
// （FOR UPDATE 需要打开的事务）。
func (f *fixture) apply(in ApplyInput) (*model.MoneyLog, error) {
	var (
		log *model.MoneyLog
		err error
	)
	err = f.db.Transaction(func(tx *gorm.DB) error {
		log, err = f.svc.ApplyDelta(tx, in)
		return err
	})
	return log, err
}

func (f *fixture) logs(userID int32) []model.MoneyLog {
	var logs []model.MoneyLog
	require.NoError(f.t, f.db.Where("user_id = ?", userID).Order("id").Find(&logs).Error)
	return logs
}

func (f *fixture) countLogs(userID int32) int64 {
	var n int64
	require.NoError(f.t, f.db.Model(&model.MoneyLog{}).Where("user_id = ?", userID).Count(&n).Error)
	return n
}

func TestApplyDeltaSystemFlowSuccess(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.addUser(t, 20, "money-system")

	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1.25, Memo: "system", Scope: SystemScope()})
	require.NoError(t, err)

	// 日志字段沿用既有语义：AdminID 是目标用户的属主，不是操作者身份。
	require.Equal(t, int32(20), log.AdminID)
	require.Equal(t, u.ID, log.UserID)
	require.InDelta(t, float64(0), log.Before, 1e-9)
	require.InDelta(t, float64(1.25), log.Money, 1e-9)
	require.InDelta(t, float64(1.25), log.After, 1e-9)
	require.Equal(t, "system", log.Memo)
	require.Positive(t, log.ID)

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(1.25), got.Money, 1e-9)
}

func TestApplyDeltaRejectsInsufficientBalance(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.addUser(t, 20, "money-insufficient")

	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: -1, Memo: "too low", Scope: SystemScope()})
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Nil(t, log)

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(0), got.Money, 1e-9)
	require.Equal(t, int64(0), f.countLogs(u.ID))
}

func TestApplyDeltaReturnsNotFoundForUnknownUser(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	log, err := f.apply(ApplyInput{UserID: 999999, Delta: 1, Scope: SystemScope()})
	require.ErrorIs(t, err, ErrUserNotFound)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.Nil(t, log)
}

func TestApplyDeltaRejectsNoOwner(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.addUser(t, 0, "money-no-owner")

	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1, Scope: SystemScope()})
	require.ErrorIs(t, err, ErrNoOwner)
	require.Nil(t, log)
	require.Equal(t, int64(0), f.countLogs(u.ID))
}

func TestApplyDeltaRequiresScope(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.addUser(t, 20, "money-no-scope")

	// nil scope 会被拒绝，而不是被静默当作“不受限”。
	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1})
	require.ErrorIs(t, err, ErrScopeRequired)
	require.Nil(t, log)

	_, err = f.svc.ApplyDelta(nil, ApplyInput{UserID: u.ID, Delta: 1, Scope: SystemScope()})
	require.ErrorIs(t, err, gorm.ErrInvalidDB)
}

func TestApplyDeltaHonorsScope(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.addUser(t, 20, "money-scoped")

	// 拒绝属主的 scope 必须让加锁读取失败，证明 scope 确实作用在余额查询本身。
	denied := Scope(func(db *gorm.DB) *gorm.DB {
		return db.Where("admin_id = ?", 40)
	})
	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1, Scope: denied})
	require.ErrorIs(t, err, ErrUserNotFound)
	require.Nil(t, log)

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(0), got.Money, 1e-9)
	require.Equal(t, int64(0), f.countLogs(u.ID))
}

func TestApplyDeltaNegativeAndPositiveDeltas(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.addUser(t, 20, "money-chain")

	_, err := f.apply(ApplyInput{UserID: u.ID, Delta: 10, Memo: "deposit", Scope: SystemScope()})
	require.NoError(t, err)
	_, err = f.apply(ApplyInput{UserID: u.ID, Delta: -4, Memo: "withdraw", Scope: SystemScope()})
	require.NoError(t, err)

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(6), got.Money, 1e-9)

	logs := f.logs(u.ID)
	require.Len(t, logs, 2)
	require.InDelta(t, float64(0), logs[0].Before, 1e-9)
	require.InDelta(t, float64(10), logs[0].After, 1e-9)
	require.InDelta(t, float64(10), logs[1].Before, 1e-9)
	require.InDelta(t, float64(6), logs[1].After, 1e-9)
}

func TestApplyDeltaTypeResolution(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.addUser(t, 20, "money-type")

	// Type 为空且 Log 载波未预置时默认 "system"。
	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1, Scope: SystemScope()})
	require.NoError(t, err)
	require.Equal(t, "system", log.Type)

	// Log 载波未预置时以 ApplyInput.Type 为准。
	log, err = f.apply(ApplyInput{UserID: u.ID, Delta: 1, Type: "recharge", Scope: SystemScope()})
	require.NoError(t, err)
	require.Equal(t, "recharge", log.Type)

	// Log 载波上预置的 Type 优先于 ApplyInput.Type。
	carrier := &model.MoneyLog{Type: "withdraw"}
	log, err = f.apply(ApplyInput{UserID: u.ID, Delta: 1, Type: "recharge", Log: carrier, Scope: SystemScope()})
	require.NoError(t, err)
	require.Equal(t, "withdraw", log.Type)
	require.Same(t, carrier, log)
}

func TestApplyDeltaConcurrentForUpdateConsistency(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	u := f.addUser(t, 20, "money-concurrent")

	// 先播种余额 10，再并发 +5 与 -3。FOR UPDATE 必须串行化两个事务，
	// 使变动链保持连续且最终余额精确。
	_, err := f.apply(ApplyInput{UserID: u.ID, Delta: 10, Scope: SystemScope()})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	apply := func(delta float64, memo string) {
		defer wg.Done()
		_, err := f.apply(ApplyInput{UserID: u.ID, Delta: delta, Memo: memo, Scope: SystemScope()})
		errs <- err
	}
	wg.Add(2)
	go apply(5, "credit")
	go apply(-3, "debit")
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.InDelta(t, float64(12), got.Money, 1e-9)

	logs := f.logs(u.ID)
	require.Len(t, logs, 3)
	require.InDelta(t, float64(0), logs[0].Before, 1e-9)
	require.InDelta(t, float64(10), logs[0].After, 1e-9)
	// 先充值后扣款（10->15->12）或先扣款后充值（10->7->12）均可，
	// 但变动链必须连续。
	require.InDelta(t, logs[1].After, logs[2].Before, 1e-9)
	require.InDelta(t, float64(12), logs[2].After, 1e-9)
}
