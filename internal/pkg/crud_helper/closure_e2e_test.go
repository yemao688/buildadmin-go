package crud_helper

import (
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/pkg/data_scope"
)

func TestGeneratedCRUDClosureMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	dsn := testutil.MySQLDSN(cfg.MysqlTest, cfg.Database.Database)

	tmp := t.TempDir()
	autoRepo, autoEntity := renderE2EModel(t, crudmodel.Table{
		Name: "scopeitems", ModelFile: "internal/model/scopeitems.go", ControllerFile: "internal/admin/handler/scopeitems.go",
		FormFields: []string{"name", "admin_id"}, DataScope: nil,
	}, []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "admin_id", Type: "int", DesignType: "number"},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}, nil, compileDemoStruct("Scopeitems", "admin_id", "AdminID", "admin_id"))
	globalRepo, globalEntity := renderE2EModel(t, crudmodel.Table{
		Name: "banner", ModelFile: "internal/model/banner.go", ControllerFile: "internal/admin/handler/banner.go",
		FormFields: []string{"name"}, DataScope: &data_scope.Config{Mode: data_scope.ModeNone},
	}, []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}, &data_scope.Config{Mode: data_scope.ModeNone}, compileDemoStruct("Banner", "", "", ""))

	writeE2EFixture(t, tmp, autoRepo, autoEntity, globalRepo, globalEntity, closureE2EChildTest, "", "")
	run := exec.Command("go", "test", "./internal/admin/repository", "-run", "TestGeneratedClosureBehavior", "-count=1", "-v")
	run.Dir = tmp
	run.Env = append(os.Environ(), "GO_BUILD_ADMIN_TEST_CHILD_DSN="+dsn)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Logf("generated E2E output:\n%s", out)
	}
	require.NoError(t, err)
}

// TestGeneratedClosureMySQLReassignable 验证 reassignable 生成产物的运行时语义：
// 受限操作者 Add 强制归属、Edit 只能改到自己麾下；超管 Add 可指定归属、Edit 任意。
func TestGeneratedClosureMySQLReassignable(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	dsn := testutil.MySQLDSN(cfg.MysqlTest, cfg.Database.Database)

	tmp := t.TempDir()
	dsCfg := &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}
	scopeStruct := compileDemoStruct("Scopeitems", "admin_id", "AdminID", "admin_id")
	autoRepo, autoEntity := renderE2EModel(t, crudmodel.Table{
		Name: "scopeitems", ModelFile: "internal/model/scopeitems.go", ControllerFile: "internal/admin/handler/scopeitems.go",
		FormFields: []string{"name", "admin_id"}, DataScope: dsCfg,
	}, []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "admin_id", Type: "int", DesignType: "number"},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}, dsCfg, scopeStruct)
	globalRepo, globalEntity := renderE2EModel(t, crudmodel.Table{
		Name: "banner", ModelFile: "internal/model/banner.go", ControllerFile: "internal/admin/handler/banner.go",
		FormFields: []string{"name"}, DataScope: &data_scope.Config{Mode: data_scope.ModeNone},
	}, []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}, &data_scope.Config{Mode: data_scope.ModeNone}, compileDemoStruct("Banner", "", "", ""))
	handlerCode, dtoCode := renderE2EHandler(t, crudmodel.Table{
		Name: "scopeitems", ModelFile: "internal/model/scopeitems.go", ControllerFile: "internal/admin/handler/scopeitems.go",
		FormFields: []string{"name", "admin_id"}, DataScope: dsCfg,
	}, []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "admin_id", Type: "int", DesignType: "number"},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}, dsCfg, scopeStruct)

	writeE2EFixture(t, tmp, autoRepo, autoEntity, globalRepo, globalEntity, closureReassignableE2EChildTest, handlerCode, dtoCode)
	run := exec.Command("go", "test", "./internal/admin/repository", "-run", "TestGeneratedClosureReassignable", "-count=1", "-v")
	run.Dir = tmp
	run.Env = append(os.Environ(), "GO_BUILD_ADMIN_TEST_CHILD_DSN="+dsn)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Logf("generated E2E output:\n%s", out)
	}
	require.NoError(t, err)
}

