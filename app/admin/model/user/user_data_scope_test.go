package user

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	adminauth "go-build-admin/app/admin/model/auth"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/app/pkg/requesttx"
	"go-build-admin/app/pkg/testutil"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Admin = adminauth.Admin
type AdminGroup = adminauth.AdminGroup

type scopeFixture struct {
	db     *gorm.DB
	cfg    *conf.Configuration
	root   *UserModel
	money  *MoneyLogModel
	admins map[int32]Admin
	users  map[int32]User
}

func newScopeFixture(t *testing.T) *scopeFixture {
	t.Helper()
	prefix := fmt.Sprintf("it_%d_", time.Now().UnixNano())
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = prefix
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	f := &scopeFixture{db: db, cfg: cfg, admins: map[int32]Admin{}, users: map[int32]User{}}
	require.NoError(t, db.AutoMigrate(&Admin{}, &AdminGroup{}, &User{}))
	require.NoError(t, db.Exec("CREATE TABLE `"+prefix+"user_money_log` (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, user_id INT NOT NULL, money DECIMAL(12,2) NOT NULL, `before` DECIMAL(12,2) NOT NULL, `after` DECIMAL(12,2) NOT NULL, memo VARCHAR(255) NOT NULL DEFAULT '', create_time BIGINT NOT NULL)").Error)
	require.NoError(t, db.Exec("ALTER TABLE `"+prefix+"user` MODIFY `last_login_ip` VARCHAR(50) NOT NULL DEFAULT '', MODIFY `login_failure` INT NOT NULL DEFAULT 0").Error)
	closure := prefix + "admin_closure"
	require.NoError(t, db.Exec("CREATE TABLE `"+closure+"` (`ancestor_id` INT NOT NULL, `descendant_id` INT NOT NULL, `depth` INT NOT NULL, PRIMARY KEY (`ancestor_id`,`descendant_id`), KEY (`descendant_id`,`ancestor_id`)) ENGINE=InnoDB").Error)
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "user_money_log`")
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "user`")
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "admin_closure`")
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "admin`")
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "admin_group`")
	})
	for _, a := range []Admin{{ID: 10, Username: "root", Nickname: "root"}, {ID: 20, Username: "child", Nickname: "child"}, {ID: 30, Username: "leaf", Nickname: "leaf"}, {ID: 40, Username: "other", Nickname: "other"}} {
		require.NoError(t, db.Create(&a).Error)
		f.admins[a.ID] = a
	}
	for _, row := range []struct{ a, d, depth int32 }{{10, 10, 0}, {10, 20, 1}, {10, 30, 2}, {10, 40, 1}, {20, 20, 0}, {20, 30, 1}, {30, 30, 0}, {40, 40, 0}} {
		require.NoError(t, db.Table(closure).Create(map[string]any{"ancestor_id": row.a, "descendant_id": row.d, "depth": row.depth}).Error)
	}
	f.root = NewUserModel(db, cfg, data_scope.NewClosureEnforcer(cfg))
	f.money = NewMoneyLogModel(db, cfg, data_scope.NewClosureEnforcer(cfg))
	return f
}

func scopeCtx(t *testing.T, id int32, unrestricted bool) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	a := data_scope.Actor{AdminID: id, Unrestricted: unrestricted}
	require.NoError(t, data_scope.SetActor(c, a))
	c.Request = httptest.NewRequest("GET", "/?limit=100", nil)
	return c
}

func (f *scopeFixture) addUser(t *testing.T, ctx *gin.Context, adminID int32, name string) User {
	u := User{AdminID: adminID, Username: name, Nickname: name, Password: "p", Status: "enable"}
	require.NoError(t, f.db.Create(&u).Error)
	f.users[u.ID] = u
	return u
}

func TestUserClosureScopeCRUDAndSelect(t *testing.T) {
	f := newScopeFixture(t)
	root := scopeCtx(t, 10, false)
	child := scopeCtx(t, 20, false)
	leaf := scopeCtx(t, 30, false)
	other := scopeCtx(t, 40, false)
	u20 := f.addUser(t, root, 20, "u20")
	u30 := f.addUser(t, root, 30, "u30")
	u40 := f.addUser(t, root, 40, "u40")
	list, total, err := f.root.List(child)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(2))
	require.GreaterOrEqual(t, len(list), 2)
	var ids []int32
	for _, row := range list {
		ids = append(ids, row.ID)
	}
	require.Contains(t, ids, u20.ID)
	require.Contains(t, ids, u30.ID)
	_, err = f.root.GetOne(leaf, u20.ID)
	require.Error(t, err)
	_, err = f.root.GetOne(child, u30.ID)
	require.NoError(t, err)
	_, err = f.root.GetOne(child, u40.ID)
	require.Error(t, err)
	for _, c := range []*gin.Context{child, leaf} {
		u := User{ID: u40.ID, Username: "spoof", Nickname: "spoof", Status: "enable"}
		require.Error(t, f.root.Edit(c, &u, ""))
		require.Error(t, f.root.UpdateStatus(c, u40.ID, "disable"))
	}
	newUser := User{AdminID: 30, Username: "forged", Nickname: "forged", Password: "p", Status: "enable"}
	require.NoError(t, f.root.Add(child, &newUser))
	require.Equal(t, int32(30), newUser.AdminID)
	require.Error(t, f.root.Del(leaf, []int32{u20.ID}))
	require.NoError(t, f.root.Del(child, []int32{u30.ID}))
	require.NoError(t, f.root.Del(other, []int32{u40.ID}))
	// A normal scoped list is also the select source and must never expose a sibling.
	list, _, err = f.root.List(child)
	require.NoError(t, err)
	for _, row := range list {
		require.NotEqual(t, u40.ID, row.ID)
	}
}

