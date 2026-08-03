package crud_helper

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/utils"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// F2: 历史 manifest 只校验根不校验模块归属——构造 manifest 可指向允许根下
// 任意文件。删除必须拒绝"根内但非本模块"的路径。
func TestDeleteRejectsManifestPathOutsideModuleOwnership(t *testing.T) {
	db, cfg, _ := newOwnershipFixtureOpt(t, true)
	err := DeleteFromSpec(db, cfg, "delete_fault")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ownership")
	// 非本模块文件必须原样保留
	unowned := filepath.Join(utils.RootPath(), "internal", "model", "other_table.go")
	_, statErr := os.Stat(unowned)
	require.NoError(t, statErr, "unowned file must not be touched")
}

// F3: 删除失败回滚必须恢复被递归删除的 menu_dir 父链（Pid 指向的父级
// 若被 Delete 一并删除，回滚后必须重建且 Pid 正确）。
func TestDeleteFailureRestoresNestedMenuDirAncestors(t *testing.T) {
	db, cfg, fixture := newOwnershipFixture(t)
	originalWire := runWire
	originalBuild := runProjectBuild
	t.Cleanup(func() {
		runWire = originalWire
		runProjectBuild = originalBuild
	})
	runWire = func() error { return errors.New("wire injected") }
	runProjectBuild = func() error { return nil }

	err := DeleteFromSpec(db, cfg, "delete_fault")
	require.Error(t, err)
	require.Contains(t, err.Error(), "stage=wire")

	for _, name := range []string{"delete", "delete/sub", fixture.menuName} {
		var row crudmodel.AdminRule
		require.NoError(t, db.Table(cfg.Database.Prefix+"admin_rule").Where("name=?", name).First(&row).Error, "menu ancestor %q must be restored", name)
	}
	// Pid 链必须完整：delete/sub.pid == delete.id；menu.pid == delete/sub.id
	var dir, sub, menu crudmodel.AdminRule
	require.NoError(t, db.Table(cfg.Database.Prefix+"admin_rule").Where("name=?", "delete").First(&dir).Error)
	require.NoError(t, db.Table(cfg.Database.Prefix+"admin_rule").Where("name=?", "delete/sub").First(&sub).Error)
	require.NoError(t, db.Table(cfg.Database.Prefix+"admin_rule").Where("name=?", fixture.menuName).First(&menu).Error)
	require.Equal(t, sub.Pid, dir.ID)
	require.Equal(t, menu.Pid, sub.ID)
}

// F4: 生成失败时，SyncMenu 对既有菜单行的更新必须回滚到原值（而不只是
// 删除本次新建的行）。
func TestMenuSnapshotRestoresUpdatedExistingRows(t *testing.T) {
	db, cfg, fixture := newOwnershipFixture(t)

	// F4 语义：生成前快照（原值）→ 生成 update（漂移）→ 失败恢复
	rows, err := snapshotMenuRules(db, cfg, fixture.menuName)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.NoError(t, db.Table(cfg.Database.Prefix+"admin_rule").
		Where("name=?", fixture.menuName).
		Updates(map[string]any{"title": "漂移标题", "component": "/src/views/backend/drift/index.vue"}).Error)
	require.NoError(t, restoreMenuRules(db, cfg, rows))

	var restored crudmodel.AdminRule
	require.NoError(t, db.Table(cfg.Database.Prefix+"admin_rule").Where("name=?", fixture.menuName).First(&restored).Error)
	require.Equal(t, fixture.menu.Title, restored.Title, "updated menu row must be restored to snapshot value")
	require.Equal(t, fixture.menu.Component, restored.Component)
}

// ownershipFixture：三层 menu_dir（delete → delete/sub → menu）+ crud_log
// manifest 记录"本模块文件 + 一个允许根下非本模块文件"。
type ownershipFixture struct {
	tableName string
	menuName  string
	menu      crudmodel.AdminRule
}

func newOwnershipFixture(t *testing.T) (*gorm.DB, *conf.Configuration, ownershipFixture) {
	return newOwnershipFixtureOpt(t, false)
}