// TestGeneratedClosureMySQLCascade 验证级联归属生成产物的运行时语义：
// 子表（inheritFrom）Add 继承主实体当前归属并校验 actor 麾下；主表
// （reassignable + cascadeOwners）Edit 改归属时同事务级联回首子表 admin_id。
func TestGeneratedClosureMySQLCascade(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	dsn := testutil.MySQLDSN(cfg.MysqlTest, cfg.Database.Database)

	tmp := t.TempDir()
	userCfg := &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}
	userStruct := compileDemoStruct("User", "admin_id", "AdminID", "admin_id")
	userRepo, userEntity := renderE2EModel(t, crudmodel.Table{
		Name: "user", ModelFile: "internal/model/user.go", ControllerFile: "internal/admin/handler/user.go",
		FormFields: []string{"name", "admin_id"}, DataScope: userCfg,
	}, []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "admin_id", Type: "int", DesignType: "number"},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}, userCfg, userStruct)
	// 方案 A：子表生成时生成器自动往主实体 repo 锚点块注入注册条目，此处
	// 模拟 applyCascadeAnchor 的产物（child/child2 均以 user_id 关联）。
	userRepo = injectCascadeAnchorEntriesForTest(t, userRepo, "child", "child2")

	childCfg := &data_scope.Config{Mode: data_scope.ModeAuto, InheritFrom: &data_scope.InheritRef{Table: "user", ByColumn: "user_id"}}
	childStruct := func(className string) string {
		return `import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"buildadmin-go/internal/conf"
)
// ` + className + ` demo table
type ` + className + ` struct {
	ID int32 ` + "`gorm:\"column:id;primaryKey;autoIncrement:true\" json:\"id\"`" + `
	UserID int32 ` + "`gorm:\"column:user_id\" json:\"user_id\"`" + `
	AdminID int32 ` + "`gorm:\"column:admin_id\" json:\"admin_id\"`" + `
	Name string ` + "`gorm:\"column:name\" json:\"name\"`" + `
	CreateTime int64 ` + "`gorm:\"column:create_time\" json:\"create_time\"`" + `
	UpdateTime int64 ` + "`gorm:\"column:update_time\" json:\"update_time\"`" + `
}
`
	}
	childFields := []crudmodel.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "user_id", Type: "int", DesignType: "number"},
		{Name: "admin_id", Type: "int", DesignType: "number"},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}
	childRepo, childEntity := renderE2EModel(t, crudmodel.Table{
		Name: "child", ModelFile: "internal/model/child.go", ControllerFile: "internal/admin/handler/child.go",
		FormFields: []string{"name"}, DataScope: childCfg,
	}, childFields, childCfg, childStruct("Child"))
	child2Repo, child2Entity := renderE2EModel(t, crudmodel.Table{
		Name: "child2", ModelFile: "internal/model/child2.go", ControllerFile: "internal/admin/handler/child2.go",
		FormFields: []string{"name"}, DataScope: childCfg,
	}, childFields, childCfg, childStruct("Child2"))

	writeCascadeE2EFixture(t, tmp, userRepo, userEntity, childRepo, childEntity, child2Repo, child2Entity, closureCascadeE2EChildTest)
	run := exec.Command("go", "test", "./internal/admin/repository", "-run", "TestGeneratedCascadeBehavior", "-count=1", "-v")
	run.Dir = tmp
	run.Env = append(os.Environ(), "GO_BUILD_ADMIN_TEST_CHILD_DSN="+dsn)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Logf("generated E2E output:\n%s", out)
	}
	require.NoError(t, err)
}

func renderE2EModel(t *testing.T, table crudmodel.Table, fields []crudmodel.Field, cfg *data_scope.Config, structContent string) (string, string) {
	t.Helper()
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	modelData, _, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.NoError(t, err)
	modelData.Pk = "id"
	modelData.StructTemp = structContent
	repoCode, err := renderModel(modelData)
	require.NoError(t, err)
	entityCode, err := renderEntity(modelData)
	require.NoError(t, err)
	return repoCode, entityCode
}

// renderE2EHandler 渲染生成 handler 与其请求 DTO（生产同路径 buildParamStruct），
// 供 e2e fixture 中 handler 级运行时测试使用。
func renderE2EHandler(t *testing.T, table crudmodel.Table, fields []crudmodel.Field, cfg *data_scope.Config, structContent string) (string, string) {
	t.Helper()
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	_, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.NoError(t, err)
	handlerCode, err := renderHandler(handlerData)
	require.NoError(t, err)
	dtoCode, err := renderDTO(buildParamStruct(structContent, handlerData))
	require.NoError(t, err)
	return handlerCode, dtoCode
}

