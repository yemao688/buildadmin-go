package auth

import (
	"testing"

	"go-build-admin/internal/conf"
	"go-build-admin/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGetMenusUsesAdministratorRules(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-auth-menus-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AdminRule{}, &model.AdminGroup{}, &model.AdminGroupAccess{}); err != nil {
		t.Fatal(err)
	}

	rules := []model.AdminRule{
		{ID: 1, Pid: 0, Type: "menu", Title: "Dashboard", Name: "dashboard", Path: "dashboard", MenuType: "tab", Status: "1", Weigh: 999},
		{ID: 2, Pid: 0, Type: "menu_dir", Title: "Authorization", Name: "auth", Path: "auth", Status: "1", Weigh: 100},
		{ID: 3, Pid: 2, Type: "menu", Title: "Rules", Name: "auth/rule", Path: "auth/rule", MenuType: "tab", Status: "1", Weigh: 99},
	}
	if err := db.Create(&rules).Error; err != nil {
		t.Fatal(err)
	}
	groups := []model.AdminGroup{
		{ID: 1, Name: "super", Rules: "*", Status: "1"},
		{ID: 2, Name: "dashboard", Rules: "1", Status: "1"},
	}
	if err := db.Create(&groups).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create([]model.AdminGroupAccess{{UID: 1, GroupID: 1}, {UID: 2, GroupID: 2}}).Error; err != nil {
		t.Fatal(err)
	}

	auth := NewAuthRepository(db, nil, &conf.Configuration{})
	superMenus, err := auth.GetMenus(nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !menusContain(superMenus, "auth/rule") {
		t.Fatal("super administrator should receive the auth/rule menu")
	}

	adminMenus, err := auth.GetMenus(nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if menusContain(adminMenus, "auth/rule") {
		t.Fatal("administrator without the auth/rule permission must not receive its menu")
	}
	if !menusContain(adminMenus, "dashboard") {
		t.Fatal("administrator should still receive the dashboard menu")
	}
}

func menusContain(rules []Rule, name string) bool {
	for _, rule := range rules {
		if rule.Name == name || menusContain(rule.Children, name) {
			return true
		}
	}
	return false
}
