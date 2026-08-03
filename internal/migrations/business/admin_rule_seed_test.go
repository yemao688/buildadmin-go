package business

import (
	"testing"

	"buildadmin-go/internal/conf"

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

// F6: 判重必须按 pid+name+type——同名不同父的规则不得相互吞并。
func TestSeedAdminRuleDedupeByPidNameType(t *testing.T) {
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

	// 两个不同父级下同名的 "ops" 目录，以及一个同名 menu 与 menu_dir
	seed := AdminRuleSeed{Type: "menu_dir", Title: "ops", Name: "ops", Path: "ops", Children: []AdminRuleSeed{
		{Type: "menu", Title: "item", Name: "ops/item", Path: "ops/item"},
	}}
	if err := SeedAdminRule(db, config, seed); err != nil {
		t.Fatal(err)
	}
	second := AdminRuleSeed{Pid: 42, Type: "menu_dir", Title: "ops-2", Name: "ops", Path: "ops-2"}
	if err := SeedAdminRule(db, config, second); err != nil {
		t.Fatal(err)
	}
	// 同名但不同 type：根下 menu 与 menu_dir 并存
	menu := AdminRuleSeed{Pid: 0, Type: "menu", Title: "ops menu", Name: "ops", Path: "ops"}
	if err := SeedAdminRule(db, config, menu); err != nil {
		t.Fatal(err)
	}
	// 幂等：再跑一遍不新增
	if err := SeedAdminRule(db, config, seed); err != nil {
		t.Fatal(err)
	}
	if err := SeedAdminRule(db, config, second); err != nil {
		t.Fatal(err)
	}

	var rows []struct {
		ID    int32
		Pid   int32
		Type  string
		Name  string
		Title string
	}
	if err := db.Table("business_admin_rule").Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	// 期望：根 ops menu_dir、根 ops menu、pid=42 的 ops menu_dir、根 ops 的 item
	if len(rows) != 4 {
		t.Fatalf("rows=%d, want 4: %+v", len(rows), rows)
	}
	byPidType := map[[2]any]bool{}
	for _, row := range rows {
		key := [2]any{row.Pid, row.Type}
		if byPidType[key] {
			t.Fatalf("duplicate pid+type pair %v: %+v", key, rows)
		}
		byPidType[key] = true
	}
	var under42 int64
	if err := db.Table("business_admin_rule").Where("pid=42 AND name='ops'").Count(&under42).Error; err != nil {
		t.Fatal(err)
	}
	if under42 != 1 {
		t.Fatalf("pid=42 ops count=%d, want 1", under42)
	}
}