// closureE2EChildTest 是既有（非 reassignable）e2e 场景的子进程测试。
const closureE2EChildTest = `package repository

import (
	"errors"
	"fmt"
	"os"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/pkg/data_scope"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGeneratedClosureBehavior(t *testing.T) {
	dsn := os.Getenv("GO_BUILD_ADMIN_TEST_CHILD_DSN")
	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)
	prefix := fmt.Sprintf("ba_e2e_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{TablePrefix: prefix}
	resourceTable := prefix + "scopeitems"
	bannerTable := prefix + "banner"
	closureTable := prefix + "admin_closure"
	for _, table := range []string{resourceTable, bannerTable, closureTable} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table).Error)
		table := table
		t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS " + table).Error })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+closureTable+" (ancestor_id BIGINT NOT NULL, descendant_id BIGINT NOT NULL, PRIMARY KEY (ancestor_id, descendant_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+resourceTable+" (id INT PRIMARY KEY, admin_id INT NOT NULL, name VARCHAR(100) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0, update_time BIGINT NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+bannerTable+" (id INT PRIMARY KEY, name VARCHAR(100) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0, update_time BIGINT NOT NULL DEFAULT 0)").Error)
	for _, pair := range [][2]int{{1,1},{2,2},{3,3},{4,4},{2,4}} {
		require.NoError(t, db.Exec("INSERT INTO "+closureTable+" (ancestor_id, descendant_id) VALUES (?, ?)", pair[0], pair[1]).Error)
	}
	for _, row := range [][3]interface{}{{1,2,"B"},{2,3,"C"},{3,4,"D"}} {
		require.NoError(t, db.Exec("INSERT INTO "+resourceTable+" (id, admin_id, name) VALUES (?, ?, ?)", row[0], row[1], row[2]).Error)
	}

	cfg := &conf.Configuration{}
	cfg.Database.Prefix = prefix
	enforcer := data_scope.NewClosureEnforcer(cfg)
	repo := NewScopeitemsRepository(db, cfg, enforcer)
	t.Logf("generated table=%q expected=%q", repo.TableName, resourceTable)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, data_scope.SetActor(ctx, data_scope.Actor{AdminID: 2}))

	list, total, err := repo.List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	var aggregate struct{ Total int64 }
	require.NoError(t, repo.readScopedDB(ctx, repo.DBFor(ctx)).Table(repo.TableName).Select("COUNT(*) AS total").Scan(&aggregate).Error)
	require.Equal(t, total, aggregate.Total)
	_, err = repo.GetOne(ctx, 2)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, repo.Add(ctx, model.Scopeitems{ID: 10, AdminID: 3, Name: "forged"}))
	var owner int32
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 10").Scan(&owner).Error)
	require.Equal(t, int32(2), owner)

	require.NoError(t, repo.Edit(ctx, model.Scopeitems{ID: 1, AdminID: 3, Name: ""}))
	var edited model.Scopeitems
	require.NoError(t, db.Raw("SELECT id, admin_id, name FROM "+resourceTable+" WHERE id = 1").Scan(&edited).Error)
	require.Equal(t, int32(2), edited.AdminID)
	require.Equal(t, "", edited.Name)
	require.NoError(t, repo.Edit(ctx, model.Scopeitems{ID: 1, AdminID: 3, Name: ""}))
	require.ErrorIs(t, repo.Edit(ctx, model.Scopeitems{ID: 2, AdminID: 2, Name: "blocked"}), gorm.ErrRecordNotFound)

	require.NoError(t, repo.Del(ctx, []int32{1, 3, 3}))
	require.ErrorIs(t, repo.Del(ctx, []int32{2}), gorm.ErrRecordNotFound)
	require.NoError(t, db.Exec("INSERT INTO "+resourceTable+" (id, admin_id, name) VALUES (20, 2, 'B2'), (21, 3, 'C2')").Error)
	require.ErrorIs(t, repo.Del(ctx, []int32{20, 21}), gorm.ErrRecordNotFound)
	var remaining int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+resourceTable+" WHERE id IN (?, ?)", 20, 21).Scan(&remaining).Error)
	require.Equal(t, int64(2), remaining)

	super, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, data_scope.SetActor(super, data_scope.Actor{AdminID: 1, Unrestricted: true}))
	_, superTotal, err := repo.List(super)
	require.NoError(t, err)
	require.Equal(t, int64(4), superTotal)
	missing, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, _, err = repo.List(missing)
	require.Error(t, err)
	require.True(t, errors.Is(err, data_scope.ErrScopedAccessDenied) || errors.Is(err, data_scope.ErrInvalidActor))

	banner := NewBannerRepository(db, cfg, nil)
	require.NoError(t, banner.Add(missing, model.Banner{ID: 1, Name: "global"}))
	require.NoError(t, banner.Edit(missing, model.Banner{ID: 1, Name: ""}))
	require.NoError(t, banner.Del(missing, []int32{1}))
}
`