func TestUserOwnerAssignmentAndLogTransfer(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	defaultOwner := User{Username: "default-owner", Nickname: "default-owner", Password: "p", Status: "enable"}
	require.NoError(t, f.root.Add(ctx, &defaultOwner))
	require.Equal(t, int32(20), defaultOwner.AdminID)

	specified := User{AdminID: 30, Username: "specified-owner", Nickname: "specified-owner", Password: "p", Status: "enable"}
	require.NoError(t, f.root.Add(ctx, &specified))
	require.Equal(t, int32(30), specified.AdminID)

	blocked := User{AdminID: 40, Username: "blocked-owner", Nickname: "blocked-owner", Password: "p", Status: "enable"}
	require.Error(t, f.root.Add(ctx, &blocked))

	transfer := f.addUser(t, ctx, 20, "transfer")
	require.NoError(t, f.db.Create(&MoneyLog{UserID: transfer.ID, AdminID: 20, Money: 1.00}).Error)
	transfer.AdminID = 30
	require.NoError(t, f.root.Edit(ctx, &transfer, ""))
	var moneyOwner int32
	require.NoError(t, f.db.Table(f.cfg.Database.Prefix+"user_money_log").Where("user_id = ?", transfer.ID).Pluck("admin_id", &moneyOwner).Error)
	require.Equal(t, int32(30), moneyOwner)

	mismatch := f.addUser(t, ctx, 20, "mismatch")
	require.NoError(t, f.db.Create(&MoneyLog{UserID: mismatch.ID, AdminID: 30, Money: 1.00}).Error)
	mismatch.AdminID = 30
	require.Error(t, f.root.Edit(ctx, &mismatch, ""))
	var unchanged int32
	require.NoError(t, f.db.Table(f.cfg.Database.Prefix+"user").Where("id = ?", mismatch.ID).Pluck("admin_id", &unchanged).Error)
	require.Equal(t, int32(20), unchanged)
	require.NoError(t, f.db.Model(&Admin{}).Where("id = ?", 30).Update("status", "disable").Error)
	disabled := User{AdminID: 30, Username: "disabled-owner", Nickname: "disabled-owner", Password: "p", Status: "enable"}
	require.Error(t, f.root.Add(ctx, &disabled))

	// An unchanged disabled owner must not block ordinary profile edits.
	keep := f.addUser(t, ctx, 30, "keep-disabled-owner")
	keep.Nickname = "keep-disabled-owner-renamed"
	require.NoError(t, f.root.Edit(ctx, &keep, ""))
}

func TestUserOwnerAssignmentInvalidTargetStates(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)

	// admin_id=0 means "assign to the actor" (covered by
	// TestUserOwnerAssignmentAndLogTransfer); only negative owners are invalid.
	for _, adminID := range []int32{-1} {
		invalid := User{AdminID: adminID, Username: "invalid-owner", Nickname: "invalid-owner", Password: "p", Status: "enable"}
		require.Error(t, f.root.Add(ctx, &invalid))
	}

	current := f.addUser(t, ctx, 20, "keep-owner")
	current.Nickname = "keep-owner-renamed"
	require.NoError(t, f.root.Edit(ctx, &current, ""))
	var owner int32
	require.NoError(t, f.db.Table(f.cfg.Database.Prefix+"user").Where("id = ?", current.ID).Pluck("admin_id", &owner).Error)
	require.Equal(t, int32(20), owner)
}

func TestUserEditAndAddRollbackWithActiveRequestTransaction(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	u := f.addUser(t, ctx, 20, "atomic")
	tx := f.db.Begin()
	bound := requesttx.Bind(context.Background(), tx)
	require.NoError(t, requesttx.Transaction(bound, func(db *gorm.DB) error {
		return db.Model(&User{}).Where("id = ?", u.ID).Updates(map[string]any{"nickname": "changed", "password": "temporary"}).Error
	}))
	require.NoError(t, tx.Rollback().Error)
	var got User
	require.NoError(t, f.db.Session(&gorm.Session{NewDB: true}).First(&got, u.ID).Error)
	require.Equal(t, "atomic", got.Nickname)
	copy := u
	copy.Nickname = "changed"
	require.NoError(t, f.root.Edit(ctx, &copy, "new-password"))
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, "changed", got.Nickname)
	other := f.addUser(t, ctx, 20, "existing")
	copy.Username = other.Username
	require.Error(t, f.root.Edit(ctx, &copy, "rollback-password"))
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, "changed", got.Nickname)
}

func TestUserConcurrentAddSameUsernameReturnsOneFriendlyDuplicateError(t *testing.T) {
	f := newScopeFixture(t)
	const username = "concurrent-username"
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := scopeCtx(t, 20, false)
			candidate := User{AdminID: 20, Username: username, Nickname: username, Password: "p", Status: "enable"}
			results <- f.root.Add(ctx, &candidate)
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		require.Contains(t, strings.ToLower(err.Error()), "username")
		require.Contains(t, strings.ToLower(err.Error()), "exist")
	}
	require.Equal(t, 1, successes)
}