func newOwnershipFixtureOpt(t *testing.T, includeUnowned bool) (*gorm.DB, *conf.Configuration, ownershipFixture) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "ba_", SingularTable: true}})
	require.NoError(t, err)
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = "ba_"
	require.NoError(t, db.Exec(`CREATE TABLE ba_admin_rule (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		pid INTEGER NOT NULL DEFAULT 0,
		"type" TEXT NOT NULL DEFAULT 'menu',
		title TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		path TEXT NOT NULL DEFAULT '',
		icon TEXT NOT NULL DEFAULT '',
		menu_type TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		component TEXT NOT NULL DEFAULT '',
		keepalive INTEGER NOT NULL DEFAULT 0,
		extend TEXT NOT NULL DEFAULT 'none',
		remark TEXT NOT NULL DEFAULT '',
		weigh INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT '1',
		update_time INTEGER,
		create_time INTEGER
	)`).Error)
	require.NoError(t, db.Exec("CREATE TABLE ba_crud_log (id INTEGER PRIMARY KEY AUTOINCREMENT, admin_id INTEGER NOT NULL, table_name TEXT NOT NULL, `table` BLOB, fields BLOB, status TEXT NOT NULL, comment TEXT, connection TEXT NOT NULL, sync INTEGER, create_time INTEGER)").Error)

	// 三层菜单：delete → delete/sub → delete/sub/fault
	dir := crudmodel.AdminRule{Pid: 0, Type: "menu_dir", Title: "delete", Name: "delete", Path: "delete", Status: "1"}
	require.NoError(t, db.Table("ba_admin_rule").Create(&dir).Error)
	sub := crudmodel.AdminRule{Pid: dir.ID, Type: "menu_dir", Title: "sub", Name: "delete/sub", Path: "delete/sub", Status: "1"}
	require.NoError(t, db.Table("ba_admin_rule").Create(&sub).Error)
	menuName := "delete/sub/fault"
	menu := crudmodel.AdminRule{Pid: sub.ID, Type: "menu", Title: "Delete fault", Name: menuName, Path: menuName, MenuType: "tab", Component: "/src/views/backend/delete/fault/index.vue", Status: "1"}
	require.NoError(t, db.Table("ba_admin_rule").Create(&menu).Error)
	button := crudmodel.AdminRule{Pid: menu.ID, Type: "button", Title: "查看", Name: menuName + "/index", Status: "1"}
	require.NoError(t, db.Table("ba_admin_rule").Create(&button).Error)

	// 本模块文件 + 非本模块文件（允许根 internal/model 下但基名不是本表）。
	// 目录与文件名用表名推导的历史形态（delete_fault → 目录 delete、实体 fault），
	// legacy 布局：模型文件在 internal/admin/model 下。
	modelDir := filepath.Join(utils.RootPath(), "internal", "admin", "model", "delete")
	handlerDir := filepath.Join(utils.RootPath(), "internal", "admin", "handler", "delete")
	require.NoError(t, os.MkdirAll(modelDir, 0755))
	require.NoError(t, os.MkdirAll(handlerDir, 0755))
	t.Cleanup(func() { _ = os.RemoveAll(modelDir) })
	t.Cleanup(func() { _ = os.RemoveAll(handlerDir) })
	modelProvider := filepath.Join(modelDir, "provider.go")
	handlerProvider := filepath.Join(handlerDir, "provider.go")
	require.NoError(t, os.WriteFile(modelProvider, []byte("package fixture\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewFaultModel,\n)\n"), 0644))
	require.NoError(t, os.WriteFile(handlerProvider, []byte("package fixture\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewFaultHandler,\n\tNewFaultRegistrar,\n)\n"), 0644))
	ownFile := filepath.Join(modelDir, "fault.go")
	require.NoError(t, os.WriteFile(ownFile, []byte("package fixture\n"), 0644))
	generated := []string{ownFile}
	if includeUnowned {
		unowned := filepath.Join(utils.RootPath(), "internal", "model", "other_table.go")
		require.NoError(t, os.WriteFile(unowned, []byte("package model\n"), 0644))
		t.Cleanup(func() { _ = os.Remove(unowned) })
		generated = append(generated, unowned)
	}

	fields := []crudmodel.Field{{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, Unsigned: true}}
	table := crudmodel.Table{
		Name:                "delete_fault",
		GenerateRelativePath: "delete_fault",
		ModelFile:           filepath.ToSlash(filepath.Join("internal", "admin", "model", "delete", "fault.go")),
		ControllerFile:      filepath.ToSlash(filepath.Join("internal", "admin", "handler", "delete", "fault.go")),
		WebViewsDir:         "web/src/views/backend/delete/fault",
		Manifest: &crudmodel.CRUDFileManifest{
			Generated: generated,
			Shared:    []string{modelProvider, handlerProvider, filepath.Join(utils.RootPath(), "cmd", "server", "wire.go"), filepath.Join(utils.RootPath(), "cmd", "server", "wire_gen.go")},
		},
	}
	tableJSON, err := json.Marshal(table)
	require.NoError(t, err)
	fieldsJSON, err := json.Marshal(fields)
	require.NoError(t, err)
	require.NoError(t, db.Exec("INSERT INTO ba_crud_log (admin_id, table_name, `table`, fields, status, connection, sync, create_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", 1, "delete_fault", tableJSON, fieldsJSON, "success", "mysql", 0, 1).Error)
	return db, cfg, ownershipFixture{tableName: "delete_fault", menuName: menuName, menu: menu}
}

// snapshotMenuRules 必须包含目标菜单的祖先链（F3 依赖）。
func TestSnapshotMenuRulesIncludesAncestors(t *testing.T) {
	db, cfg, fixture := newOwnershipFixture(t)
	rows, err := snapshotMenuRules(db, cfg, fixture.menuName)
	require.NoError(t, err)
	names := map[string]bool{}
	for _, row := range rows {
		names[row.Name] = true
	}
	require.True(t, names["delete"], "snapshot must include top-level menu_dir")
	require.True(t, names["delete/sub"], "snapshot must include intermediate menu_dir")
	require.True(t, names[fixture.menuName])
	require.True(t, names[fixture.menuName+"/index"])
}

// restoreMenuRules 对"存在但被更新"的行必须恢复原值（F4 依赖）。
func TestRestoreMenuRulesRestoresUpdatedRows(t *testing.T) {
	db, cfg, fixture := newOwnershipFixture(t)
	rows, err := snapshotMenuRules(db, cfg, fixture.menuName)
	require.NoError(t, err)
	snapshot := map[string]crudmodel.AdminRule{}
	for _, row := range rows {
		snapshot[row.Name] = row
	}

	// 模拟 Delete/生成流程对既有行的修改
	require.NoError(t, db.Table(cfg.Database.Prefix+"admin_rule").
		Where("name=?", fixture.menuName).
		Updates(map[string]any{"title": "改过的标题", "component": "/drift.vue"}).Error)

	require.NoError(t, restoreMenuRules(db, cfg, rows))

	var restored crudmodel.AdminRule
	require.NoError(t, db.Table(cfg.Database.Prefix+"admin_rule").Where("name=?", fixture.menuName).First(&restored).Error)
	require.True(t, reflect.DeepEqual(restored, snapshot[fixture.menuName]), "restored row must equal snapshot: got %+v want %+v", restored, snapshot[fixture.menuName])
}