// closureReassignableE2EChildTest 是 reassignable 场景的子进程测试：受限操作者
// 只能把归属改到自己麾下（自己+后代），超管可任意指定（仅层级校验）；handler 级
// 恢复语义（不传/传 0 admin_id → 归属保持原值）。
// 注意：必须是外部测试包（repository_test）——测试内要 import handler 包，
// 而 handler 包会 import repository 包；内部测试包会构成 import cycle。
const closureReassignableE2EChildTest = `package repository_test

import (
	"fmt"
	"os"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	handler "buildadmin-go/internal/admin/handler"
	repository "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/pkg/data_scope"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGeneratedClosureReassignable(t *testing.T) {
	dsn := os.Getenv("GO_BUILD_ADMIN_TEST_CHILD_DSN")
	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)
	prefix := fmt.Sprintf("ba_e2e_re_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{TablePrefix: prefix}
	resourceTable := prefix + "scopeitems"
	adminTable := prefix + "admin"
	closureTable := prefix + "admin_closure"
	for _, table := range []string{resourceTable, adminTable, closureTable} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table).Error)
		table := table
		t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS " + table).Error })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+adminTable+" (id INT PRIMARY KEY, username VARCHAR(100) NOT NULL)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+closureTable+" (ancestor_id BIGINT NOT NULL, descendant_id BIGINT NOT NULL, depth BIGINT NOT NULL DEFAULT 0, PRIMARY KEY (ancestor_id, descendant_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+resourceTable+" (id INT PRIMARY KEY, admin_id INT NOT NULL, name VARCHAR(100) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0, update_time BIGINT NOT NULL DEFAULT 0)").Error)
	for _, admin := range []int{1, 2, 3, 4} {
		require.NoError(t, db.Exec("INSERT INTO "+adminTable+" (id, username) VALUES (?, ?)", admin, fmt.Sprintf("admin-%d", admin)).Error)
	}
	for _, pair := range [][2]int{{1,1},{2,2},{3,3},{4,4},{2,4}} {
		require.NoError(t, db.Exec("INSERT INTO "+closureTable+" (ancestor_id, descendant_id, depth) VALUES (?, ?, 0)", pair[0], pair[1]).Error)
	}
	for _, row := range [][3]interface{}{{1,2,"A"},{2,3,"B"},{3,4,"C"}} {
		require.NoError(t, db.Exec("INSERT INTO "+resourceTable+" (id, admin_id, name) VALUES (?, ?, ?)", row[0], row[1], row[2]).Error)
	}

	cfg := &conf.Configuration{}
	cfg.Database.Prefix = prefix
	enforcer := data_scope.NewClosureEnforcer(cfg)
	repo := repository.NewScopeitemsRepository(db, cfg, enforcer)

	restricted, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, data_scope.SetActor(restricted, data_scope.Actor{AdminID: 2}))

	// 受限 actor Add 传 AdminID=3 → 强制归属操作者 2
	require.NoError(t, repo.Add(restricted, model.Scopeitems{ID: 10, AdminID: 3, Name: "forged"}))
	var owner int32
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 10").Scan(&owner).Error)
	require.Equal(t, int32(2), owner)

	// 受限 actor Edit 行 1（原 admin_id=2）改 AdminID=4（麾下 {2,4}）→ 成功且落库 4
	require.NoError(t, repo.Edit(restricted, model.Scopeitems{ID: 1, AdminID: 4, Name: "A2"}))
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 1").Scan(&owner).Error)
	require.Equal(t, int32(4), owner)

	// 受限 actor Edit 改 AdminID=3（麾外）→ ErrScopedAccessDenied
	err = repo.Edit(restricted, model.Scopeitems{ID: 1, AdminID: 3, Name: "A3"})
	require.ErrorIs(t, err, data_scope.ErrScopedAccessDenied)

	// 受限 actor Edit 改回 AdminID=2（麾下，变更）→ 成功
	require.NoError(t, repo.Edit(restricted, model.Scopeitems{ID: 1, AdminID: 2, Name: "A4"}))
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 1").Scan(&owner).Error)
	require.Equal(t, int32(2), owner)

	// 受限 actor Edit 传与当前相同值 → 成功（未变更归属跳过校验）
	require.NoError(t, repo.Edit(restricted, model.Scopeitems{ID: 1, AdminID: 2, Name: "A4"}))

	// 受限 actor Edit 麾外行（admin_id=3，actor 2 无权编辑）→ gorm.ErrRecordNotFound
	err = repo.Edit(restricted, model.Scopeitems{ID: 2, AdminID: 2, Name: "blocked"})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	super, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, data_scope.SetActor(super, data_scope.Actor{AdminID: 1, Unrestricted: true}))

	// 超管 Add 传 AdminID=3 → 保留指定 3
	require.NoError(t, repo.Add(super, model.Scopeitems{ID: 11, AdminID: 3, Name: "s1"}))
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 11").Scan(&owner).Error)
	require.Equal(t, int32(3), owner)

	// 超管 Add 传 AdminID=999（不存在）→ 失败
	err = repo.Add(super, model.Scopeitems{ID: 12, AdminID: 999, Name: "s2"})
	require.ErrorIs(t, err, data_scope.ErrScopedAccessDenied)

	// 超管 Edit 行（当前 admin_id=2）改 AdminID=3（任意）→ 成功
	require.NoError(t, repo.Edit(super, model.Scopeitems{ID: 1, AdminID: 3, Name: "A5"}))
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 1").Scan(&owner).Error)
	require.Equal(t, int32(3), owner)

	// —— handler 级恢复语义：Edit 请求不传 admin_id / 传 0 → 归属保持原值 ——
	// 行 3 归属 admin_id=4（actor 2 麾下 {2,4}，可见）。
	hdl := handler.NewScopeitemsHandler(zap.NewNop(), repo)
	editRow := func(body string) {
		hc, _ := gin.CreateTestContext(httptest.NewRecorder())
		require.NoError(t, data_scope.SetActor(hc, data_scope.Actor{AdminID: 2}))
		hc.Request = httptest.NewRequest("POST", "/edit", strings.NewReader(body))
		hdl.Edit(hc)
	}

	// 请求体不传 admin_id → 归属保持 4
	editRow("{\"id\":3,\"name\":\"H1\"}")
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 3").Scan(&owner).Error)
	require.Equal(t, int32(4), owner)
	var rowName string
	require.NoError(t, db.Raw("SELECT name FROM "+resourceTable+" WHERE id = 3").Scan(&rowName).Error)
	require.Equal(t, "H1", rowName)

	// 请求体显式传 0 → 归属保持 4
	editRow("{\"id\":3,\"name\":\"H2\",\"admin_id\":0}")
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 3").Scan(&owner).Error)
	require.Equal(t, int32(4), owner)

	// 请求体显式传麾下 admin_id=2 → handler 透传真实重分配，归属变更为 2
	editRow("{\"id\":3,\"name\":\"H3\",\"admin_id\":2}")
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 3").Scan(&owner).Error)
	require.Equal(t, int32(2), owner)
}
`

