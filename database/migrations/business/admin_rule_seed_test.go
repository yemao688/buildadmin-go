package business

import (
	"testing"

	"go-build-admin/conf"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedAdminRuleIsPrefixSafeIdempotentAndRecursive(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	config := &conf.Configuration{Database: conf.Database{Prefix: "business_"}}
	if err := db.Exec(`CREATE TABLE business_admin_rule (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		pid INTEGER NOT NULL DEFAULT 0,
		type TEXT NOT NULL DEFAULT 'menu',
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

	seed := AdminRuleSeed{
		Type:  "menu_dir",
		Title: "Operations",
		Name:  "ops",
		Path:  "ops",
		Icon:  "ops-icon",
		Weigh: 10,
		Children: []AdminRuleSeed{
			{
				Pid:       999,
				Type:      "menu",
				Title:     "Orders",
				Name:      "ops/orders",
				Path:      "ops/orders",
				MenuType:  "tab",
				Component: "/src/views/backend/ops/orders/index.vue",
				Children: []AdminRuleSeed{
					{Type: "button", Title: "View", Name: "ops/orders/index"},
				},
			},
		},
	}

	if err := SeedAdminRule(db, config, seed); err != nil {
		t.Fatal(err)
	}
	if err := SeedAdminRule(db, config, seed); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := db.Table("business_admin_rule").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("admin_rule count=%d, want 3", count)
	}
	var rows []struct {
		ID        int32
		Pid       int32
		Name      string
		Component string
	}
	if err := db.Table("business_admin_rule").Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows[0].Pid != 0 || rows[0].Name != "ops" || rows[0].Component != "" {
		t.Fatalf("root rule=%+v", rows[0])
	}
	if rows[1].Pid != rows[0].ID || rows[1].Name != "ops/orders" || rows[1].Component != "/src/views/backend/ops/orders/index.vue" {
		t.Fatalf("menu rule=%+v, parent=%d", rows[1], rows[0].ID)
	}
	if rows[2].Pid != rows[1].ID || rows[2].Name != "ops/orders/index" {
		t.Fatalf("button rule=%+v, parent=%d", rows[2], rows[1].ID)
	}
}

func TestSeedAdminRuleRejectsInvalidPrefixAndName(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	if err := SeedAdminRule(db, &conf.Configuration{Database: conf.Database{Prefix: "bad`_"}}, AdminRuleSeed{Name: "ops"}); err == nil {
		t.Fatal("invalid prefix was accepted")
	}
	if err := SeedAdminRule(db, &conf.Configuration{}, AdminRuleSeed{}); err == nil {
		t.Fatal("empty rule name was accepted")
	}
}
