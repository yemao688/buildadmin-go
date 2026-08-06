package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/common/money"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Admin = model.Admin
type AdminGroup = model.AdminGroup

// scopeFixture wires the repository scoped primitives and the graduated
// services against an isolated per-test table prefix.
type scopeFixture struct {
	db       *gorm.DB
	cfg      *conf.Configuration
	root     *adminmodel.UserRepository
	money    *adminmodel.UserMoneyLogRepository
	userSvc  *UserService
	moneySvc *UserMoneyLogService
	admins   map[int32]Admin
	users    map[int32]model.User
}

func newScopeFixture(t *testing.T) *scopeFixture {
	t.Helper()
	prefix := fmt.Sprintf("it_%d_", time.Now().UnixNano())
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = prefix
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	f := &scopeFixture{db: db, cfg: cfg, admins: map[int32]Admin{}, users: map[int32]model.User{}}
	require.NoError(t, db.AutoMigrate(&Admin{}, &AdminGroup{}, &model.User{}))
	require.NoError(t, db.Exec("CREATE TABLE `"+prefix+"user_money_log` (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, user_id INT NOT NULL, money DECIMAL(12,2) NOT NULL, `before` DECIMAL(12,2) NOT NULL, `after` DECIMAL(12,2) NOT NULL, `type` VARCHAR(30) NOT NULL DEFAULT 'system', memo VARCHAR(255) NOT NULL DEFAULT '', create_time BIGINT NOT NULL)").Error)
	// 级联注册表联动建表：UserService.Edit 变更归属会遍历 CascadeOwners()，
	// 业务 fork 追加子表注册（如 user_recharge/user_withdraw）后未建表会 1146。
	// 这里为除 user_money_log（上方已完整建表）外的每张注册表建最小表，列名
	// 取条目的 byColumn/ownerColumn；Cleanup 联动 DROP。
	for _, owner := range adminmodel.NewUserRepository(db, cfg, data_scope.NewClosureEnforcer(cfg)).CascadeOwners() {
		if owner.Table == "" || owner.Table == "user_money_log" {
			continue
		}
		byColumn := owner.ByColumn
		if byColumn == "" {
			byColumn = "user_id"
		}
		ownerColumn := owner.OwnerColumn
		if ownerColumn == "" {
			ownerColumn = "admin_id"
		}
		table := prefix + owner.Table
		require.NoError(t, db.Exec("CREATE TABLE `"+table+"` (id INT AUTO_INCREMENT PRIMARY KEY, `"+byColumn+"` INT NOT NULL, `"+ownerColumn+"` INT NOT NULL DEFAULT 0)").Error)
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS `" + table + "`") })
	}
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
	f.root = adminmodel.NewUserRepository(db, cfg, data_scope.NewClosureEnforcer(cfg))
	f.money = adminmodel.NewUserMoneyLogRepository(db, cfg, data_scope.NewClosureEnforcer(cfg))
	f.userSvc = NewUserService(f.root)
	f.moneySvc = NewUserMoneyLogService(f.money, money.NewUserBalanceService())
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

// scopeActor returns the actor attached by scopeCtx.
func scopeActor(t *testing.T, ctx *gin.Context) data_scope.Actor {
	t.Helper()
	a, ok := data_scope.ActorFromContext(ctx)
	require.True(t, ok)
	return a
}

func (f *scopeFixture) addUser(t *testing.T, ctx *gin.Context, adminID int32, name string) model.User {
	u := model.User{AdminID: adminID, Username: name, Nickname: name, Password: "p", Status: "enable"}
	require.NoError(t, f.db.Create(&u).Error)
	f.users[u.ID] = u
	return u
}

// userParams converts a model.User into the transport-free service shape.
func userParams(u model.User) UserParams {
	params := UserParams{
		Username: u.Username,
		Nickname: u.Nickname,
		Email:    u.Email,
		Mobile:   u.Mobile,
		Avatar:   u.Avatar,
		JoinIP:   u.JoinIP,
		JoinTime: u.JoinTime,
		Password: u.Password,
		Status:   u.Status,
	}
	if u.AdminID != 0 {
		params.AdminID = &u.AdminID
	}
	return params
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
		actor := scopeActor(t, c)
		u := model.User{ID: u40.ID, Username: "spoof", Nickname: "spoof", Status: "enable"}
		require.Error(t, f.userSvc.Edit(c.Request.Context(), u.ID, userParams(u), actor))
		require.Error(t, f.userSvc.UpdateStatus(c.Request.Context(), u40.ID, "disable", actor))
	}
	newUser := model.User{AdminID: 30, Username: "forged", Nickname: "forged", Password: "p", Status: "enable"}
	require.NoError(t, f.userSvc.Add(child.Request.Context(), userParams(newUser), scopeActor(t, child)))
	var newRow model.User
	require.NoError(t, f.db.Where("username = ?", newUser.Username).First(&newRow).Error)
	require.Equal(t, int32(30), newRow.AdminID)
	require.Error(t, f.userSvc.Del(leaf.Request.Context(), []int32{u20.ID}, scopeActor(t, leaf)))
	require.NoError(t, f.userSvc.Del(child.Request.Context(), []int32{u30.ID}, scopeActor(t, child)))
	require.NoError(t, f.userSvc.Del(other.Request.Context(), []int32{u40.ID}, scopeActor(t, other)))
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
	actor := scopeActor(t, ctx)
	defaultOwner := model.User{Username: "default-owner", Nickname: "default-owner", Password: "p", Status: "enable"}
	require.NoError(t, f.userSvc.Add(ctx.Request.Context(), userParams(defaultOwner), actor))
	var ownerRow model.User
	require.NoError(t, f.db.Where("username = ?", defaultOwner.Username).First(&ownerRow).Error)
	require.Equal(t, int32(20), ownerRow.AdminID)

	specified := model.User{AdminID: 30, Username: "specified-owner", Nickname: "specified-owner", Password: "p", Status: "enable"}
	require.NoError(t, f.userSvc.Add(ctx.Request.Context(), userParams(specified), actor))
	var specifiedRow model.User
	require.NoError(t, f.db.Where("username = ?", specified.Username).First(&specifiedRow).Error)
	require.Equal(t, int32(30), specifiedRow.AdminID)

	blocked := model.User{AdminID: 40, Username: "blocked-owner", Nickname: "blocked-owner", Password: "p", Status: "enable"}
	require.Error(t, f.userSvc.Add(ctx.Request.Context(), userParams(blocked), actor))

	transfer := f.addUser(t, ctx, 20, "transfer")
	require.NoError(t, f.db.Create(&model.MoneyLog{UserID: transfer.ID, AdminID: 20, Money: 1.00}).Error)
	transfer.AdminID = 30
	require.NoError(t, f.userSvc.Edit(ctx.Request.Context(), transfer.ID, userParams(transfer), actor))
	var moneyOwner int32
	require.NoError(t, f.db.Table(f.cfg.Database.Prefix+"user_money_log").Where("user_id = ?", transfer.ID).Pluck("admin_id", &moneyOwner).Error)
	require.Equal(t, int32(30), moneyOwner)

	mismatch := f.addUser(t, ctx, 20, "mismatch")
	require.NoError(t, f.db.Create(&model.MoneyLog{UserID: mismatch.ID, AdminID: 30, Money: 1.00}).Error)
	mismatch.AdminID = 30
	require.Error(t, f.userSvc.Edit(ctx.Request.Context(), mismatch.ID, userParams(mismatch), actor))
	var unchanged int32
	require.NoError(t, f.db.Table(f.cfg.Database.Prefix+"user").Where("id = ?", mismatch.ID).Pluck("admin_id", &unchanged).Error)
	require.Equal(t, int32(20), unchanged)
	require.NoError(t, f.db.Model(&Admin{}).Where("id = ?", 30).Update("status", "disable").Error)
	disabled := model.User{AdminID: 30, Username: "disabled-owner", Nickname: "disabled-owner", Password: "p", Status: "enable"}
	require.Error(t, f.userSvc.Add(ctx.Request.Context(), userParams(disabled), actor))

	// An unchanged disabled owner must not block ordinary profile edits.
	keep := f.addUser(t, ctx, 30, "keep-disabled-owner")
	keep.Nickname = "keep-disabled-owner-renamed"
	require.NoError(t, f.userSvc.Edit(ctx.Request.Context(), keep.ID, userParams(keep), actor))
}