// closureCascadeE2EChildTest 是级联归属场景的子进程测试（内部包 repository）：
// 主表 user（reassignable + cascadeOwners→child/child2）+ 子表 child/child2
// （inheritFrom user）。覆盖 Add 继承、麾外拒绝、超管继承、主表改归属事务内
// 级联回首、归属不变不动、幂等（已是最新值不误动）、父行不存在、父表脏归属
// （admin_id=0）fail-closed、多子表同时级联。
const closureCascadeE2EChildTest = `package repository

import (
	"fmt"
	"os"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/pkg/data_scope"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGeneratedCascadeBehavior(t *testing.T) {
	dsn := os.Getenv("GO_BUILD_ADMIN_TEST_CHILD_DSN")
	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)
	prefix := fmt.Sprintf("ba_e2e_cas_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{TablePrefix: prefix}
	userTable := prefix + "user"
	childTable := prefix + "child"
	child2Table := prefix + "child2"
	adminTable := prefix + "admin"
	closureTable := prefix + "admin_closure"
	for _, table := range []string{userTable, childTable, child2Table, adminTable, closureTable} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table).Error)
		table := table
		t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS " + table).Error })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+adminTable+" (id INT PRIMARY KEY, username VARCHAR(100) NOT NULL)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+closureTable+" (ancestor_id BIGINT NOT NULL, descendant_id BIGINT NOT NULL, depth BIGINT NOT NULL DEFAULT 0, PRIMARY KEY (ancestor_id, descendant_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+userTable+" (id INT PRIMARY KEY, admin_id INT NOT NULL, name VARCHAR(100) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0, update_time BIGINT NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+childTable+" (id INT PRIMARY KEY, user_id INT NOT NULL, admin_id INT NOT NULL, name VARCHAR(100) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0, update_time BIGINT NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+child2Table+" (id INT PRIMARY KEY, user_id INT NOT NULL, admin_id INT NOT NULL, name VARCHAR(100) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0, update_time BIGINT NOT NULL DEFAULT 0)").Error)
	for _, admin := range []int{1, 2, 3, 4} {
		require.NoError(t, db.Exec("INSERT INTO "+adminTable+" (id, username) VALUES (?, ?)", admin, fmt.Sprintf("admin-%d", admin)).Error)
	}
	for _, pair := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {4, 4}, {2, 4}} {
		require.NoError(t, db.Exec("INSERT INTO "+closureTable+" (ancestor_id, descendant_id, depth) VALUES (?, ?, 0)", pair[0], pair[1]).Error)
	}
	// 主表数据：user1 → admin2，user2 → admin3
	require.NoError(t, db.Exec("INSERT INTO "+userTable+" (id, admin_id, name) VALUES (1, 2, 'user1'), (2, 3, 'user2')").Error)
	// user2 的既有子行（归属 3），级联不得误动
	require.NoError(t, db.Exec("INSERT INTO "+childTable+" (id, user_id, admin_id, name) VALUES (20, 2, 3, 'u2c1')").Error)

	cfg := &conf.Configuration{}
	cfg.Database.Prefix = prefix
	enforcer := data_scope.NewClosureEnforcer(cfg)
	userRepo := NewUserRepository(db, cfg, enforcer)
	childRepo := NewChildRepository(db, cfg, enforcer)

	restricted, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, data_scope.SetActor(restricted, data_scope.Actor{AdminID: 2}))

	// 受限 actor(2) Add child（user_id=1，forged admin_id=3）→ 落库 2（继承）
	require.NoError(t, childRepo.Add(restricted, model.Child{ID: 10, UserID: 1, AdminID: 3, Name: "c1"}))
	var owner int32
	require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = 10").Scan(&owner).Error)
	require.Equal(t, int32(2), owner)

	// 受限 actor(2) Add child（user_id=2，主实体归属 3 在麾外）→ ErrScopedAccessDenied
	err = childRepo.Add(restricted, model.Child{ID: 11, UserID: 2, AdminID: 0, Name: "c2"})
	require.ErrorIs(t, err, data_scope.ErrScopedAccessDenied)
	var orphan int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+childTable+" WHERE id = 11").Scan(&orphan).Error)
	require.Equal(t, int64(0), orphan)

	super, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, data_scope.SetActor(super, data_scope.Actor{AdminID: 1, Unrestricted: true}))

	// 超管 Add child（user_id=1，forged admin_id=99）→ 落库 2（继承，非超管自己）
	require.NoError(t, childRepo.Add(super, model.Child{ID: 12, UserID: 1, AdminID: 99, Name: "c3"}))
	require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = 12").Scan(&owner).Error)
	require.Equal(t, int32(2), owner)

	// 主表 user1 Edit 归属 2→4（麾下 {2,4}）→ child 表 user_id=1 行同步为 4
	require.NoError(t, userRepo.Edit(restricted, model.User{ID: 1, AdminID: 4, Name: "user1"}))
	for _, id := range []int{10, 12} {
		require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = ?", id).Scan(&owner).Error)
		require.Equal(t, int32(4), owner, "child id=%d", id)
	}
	// user2 的 child（归属 3）不受级联影响
	require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = 20").Scan(&owner).Error)
	require.Equal(t, int32(3), owner)

	// 主表 user1 Edit 归属不变 → child 不动
	require.NoError(t, userRepo.Edit(restricted, model.User{ID: 1, AdminID: 4, Name: "user1"}))
	require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = 10").Scan(&owner).Error)
	require.Equal(t, int32(4), owner)

	// 幂等：child 已有行 admin_id=新值 → 不误动；其余行回首为 2
	require.NoError(t, db.Exec("INSERT INTO "+childTable+" (id, user_id, admin_id, name) VALUES (21, 1, 2, 'already')").Error)
	require.NoError(t, userRepo.Edit(restricted, model.User{ID: 1, AdminID: 2, Name: "user1"}))
	for _, id := range []int{10, 12, 21} {
		require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = ?", id).Scan(&owner).Error)
		require.Equal(t, int32(2), owner, "child id=%d", id)
	}
	require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = 20").Scan(&owner).Error)
	require.Equal(t, int32(3), owner)

	// —— 追加场景：父行不存在 / 父表脏归属 / 多子表级联 ——

	// 父行不存在：受限 actor Add child（user_id=999）→ gorm.ErrRecordNotFound（Take 自然上抛）
	err = childRepo.Add(restricted, model.Child{ID: 13, UserID: 999, AdminID: 0, Name: "c4"})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	var missing int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+childTable+" WHERE id = 13").Scan(&missing).Error)
	require.Equal(t, int64(0), missing)

	// 父表 admin_id=0 脏数据：user3 归属 0（闭包/管理表中不存在），Add 继承 0
	// → OwnerInScopeWithActor(ownerID<=0) fail-closed 拒绝
	require.NoError(t, db.Exec("INSERT INTO "+userTable+" (id, admin_id, name) VALUES (3, 0, 'user3-dirty')").Error)
	err = childRepo.Add(restricted, model.Child{ID: 14, UserID: 3, AdminID: 0, Name: "c5"})
	require.ErrorIs(t, err, data_scope.ErrScopedAccessDenied)
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+childTable+" WHERE id = 14").Scan(&missing).Error)
	require.Equal(t, int64(0), missing)

	// 多子表级联：child2 与 child 同 user_id 关联；user1 Edit 归属 2→4 → 两表都同步
	require.NoError(t, db.Exec("INSERT INTO "+child2Table+" (id, user_id, admin_id, name) VALUES (30, 1, 2, 'u1c2'), (31, 2, 3, 'u2c2')").Error)
	require.NoError(t, userRepo.Edit(restricted, model.User{ID: 1, AdminID: 4, Name: "user1"}))
	for _, id := range []int{10, 12, 21} {
		require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = ?", id).Scan(&owner).Error)
		require.Equal(t, int32(4), owner, "child id=%d", id)
	}
	require.NoError(t, db.Raw("SELECT admin_id FROM "+child2Table+" WHERE id = 30").Scan(&owner).Error)
	require.Equal(t, int32(4), owner, "child2 id=30")
	// user2 的两表子行均不受级联影响
	require.NoError(t, db.Raw("SELECT admin_id FROM "+childTable+" WHERE id = 20").Scan(&owner).Error)
	require.Equal(t, int32(3), owner)
	require.NoError(t, db.Raw("SELECT admin_id FROM "+child2Table+" WHERE id = 31").Scan(&owner).Error)
	require.Equal(t, int32(3), owner, "child2 id=31")
}
`

