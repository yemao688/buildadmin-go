package crud_helper

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
)

func TestGeneratedCRUDClosureMySQL(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN not set; skipping generated CRUD ClosureEnforcer E2E")
	}

	tmp := t.TempDir()
	autoCode := renderE2EModel(t, model.Table{
		Name: "scopeitems", ModelFile: "app/admin/model/Scopeitems.go", ControllerFile: "app/admin/handler/Scopeitems.go",
		FormFields: []string{"name", "admin_id"}, DataScope: nil,
	}, []model.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "admin_id", Type: "int", DesignType: "number"},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}, nil, compileDemoStruct("Scopeitems", "admin_id", "AdminID", "admin_id"))
	globalCode := renderE2EModel(t, model.Table{
		Name: "banner", ModelFile: "app/admin/model/Banner.go", ControllerFile: "app/admin/handler/Banner.go",
		FormFields: []string{"name"}, DataScope: &data_scope.Config{Mode: data_scope.ModeNone},
	}, []model.Field{
		{Name: "id", Type: "int", DesignType: "pk", PrimaryKey: true, FormBuildExclude: true},
		{Name: "name", Type: "varchar", DesignType: "string"},
	}, &data_scope.Config{Mode: data_scope.ModeNone}, compileDemoStruct("Banner", "", "", ""))

	writeE2EFixture(t, tmp, autoCode, globalCode)
	run := exec.Command("go", "test", "./app/admin/model", "-run", "TestGeneratedClosureBehavior", "-count=1", "-v")
	run.Dir = tmp
	run.Env = append(os.Environ(), "BUILDADMIN_TEST_MYSQL_DSN="+dsn)
	out, err := run.CombinedOutput()
	if err != nil {
		t.Logf("generated E2E output:\n%s", out)
	}
	require.NoError(t, err)
}

func renderE2EModel(t *testing.T, table model.Table, fields []model.Field, cfg *data_scope.Config, structContent string) string {
	t.Helper()
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	modelData, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, cfg, getTableName, proveAll)
	require.NoError(t, err)
	modelData.Pk = "id"
	modelData.StructTemp = structContent
	code, err := renderModel(modelData)
	require.NoError(t, err)
	return code
}

func writeE2EFixture(t *testing.T, root, autoCode, globalCode string) {
	t.Helper()
	goMod, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	require.NoError(t, err)
	goSum, err := os.ReadFile(filepath.Join(repoRoot(t), "go.sum"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), goMod, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.sum"), goSum, 0644))
	require.NoError(t, copyDir(filepath.Join(repoRoot(t), "app", "pkg", "data_scope"), filepath.Join(root, "app", "pkg", "data_scope")))

	files := map[string]string{
		"conf/config.go": `package conf

type Configuration struct { Database Database }
type Database struct { Prefix string }
`,
		"app/admin/model/base.go": `package model

import (
	"context"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
) 

type BaseModel struct {
	TableName string
	Key string
	QuickSearchField string
	sqlDB *gorm.DB
}
func (b *BaseModel) TableInfo() map[string]string { return map[string]string{} }
func (b *BaseModel) DBFor(context.Context) *gorm.DB { return b.sqlDB }
func (b *BaseModel) Transaction(_ context.Context, fn func(*gorm.DB) error) error { return b.sqlDB.Transaction(fn) }
func QueryBuilder(*gin.Context, map[string]string, map[string]interface{}) (string, []interface{}, string, int, int, error) {
	return "", nil, "id ASC", 100, 0, nil
}
`,
		"app/admin/model/scope_item_gen.go": autoCode,
		"app/admin/model/banner_gen.go":     globalCode,
		"app/admin/model/closure_e2e_test.go": `package model

import (
	"errors"
	"fmt"
	"os"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGeneratedClosureBehavior(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
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
	model := NewScopeitemsModel(db, cfg, enforcer)
	t.Logf("generated table=%q expected=%q", model.TableName, resourceTable)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, data_scope.SetActor(ctx, data_scope.Actor{AdminID: 2}))

	list, total, err := model.List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	var aggregate struct{ Total int64 }
	require.NoError(t, model.scopedDB(ctx).Table(model.TableName).Select("COUNT(*) AS total").Scan(&aggregate).Error)
	require.Equal(t, total, aggregate.Total)
	_, err = model.GetOne(ctx, 2)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, model.Add(ctx, Scopeitems{ID: 10, AdminID: 3, Name: "forged"}))
	var owner int32
	require.NoError(t, db.Raw("SELECT admin_id FROM "+resourceTable+" WHERE id = 10").Scan(&owner).Error)
	require.Equal(t, int32(2), owner)

	require.NoError(t, model.Edit(ctx, Scopeitems{ID: 1, AdminID: 3, Name: ""}))
	var edited Scopeitems
	require.NoError(t, db.Raw("SELECT id, admin_id, name FROM "+resourceTable+" WHERE id = 1").Scan(&edited).Error)
	require.Equal(t, int32(2), edited.AdminID)
	require.Equal(t, "", edited.Name)
	require.ErrorIs(t, model.Edit(ctx, Scopeitems{ID: 2, AdminID: 2, Name: "blocked"}), gorm.ErrRecordNotFound)

	require.NoError(t, model.Del(ctx, []int32{1, 3, 3}))
	require.ErrorIs(t, model.Del(ctx, []int32{2}), gorm.ErrRecordNotFound)
	require.NoError(t, db.Exec("INSERT INTO "+resourceTable+" (id, admin_id, name) VALUES (20, 2, 'B2'), (21, 3, 'C2')").Error)
	require.ErrorIs(t, model.Del(ctx, []int32{20, 21}), gorm.ErrRecordNotFound)
	var remaining int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+resourceTable+" WHERE id IN (?, ?)", 20, 21).Scan(&remaining).Error)
	require.Equal(t, int64(2), remaining)

	super, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, data_scope.SetActor(super, data_scope.Actor{AdminID: 1, Unrestricted: true}))
	_, superTotal, err := model.List(super)
	require.NoError(t, err)
	require.Equal(t, int64(4), superTotal)
	missing, _ := gin.CreateTestContext(httptest.NewRecorder())
	_, _, err = model.List(missing)
	require.Error(t, err)
	require.True(t, errors.Is(err, data_scope.ErrScopedAccessDenied) || errors.Is(err, data_scope.ErrInvalidActor))

	banner := NewBannerModel(db, cfg, nil)
	require.NoError(t, banner.Add(missing, Banner{ID: 1, Name: "global"}))
	require.NoError(t, banner.Edit(missing, Banner{ID: 1, Name: ""}))
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
