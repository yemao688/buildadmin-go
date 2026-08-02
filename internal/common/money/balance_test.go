package money

import (
	"fmt"
	"sync"
	"testing"
	"time"

	model "go-build-admin/internal/model"
	"go-build-admin/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// fixture opens the shared mysql_test database with an isolated per-test table
// prefix (gated by testutil.OpenMySQL; skipped when mysql_test is disabled).
type fixture struct {
	t   *testing.T
	db  *gorm.DB
	svc *BalanceService
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
	return &fixture{t: t, db: db, svc: NewBalanceService()}
}

// addUser inserts a user and returns it.
func (f *fixture) addUser(t *testing.T, owner int32, name string) model.User {
	u := model.User{AdminID: owner, Username: name, Nickname: name, Password: "p", Status: "enable"}
	require.NoError(t, f.db.Create(&u).Error)
	return u
}

// apply runs one balance change inside a real transaction, mirroring how
// adapters will call the service (FOR UPDATE requires an open transaction).
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

// logs returns the money logs of a user ordered by id.
func (f *fixture) logs(userID int32) []model.MoneyLog {
	var logs []model.MoneyLog
	require.NoError(f.t, f.db.Where("user_id = ?", userID).Order("id").Find(&logs).Error)
	return logs
}

// countLogs returns the number of money logs of a user.
func (f *fixture) countLogs(userID int32) int64 {
	var n int64
	require.NoError(f.t, f.db.Model(&model.MoneyLog{}).Where("user_id = ?", userID).Count(&n).Error)
	return n
}

func TestApplyDeltaSystemFlowSuccess(t *testing.T) {
	f := newFixture(t)
	u := f.addUser(t, 20, "money-system")

	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1.25, Memo: "system", Scope: SystemScope()})
	require.NoError(t, err)

	// Log fields mirror the legacy semantics: AdminID is the target user's
	// owner, not an operator identity.
	require.Equal(t, int32(20), log.AdminID)
	require.Equal(t, u.ID, log.UserID)
	require.Equal(t, float64(0), log.Before)
	require.Equal(t, float64(1.25), log.Money)
	require.Equal(t, float64(1.25), log.After)
	require.Equal(t, "system", log.Memo)
	require.Greater(t, log.ID, int32(0))

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, float64(1.25), got.Money)
}

func TestApplyDeltaRejectsInsufficientBalance(t *testing.T) {
	f := newFixture(t)
	u := f.addUser(t, 20, "money-insufficient")

	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: -1, Memo: "too low", Scope: SystemScope()})
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Nil(t, log)

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, float64(0), got.Money)
	require.Equal(t, int64(0), f.countLogs(u.ID))
}

func TestApplyDeltaReturnsNotFoundForUnknownUser(t *testing.T) {
	f := newFixture(t)

	log, err := f.apply(ApplyInput{UserID: 999999, Delta: 1, Scope: SystemScope()})
	require.ErrorIs(t, err, ErrUserNotFound)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.Nil(t, log)
}

func TestApplyDeltaRejectsNoOwner(t *testing.T) {
	f := newFixture(t)
	u := f.addUser(t, 0, "money-no-owner")

	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1, Scope: SystemScope()})
	require.ErrorIs(t, err, ErrNoOwner)
	require.Nil(t, log)
	require.Equal(t, int64(0), f.countLogs(u.ID))
}

func TestApplyDeltaRequiresScope(t *testing.T) {
	f := newFixture(t)
	u := f.addUser(t, 20, "money-no-scope")

	// A nil scope is rejected instead of silently meaning "unrestricted".
	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1})
	require.ErrorIs(t, err, ErrScopeRequired)
	require.Nil(t, log)

	_, err = f.svc.ApplyDelta(nil, ApplyInput{UserID: u.ID, Delta: 1, Scope: SystemScope()})
	require.ErrorIs(t, err, gorm.ErrInvalidDB)
}

func TestApplyDeltaHonorsScope(t *testing.T) {
	f := newFixture(t)
	u := f.addUser(t, 20, "money-scoped")

	// A scope that denies the owner must fail the locked read, proving the
	// scope is applied to the balance query itself.
	denied := Scope(func(db *gorm.DB) *gorm.DB {
		return db.Where("admin_id = ?", 40)
	})
	log, err := f.apply(ApplyInput{UserID: u.ID, Delta: 1, Scope: denied})
	require.ErrorIs(t, err, ErrUserNotFound)
	require.Nil(t, log)

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, float64(0), got.Money)
	require.Equal(t, int64(0), f.countLogs(u.ID))
}

func TestApplyDeltaNegativeAndPositiveDeltas(t *testing.T) {
	f := newFixture(t)
	u := f.addUser(t, 20, "money-chain")

	_, err := f.apply(ApplyInput{UserID: u.ID, Delta: 10, Memo: "deposit", Scope: SystemScope()})
	require.NoError(t, err)
	_, err = f.apply(ApplyInput{UserID: u.ID, Delta: -4, Memo: "withdraw", Scope: SystemScope()})
	require.NoError(t, err)

	var got model.User
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, float64(6), got.Money)

	logs := f.logs(u.ID)
	require.Len(t, logs, 2)
	require.Equal(t, float64(0), logs[0].Before)
	require.Equal(t, float64(10), logs[0].After)
	require.Equal(t, float64(10), logs[1].Before)
	require.Equal(t, float64(6), logs[1].After)
}

func TestApplyDeltaConcurrentForUpdateConsistency(t *testing.T) {
	f := newFixture(t)
	u := f.addUser(t, 20, "money-concurrent")

	// Seed balance 10, then apply +5 and -3 concurrently. FOR UPDATE must
	// serialize the two transactions so the chain stays continuous and the
	// final balance is exact.
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
	require.Equal(t, float64(12), got.Money)

	logs := f.logs(u.ID)
	require.Len(t, logs, 3)
	require.Equal(t, float64(0), logs[0].Before)
	require.Equal(t, float64(10), logs[0].After)
	// Either credit-then-debit (10->15->12) or debit-then-credit (10->7->12);
	// the chain must be continuous either way.
	require.Equal(t, logs[1].After, logs[2].Before)
	require.Equal(t, float64(12), logs[2].After)
}