func writeCascadeE2EFixture(t *testing.T, root, userRepo, userEntity, childRepo, childEntity, child2Repo, child2Entity, childTest string) {
	t.Helper()
	goMod, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	require.NoError(t, err)
	goSum, err := os.ReadFile(filepath.Join(repoRoot(t), "go.sum"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), goMod, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.sum"), goSum, 0644))
	require.NoError(t, copyDir(filepath.Join(repoRoot(t), "internal", "pkg", "data_scope"), filepath.Join(root, "internal", "pkg", "data_scope")))

	files := map[string]string{
		"internal/conf/config.go": `package conf

type Configuration struct { Database Database }
type Database struct { Prefix string }
`,
		"internal/pkg/persistence/base.go": `package persistence

import (
	"context"

	"gorm.io/gorm"
)

type TableInfo struct {
	TableName        string
	Key              string
	QuickSearchField string
}

type BaseModel struct {
	TableName        string
	Key              string
	QuickSearchField string
	sqlDB            *gorm.DB
}

func NewBaseModel(tableName, key, quickSearchField string, sqlDB *gorm.DB) BaseModel {
	return BaseModel{TableName: tableName, Key: key, QuickSearchField: quickSearchField, sqlDB: sqlDB}
}

func (b *BaseModel) DBFor(context.Context) *gorm.DB { return b.sqlDB }
func (b *BaseModel) Transaction(_ context.Context, fn func(*gorm.DB) error) error { return b.sqlDB.Transaction(fn) }
func (b *BaseModel) TableInfo() TableInfo {
	return TableInfo{TableName: b.TableName, Key: b.Key, QuickSearchField: b.QuickSearchField}
}
`,
		"internal/admin/repository/query_builder.go": `package repository

import (
	"buildadmin-go/internal/pkg/persistence"
	"github.com/gin-gonic/gin"
)

type TableInfo = persistence.TableInfo

func QueryBuilder(ctx *gin.Context, table TableInfo, withTables []TableInfo) (string, []interface{}, string, int, int, error) {
	return "", nil, "id ASC", 100, 0, nil
}
`,
		"internal/model/user.go":        userEntity,
		"internal/model/child.go":       childEntity,
		"internal/model/child2.go":      child2Entity,
		"internal/admin/repository/user_gen.go":  userRepo,
		"internal/admin/repository/child_gen.go": childRepo,
		"internal/admin/repository/child2_gen.go": child2Repo,
		"internal/admin/repository/cascade_e2e_test.go": childTest,
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0644))
	}
}

