package crudgen

import (
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type menuSyncRuleIDs struct {
	wrongParent int32
	parent      int32
	menu        int32
}

func newMenuSyncDB(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:menu-sync?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture table with
	// sqlite-native DDL matching the runtime column shape.
	if err := db.Exec(`CREATE TABLE ba_admin_rule (
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
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db, cfg
}

func seedMenuSyncRules(t *testing.T, db *gorm.DB) menuSyncRuleIDs {
	t.Helper()
	rules := db.Table("ba_admin_rule")
	wrongParent := model.AdminRule{Pid: 99, Type: "menu_dir", Name: "ops", Title: "wrong parent", Path: "ops", Status: "1"}
	if err := rules.Create(&wrongParent).Error; err != nil {
		t.Fatal(err)
	}
	parent := model.AdminRule{Pid: 0, Type: "menu_dir", Name: "ops", Title: "Operations", Path: "ops", Status: "1"}
	if err := rules.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	menu := model.AdminRule{Pid: parent.ID, Type: "menu", Name: "ops/orders", Title: "Old title", Path: "old", MenuType: "link", Component: "old.vue", Icon: "custom-icon", Keepalive: 1, Weigh: 7, Status: "1"}
	if err := rules.Create(&menu).Error; err != nil {
		t.Fatal(err)
	}
	if err := rules.Create(&model.AdminRule{Pid: menu.ID, Type: "button", Name: "ops/orders/index", Title: "Old view", Status: "0"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := rules.Create(&model.AdminRule{Pid: menu.ID, Type: "button", Name: "ops/orders/custom", Title: "Custom", Status: "1"}).Error; err != nil {
		t.Fatal(err)
	}
	return menuSyncRuleIDs{wrongParent: wrongParent.ID, parent: parent.ID, menu: menu.ID}
}

// assertMenuMatchedUnderParent 先断言同名菜单没有被错挂到错误的父目录下，
// 再取父目录下的实际菜单行。
func assertMenuMatchedUnderParent(t *testing.T, db *gorm.DB, ids menuSyncRuleIDs) model.AdminRule {
	t.Helper()
	var got model.AdminRule
	if err := db.Table("ba_admin_rule").Where("pid=? AND name=? AND type=?", ids.wrongParent, "ops/orders", "menu").First(&got).Error; err == nil {
		t.Fatal("menu was incorrectly matched under the wrong parent")
	}
	if err := db.Table("ba_admin_rule").Where("pid=? AND name=? AND type=?", ids.parent, "ops/orders", "menu").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	return got
}

func assertMenuSyncFields(t *testing.T, got model.AdminRule) {
	t.Helper()
	if got.Title != "Orders" || got.Path != "ops/orders" || got.MenuType != "tab" || got.Component != "/src/views/backend/ops/orders/index.vue" || got.Weigh != 7 || got.Icon != "custom-icon" || got.Keepalive != 1 {
		t.Fatalf("menu update = %+v", got)
	}
}

func assertMenuCustomButtonPreserved(t *testing.T, db *gorm.DB) {
	t.Helper()
	var custom model.AdminRule
	if err := db.Table("ba_admin_rule").Where("name=?", "ops/orders/custom").First(&custom).Error; err != nil {
		t.Fatal(err)
	}
	if custom.Title != "Custom" {
		t.Fatal("custom button was removed or overwritten")
	}
}

func assertMenuSyncUnchanged(t *testing.T, report MenuSyncReport) {
	t.Helper()
	for _, item := range report.Results {
		if item.Action != MenuUnchanged {
			t.Fatalf("second sync result = %+v", report.Results)
		}
	}
}

func TestMenuSyncUpdatesOwnedFieldsAndPreservesDownstreamFields(t *testing.T) {
	t.Parallel()
	db, cfg := newMenuSyncDB(t)
	dir := ParseWebDirNameData("ops.orders", "views", "web/src/views/backend/ops/orders")
	if len(dir.Path) != 1 || dir.Path[0] != "ops" {
		t.Fatalf("web dir = %+v", dir)
	}
	ids := seedMenuSyncRules(t, db)

	report, err := SyncMenuWithOptionsAndRecord(db, cfg, dir, "Order management", &MenuOptions{Title: "Orders"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.CreatedIDs) != 4 {
		t.Fatalf("created IDs = %v, want four missing buttons", report.CreatedIDs)
	}
	got := assertMenuMatchedUnderParent(t, db, ids)
	assertMenuSyncFields(t, got)
	assertMenuCustomButtonPreserved(t, db)

	second, err := SyncMenuWithOptionsAndRecord(db, cfg, dir, "Order management", &MenuOptions{Title: "Orders"})
	if err != nil {
		t.Fatal(err)
	}
	assertMenuSyncUnchanged(t, second)
	weight := int32(11)
	third, err := SyncMenuWithOptionsAndRecord(db, cfg, dir, "Order management", &MenuOptions{Title: "Orders", Weigh: &weight})
	if err != nil {
		t.Fatal(err)
	}
	if len(third.Results) == 0 || third.Results[1].Action != MenuUpdated {
		t.Fatalf("explicit weigh did not update menu: %+v", third.Results)
	}
	if err := db.Table("ba_admin_rule").Where("id=?", got.ID).First(&got).Error; err != nil || got.Weigh != weight {
		t.Fatalf("weigh = %d, err=%v", got.Weigh, err)
	}
}
