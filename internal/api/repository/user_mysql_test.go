package repository

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type userFixture struct {
	db   *gorm.DB
	repo *UserRepository
}

// newUserFixture 在独立 fixture 库按配置前缀建 user/admin 表，避免测试之间
// 的表名与数据互相干扰。
func newUserFixture(t *testing.T) *userFixture {
	t.Helper()
	db, cfg := testutil.OpenMySQL(t)
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Admin{}))
	return &userFixture{db: db, repo: NewUserRepository(db, cfg)}
}

func seedUser(t *testing.T, f *userFixture, user model.User) model.User {
	t.Helper()
	if user.Status == "" {
		user.Status = "enable"
	}
	require.NoError(t, f.db.Create(&user).Error)
	return user
}

func TestUserGetByID(t *testing.T) {
	t.Parallel()
	f := newUserFixture(t)
	user := seedUser(t, f, model.User{Username: "get-by-id"})

	found, err := f.repo.GetByID(user.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, user.ID, found.ID)
	require.Equal(t, "get-by-id", found.Username)

	missing, err := f.repo.GetByID(user.ID + 1000)
	require.NoError(t, err)
	require.Nil(t, missing, "记录不存在时必须返回 nil,nil")
}

func TestUserGetByAccount(t *testing.T) {
	t.Parallel()
	f := newUserFixture(t)
	seedUser(t, f, model.User{
		Username: "account-user",
		Email:    "account@example.com",
		Mobile:   "13800000001",
	})

	cases := []struct {
		intent       string
		field        string
		value        string
		wantUsername string
		wantCode     int
		wantNil      bool
	}{
		{intent: "username 命中", field: "username", value: "account-user", wantUsername: "account-user"},
		{intent: "email 命中", field: "email", value: "account@example.com", wantUsername: "account-user"},
		{intent: "mobile 命中", field: "mobile", value: "13800000001", wantUsername: "account-user"},
		{intent: "非白名单字段返回 BadRequest", field: "nickname", value: "account-user", wantCode: http.StatusBadRequest, wantNil: true},
		{intent: "无匹配返回 nil,nil", field: "username", value: "nobody", wantNil: true},
	}
	for _, tc := range cases {
		t.Run(tc.intent, func(t *testing.T) {
			t.Parallel()
			user, err := f.repo.GetByAccount(tc.field, tc.value)
			if tc.wantCode != 0 {
				var businessErr *cErr.Error
				require.ErrorAs(t, err, &businessErr)
				require.Equal(t, tc.wantCode, businessErr.ErrorCode())
				require.Nil(t, user)
				return
			}
			require.NoError(t, err)
			if tc.wantNil {
				require.Nil(t, user)
				return
			}
			require.NotNil(t, user)
			require.Equal(t, tc.wantUsername, user.Username)
		})
	}
}

func TestUserTransactionLockByID(t *testing.T) {
	t.Parallel()
	f := newUserFixture(t)
	user := seedUser(t, f, model.User{Username: "lock-user"})

	var (
		locked  *model.User
		account *model.User
	)
	err := f.repo.Transaction(func(tx *gorm.DB) error {
		var lockErr error
		if locked, lockErr = f.repo.LockByID(tx, user.ID); lockErr != nil {
			return lockErr
		}
		account, lockErr = f.repo.GetByAccountIn(tx, "username", "lock-user")
		return lockErr
	})
	require.NoError(t, err)
	require.NotNil(t, locked)
	require.Equal(t, user.ID, locked.ID)
	require.NotNil(t, account)
	require.Equal(t, user.ID, account.ID)

	err = f.repo.Transaction(func(tx *gorm.DB) error {
		_, lockErr := f.repo.LockByID(tx, user.ID+1000)
		return lockErr
	})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUserCreateAndDuplicate(t *testing.T) {
	t.Parallel()
	f := newUserFixture(t)
	first := model.User{Username: "dup-user", Status: "enable"}
	require.NoError(t, f.repo.Transaction(func(tx *gorm.DB) error {
		return f.repo.Create(tx, &first)
	}))
	require.NotZero(t, first.ID)

	second := model.User{Username: "dup-user", Status: "enable"}
	err := f.repo.Transaction(func(tx *gorm.DB) error {
		return f.repo.Create(tx, &second)
	})
	require.Error(t, err)
	require.True(t, IsDuplicateKeyError(err), "真实 MySQL 1062 应判定为唯一键冲突")
	var mysqlErr *mysql.MySQLError
	require.ErrorAs(t, err, &mysqlErr)
	require.Equal(t, uint16(1062), mysqlErr.Number)
}

func TestUserLoginMeta(t *testing.T) {
	t.Parallel()
	f := newUserFixture(t)
	user := seedUser(t, f, model.User{Username: "meta-user", LoginFailure: 2})

	require.NoError(t, f.repo.UpdateLoginMeta(user.ID, 5, 1700000000, "10.0.0.1"))
	updated, err := f.repo.GetByID(user.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, int32(5), updated.LoginFailure)
	require.Equal(t, int64(1700000000), updated.LastLoginTime)
	require.Equal(t, "10.0.0.1", updated.LastLoginIP)

	require.NoError(t, f.repo.ResetLoginFailure(user.ID))
	reset, err := f.repo.GetByID(user.ID)
	require.NoError(t, err)
	require.NotNil(t, reset)
	require.Zero(t, reset.LoginFailure)
	require.Equal(t, int64(1700000000), reset.LastLoginTime, "ResetLoginFailure 不应清零其它登录元信息")
	require.Equal(t, "10.0.0.1", reset.LastLoginIP)
}

func TestUserResolveRootAdminID(t *testing.T) {
	t.Parallel()

	t.Run("优先返回 id=1 的根管理员", func(t *testing.T) {
		t.Parallel()
		f := newUserFixture(t)
		require.NoError(t, f.db.Create(&model.Admin{ID: 1, Username: "root"}).Error)
		require.NoError(t, f.db.Create(&model.Admin{ID: 2, Username: "second"}).Error)

		id, err := f.repo.ResolveRootAdminID()
		require.NoError(t, err)
		require.Equal(t, int32(1), id)
	})

	t.Run("无 id=1 时回退到最小 id", func(t *testing.T) {
		t.Parallel()
		f := newUserFixture(t)
		require.NoError(t, f.db.Create(&model.Admin{ID: 7, Username: "seven"}).Error)
		require.NoError(t, f.db.Create(&model.Admin{ID: 3, Username: "three"}).Error)

		id, err := f.repo.ResolveRootAdminID()
		require.NoError(t, err)
		require.Equal(t, int32(3), id)
	})

	t.Run("管理员表为空时报错", func(t *testing.T) {
		t.Parallel()
		f := newUserFixture(t)

		id, err := f.repo.ResolveRootAdminID()
		require.Error(t, err)
		require.Zero(t, id)
		require.Contains(t, err.Error(), "unavailable")
	})
}

func TestIsDuplicateKeyError(t *testing.T) {
	t.Parallel()

	require.True(t, IsDuplicateKeyError(gorm.ErrDuplicatedKey))
	require.True(t, IsDuplicateKeyError(fmt.Errorf("create user: %w", gorm.ErrDuplicatedKey)))
	require.True(t, IsDuplicateKeyError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}))
	require.False(t, IsDuplicateKeyError(&mysql.MySQLError{Number: 1146, Message: "Table doesn't exist"}))
	require.False(t, IsDuplicateKeyError(errors.New("some other error")))
	require.False(t, IsDuplicateKeyError(nil))
}