func TestUserOwnerAssignmentInvalidTargetStates(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	actor := scopeActor(t, ctx)

	// admin_id=0 means "assign to the actor" (covered by
	// TestUserOwnerAssignmentAndLogTransfer); only negative owners are invalid.
	for _, adminID := range []int32{-1} {
		invalid := model.User{AdminID: adminID, Username: "invalid-owner", Nickname: "invalid-owner", Password: "p", Status: "enable"}
		require.Error(t, f.userSvc.Add(ctx.Request.Context(), userParams(invalid), actor))
	}

	current := f.addUser(t, ctx, 20, "keep-owner")
	current.Nickname = "keep-owner-renamed"
	require.NoError(t, f.userSvc.Edit(ctx.Request.Context(), current.ID, userParams(current), actor))
	var owner int32
	require.NoError(t, f.db.Table(f.cfg.Database.Prefix+"user").Where("id = ?", current.ID).Pluck("admin_id", &owner).Error)
	require.Equal(t, int32(20), owner)
}

func TestUserEditAndAddRollbackWithActiveRequestTransaction(t *testing.T) {
	f := newScopeFixture(t)
	ctx := scopeCtx(t, 20, false)
	actor := scopeActor(t, ctx)
	u := f.addUser(t, ctx, 20, "atomic")
	tx := f.db.Begin()
	bound := requesttx.Bind(context.Background(), tx)
	require.NoError(t, requesttx.Transaction(bound, func(db *gorm.DB) error {
		return db.Model(&model.User{}).Where("id = ?", u.ID).Updates(map[string]any{"nickname": "changed", "password": "temporary"}).Error
	}))
	require.NoError(t, tx.Rollback().Error)
	var got model.User
	require.NoError(t, f.db.Session(&gorm.Session{NewDB: true}).First(&got, u.ID).Error)
	require.Equal(t, "atomic", got.Nickname)
	copy := u
	copy.Nickname = "changed"
	require.NoError(t, f.userSvc.Edit(ctx.Request.Context(), copy.ID, userParams(copy), actor))
	require.NoError(t, f.db.First(&got, u.ID).Error)
	require.Equal(t, "changed", got.Nickname)
	other := f.addUser(t, ctx, 20, "existing")
	copy.Username = other.Username
	require.Error(t, f.userSvc.Edit(ctx.Request.Context(), copy.ID, userParams(copy), actor))
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
			candidate := model.User{AdminID: 20, Username: username, Nickname: username, Password: "p", Status: "enable"}
			results <- f.userSvc.Add(ctx.Request.Context(), userParams(candidate), scopeActor(t, ctx))
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
		// Both the service pre-check ("Account exist") and the transactional
		// duplicate guard ("username already exists") surface the friendly
		// duplicate wording; which one wins depends on the race.
		require.Contains(t, strings.ToLower(err.Error()), "exist")
	}
	require.Equal(t, 1, successes)
}