func writeE2EFixture(t *testing.T, root, autoRepo, autoEntity, globalRepo, globalEntity, childTest, handlerCode, dtoCode string) {
	t.Helper()
	goMod, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	require.NoError(t, err)
	goSum, err := os.ReadFile(filepath.Join(repoRoot(t), "go.sum"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), goMod, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.sum"), goSum, 0644))
	require.NoError(t, copyDir(filepath.Join(repoRoot(t), "internal", "pkg", "data_scope"), filepath.Join(root, "internal", "pkg", "data_scope")))

	files := map[string]string{
		"internal/conf/config.go": `package conf

type Configuration struct { Database Database }
type Database struct { Prefix string }
`,
		// 记录层基座（唯一 BaseModel 契约的测试替身，对齐 internal/pkg/persistence）
		"internal/pkg/persistence/base.go": `package persistence

import (
	"context"

	"gorm.io/gorm"
)

type TableInfo struct {
	TableName        string
	Key              string
	QuickSearchField string
}

type BaseModel struct {
	TableName        string
	Key              string
	QuickSearchField string
	sqlDB            *gorm.DB
}

func NewBaseModel(tableName, key, quickSearchField string, sqlDB *gorm.DB) BaseModel {
	return BaseModel{TableName: tableName, Key: key, QuickSearchField: quickSearchField, sqlDB: sqlDB}
}

func (b *BaseModel) DBFor(context.Context) *gorm.DB { return b.sqlDB }
func (b *BaseModel) Transaction(_ context.Context, fn func(*gorm.DB) error) error { return b.sqlDB.Transaction(fn) }
func (b *BaseModel) TableInfo() TableInfo {
	return TableInfo{TableName: b.TableName, Key: b.Key, QuickSearchField: b.QuickSearchField}
}
`,
		// 根仓库包的 QueryBuilder 契约替身（生成代码同包调用）
		"internal/admin/repository/query_builder.go": `package repository

import (
	"buildadmin-go/internal/pkg/persistence"
	"github.com/gin-gonic/gin"
)

type TableInfo = persistence.TableInfo

func QueryBuilder(ctx *gin.Context, table TableInfo, withTables []TableInfo) (string, []interface{}, string, int, int, error) {
	return "", nil, "id ASC", 100, 0, nil
}
`,
		"internal/model/scopeitems.go": autoEntity,
		"internal/model/banner.go":     globalEntity,
		"internal/admin/repository/scopeitem_gen.go": autoRepo,
		"internal/admin/repository/banner_gen.go":     globalRepo,
		"internal/admin/repository/closure_e2e_test.go": childTest,
	}
	// handler 级运行时测试所需的生成 handler、DTO 与契约替身；既有（纯 repo）
	// fixture 传空串不写这些文件。
	if handlerCode != "" {
		files["internal/admin/handler/scopeitems_handler.go"] = handlerCode
		files["internal/admin/handler/base.go"] = `package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Base struct {
	currentM any
	log      *zap.Logger
}

func (b *Base) Select(ctx *gin.Context) (any, bool) { return nil, false }
func (b *Base) MaybePartialEdit(ctx *gin.Context, fields map[string]bool) bool { return false }
`
		files["internal/admin/dto/scopeitems.go"] = dtoCode
		files["internal/pkg/response/response.go"] = `package response

import "github.com/gin-gonic/gin"

type Response struct {
	Code int
	Data any
	Msg  string
	Time int64
}

func Success(c *gin.Context, data any)                  {}
func SuccessWithMessage(c *gin.Context, message string) {}
func FailByErr(c *gin.Context, err error)               {}
`
		files["internal/pkg/validator/validator.go"] = `package validator

type FlexInt32 int32
type FlexInt64 int64
type FlexFloat64 float64

type Ids struct {
	Ids interface{}
}

func GetError(v interface{}, err error) error { return err }
`
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0644))
	}
}
