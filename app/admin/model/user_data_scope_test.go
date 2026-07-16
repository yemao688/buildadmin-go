package model

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/app/pkg/requesttx"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type scopeFixture struct {
	db     *gorm.DB
	cfg    *conf.Configuration
	root   *UserModel
	money  *UserMoneyLogModel
	score  *UserScoreLogModel
	admins map[int32]Admin
	users  map[int32]User
}

func newScopeFixture(t *testing.T) *scopeFixture {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	prefix := fmt.Sprintf("it_%d_", time.Now().UnixNano())
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = prefix
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}, DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	f := &scopeFixture{db: db, cfg: cfg, admins: map[int32]Admin{}, users: map[int32]User{}}
	require.NoError(t, db.AutoMigrate(&Admin{}, &AdminGroup{}, &UserGroup{}, &User{}, &UserMoneyLog{}, &UserScoreLog{}))
	require.NoError(t, db.Exec("ALTER TABLE `"+prefix+"user` MODIFY `last_login_ip` VARCHAR(50) NOT NULL DEFAULT '', MODIFY `login_failure` INT NOT NULL DEFAULT 0").Error)
	closure := prefix + "admin_closure"
	require.NoError(t, db.Exec("CREATE TABLE `"+closure+"` (`ancestor_id` INT NOT NULL, `descendant_id` INT NOT NULL, `depth` INT NOT NULL, PRIMARY KEY (`ancestor_id`,`descendant_id`), KEY (`descendant_id`,`ancestor_id`)) ENGINE=InnoDB").Error)
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "user_score_log`")
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "user_money_log`")
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "user`")
		db.Exec("DROP TABLE IF EXISTS `" + prefix + "user_group`")
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
	f.money = NewUserMoneyLogModel(db, cfg, data_scope.NewClosureEnforcer(cfg))
	f.score = NewUserScoreLogModel(db, cfg, data_scope.NewClosureEnforcer(cfg))
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
	u := User{AdminID: adminID, Username: name, Nickname: name, Password: "p", Salt: "s", Status: "enable"}
	u.Birthday = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
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
	newUser := User{AdminID: 40, Username: "forged", Nickname: "forged", Password: "p", Salt: "s", Status: "enable"}
	newUser.Birthday = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, f.root.Add(child, &newUser))
	require.Equal(t, int32(20), newUser.AdminID)
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
