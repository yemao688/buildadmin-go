package crud_helper

import (
	"encoding/json"
	"errors"
	crudmodel "go-build-admin/internal/admin/model/crud"
	"go-build-admin/internal/conf"
	model "go-build-admin/internal/model"
	"go-build-admin/internal/utils"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestDeleteWireFailureRestoresMenuWithSameID(t *testing.T) {
	db, cfg, fixture := newDeleteFailureFixture(t)
	originalWire := runWire
	originalBuild := runProjectBuild
	t.Cleanup(func() {
		runWire = originalWire
		runProjectBuild = originalBuild
	})
	runWire = func() error { return errors.New("wire injected") }
	runProjectBuild = func() error { return nil }

	err := DeleteFromSpec(db, cfg, fixture.tableName)
	if err == nil || !strings.Contains(err.Error(), "stage=wire") {
		t.Fatalf("wire failure stage = %v", err)
	}
	assertDeleteFixtureFilesRestored(t, fixture)
	var rows []model.AdminRule
	if err := db.Table(cfg.Database.Prefix+"admin_rule").Where("name=?", fixture.menuName).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != fixture.menu.ID || !reflect.DeepEqual(rows[0], fixture.menu) {
		t.Fatalf("menu was not restored byte-for-byte: got=%+v want=%+v", rows, fixture.menu)
	}
}

func TestDeleteMenuFailureRestoresFilesAndReportsStage(t *testing.T) {
	db, cfg, fixture := newDeleteFailureFixture(t)
	originalWire := runWire
	originalBuild := runProjectBuild
	t.Cleanup(func() {
		runWire = originalWire
		runProjectBuild = originalBuild
	})
	runWireCalled := false
	runWire = func() error {
		runWireCalled = true
		return nil
	}
	runProjectBuild = func() error { return nil }
	callbackName := "crud_helper_test_fail_menu_delete"
	if err := db.Callback().Delete().Before("gorm:delete").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == cfg.Database.Prefix+"admin_rule" {
			tx.AddError(errors.New("menu delete injected"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Delete().Remove(callbackName) })

	err := DeleteFromSpec(db, cfg, fixture.tableName)
	if err == nil || !strings.Contains(err.Error(), "stage=delete menu") {
		t.Fatalf("menu failure stage = %v", err)
	}
	if runWireCalled {
		t.Fatal("wire ran after menu deletion failed")
	}
	assertDeleteFixtureFilesRestored(t, fixture)
	var count int64
	if err := db.Table(cfg.Database.Prefix+"admin_rule").Where("id=?", fixture.menu.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("menu row was not retained after failed deletion: %d", count)
	}
}

type deleteFailureFixture struct {
	tableName string
	menuName  string
	menu      model.AdminRule
	generated []string
	shared    map[string][]byte
}

func newDeleteFailureFixture(t *testing.T) (*gorm.DB, *conf.Configuration, deleteFailureFixture) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "ba_", SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = "ba_"
	if err := db.Table("ba_admin_rule").AutoMigrate(&model.AdminRule{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE ba_crud_log (id INTEGER PRIMARY KEY AUTOINCREMENT, admin_id INTEGER NOT NULL, table_name TEXT NOT NULL, `table` BLOB, fields BLOB, status TEXT NOT NULL, comment TEXT, connection TEXT NOT NULL, sync INTEGER, create_time INTEGER)").Error; err != nil {
		t.Fatal(err)
	}

	tableName := "delete_fault"
	menuName := "delete/fault"
	dirName := "crud-helper-delete-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	modelDir := filepath.Join(utils.RootPath(), "internal", "admin", "model", dirName)
	handlerDir := filepath.Join(utils.RootPath(), "internal", "admin", "handler", dirName)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(handlerDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(modelDir)
		_ = os.RemoveAll(handlerDir)
	})

	modelProvider := filepath.Join(modelDir, "provider.go")
	handlerProvider := filepath.Join(handlerDir, "provider.go")
	generated := filepath.Join(modelDir, "deleteFault.go")
	if err := os.WriteFile(modelProvider, []byte("package fixture\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewDeleteFaultModel,\n)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handlerProvider, []byte("package fixture\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewDeleteFaultHandler,\n\tNewDeleteFaultRegistrar,\n)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(generated, []byte("package fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}

	routerPath := filepath.Join(utils.RootPath(), "internal", "router", "registrar_set.go")
	routerBefore, err := os.ReadFile(routerPath)
	if err != nil {
		t.Fatal(err)
	}
	routerAfter, err := addRegistrarProviderEntry(string(routerBefore), "DeleteFault", "internal/admin/handler")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routerPath, []byte(routerAfter), 0644); err != nil {
		t.Fatal(err)
	}
	shared := map[string][]byte{
		modelProvider:   []byte("package fixture\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewDeleteFaultModel,\n)\n"),
		handlerProvider: []byte("package fixture\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewDeleteFaultHandler,\n\tNewDeleteFaultRegistrar,\n)\n"),
		routerPath:      []byte(routerAfter),
	}
	t.Cleanup(func() { _ = os.WriteFile(routerPath, routerBefore, 0644) })

	menu := model.AdminRule{Pid: 0, Type: "menu", Title: "Delete fault", Name: menuName, Path: menuName, MenuType: "tab", Status: "1"}
	if err := db.Table("ba_admin_rule").Create(&menu).Error; err != nil {
		t.Fatal(err)
	}
	fields := []crudmodel.Field{{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, Unsigned: true}}
	table := crudmodel.Table{
		Name:           tableName,
		ModelFile:      filepath.ToSlash(filepath.Join("internal", "admin", "model", dirName, "deleteFault.go")),
		ControllerFile: filepath.ToSlash(filepath.Join("internal", "admin", "handler", dirName, "deleteFault.go")),
		WebViewsDir:    "web/src/views/backend/delete/fault",
		Manifest: &crudmodel.CRUDFileManifest{
			Generated: []string{generated},
			Shared:    []string{modelProvider, handlerProvider, routerPath},
		},
	}
	tableJSON, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ba_crud_log (admin_id, table_name, `table`, fields, status, connection, sync, create_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", 1, tableName, tableJSON, fieldsJSON, "success", "mysql", 0, 1).Error; err != nil {
		t.Fatal(err)
	}
	return db, cfg, deleteFailureFixture{
		tableName: tableName,
		menuName:  menuName,
		menu:      menu,
		generated: []string{generated},
		shared:    shared,
	}
}

func assertDeleteFixtureFilesRestored(t *testing.T, fixture deleteFailureFixture) {
	t.Helper()
	for _, path := range fixture.generated {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("generated file was not restored: %s: %v", path, err)
		}
		if string(content) != "package fixture\n" {
			t.Fatalf("generated file changed: %s", path)
		}
	}
	for path, want := range fixture.shared {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("shared file was not restored: %s: %v", path, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("shared file changed: %s", path)
		}
	}
}
