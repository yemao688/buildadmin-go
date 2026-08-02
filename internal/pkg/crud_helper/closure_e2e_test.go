package crud_helper

import (
	crudmodel "buildadmin-go/internal/admin/model/crud"
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

	writeE2EFixture(t, tmp, autoRepo, autoEntity, globalRepo, globalEntity)
	run := exec.Command("go", "test", "./internal/admin/repository", "-run", "TestGeneratedClosureBehavior", "-count=1", "-v")
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
	modelData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.NoError(t, err)
	modelData.Pk = "id"
	modelData.StructTemp = structContent
	repoCode, err := renderModel(modelData)
	require.NoError(t, err)
	entityCode, err := renderEntity(modelData)
	require.NoError(t, err)
	return repoCode, entityCode
}

func writeE2EFixture(t *testing.T, root, autoRepo, autoEntity, globalRepo, globalEntity string) {
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
		"internal/admin/repository/closure_e2e_test.go": `package repository

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
`,
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(body), 0644))
	}
}
