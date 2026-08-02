package migrations

import (
	"fmt"
	"go-build-admin/internal/pkg/testutil"
	"go-build-admin/internal/conf"
	"go-build-admin/internal/database/migrations/internal/core"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func getDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, _ := testutil.OpenMySQL(t)
	db.Config.NamingStrategy = schema.NamingStrategy{
		SingularTable: true,
		TablePrefix:   "go_", // 表前缀
	}
	return db
}

func TestInstall(t *testing.T) {
	db := getDB(t)
	// 本测试使用固定 go_ 前缀且不随运行变化：前一次运行的遗留表会让安装重入失败
	// （重播种子会把 owner 置回 0 而 framework 迁移已被账本跳过），每次先清出干净起点。
	var leftovers []string
	if err := db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE 'go\\_%'").Scan(&leftovers).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range leftovers {
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
	err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(core.CoreModels()...)
	fmt.Println("生成数据表:", err)

	install := NewInstall(db)
	if err := install.InsertData(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"security_data_recycle", "security_sensitive_data"} {
		var ownerColumns int64
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name IN ('admin_id','owner_column')", "go_"+table).Scan(&ownerColumns).Error; err != nil {
			t.Fatal(err)
		}
		if ownerColumns != 0 {
			t.Fatalf("security seed table %s still has admin_id", table)
		}
	}
	seedConfig := &conf.Configuration{Database: conf.Database{Prefix: "go_"}}
	if err := MarkSeedPending(db, seedConfig); err != nil {
		t.Fatal(err)
	}
	if _, err := RunOfficialMigrations(db, seedConfig, OfficialMigrations()); err != nil {
		t.Fatal(err)
	}
	if err := RunOfficialFreshSeed(db, seedConfig); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapFrameworkLedger(db, seedConfig); err != nil {
		t.Fatal(err)
	}
	if _, err := RunFrameworkMigrations(db, seedConfig, OfficialMigrations(), FrameworkMigrations()); err != nil {
		t.Fatal(err)
	}
	var ruleCount int64
	if err := db.Table("go_admin_rule").Count(&ruleCount).Error; err != nil {
		t.Fatal(err)
	}
	if ruleCount != 96 {
		t.Fatalf("admin rule count=%d, want 96", ruleCount)
	}
	var rules []struct {
		ID    int32
		Pid   int32
		Type  string
		Title string
		Name  string
	}
	if err := db.Table("go_admin_rule").Where("id IN ?", []int{109, 110}).Order("id").Find(&rules).Error; err != nil {
		t.Fatal(err)
	}
	wantRules := []struct {
		ID    int32
		Pid   int32
		Type  string
		Title string
		Name  string
	}{
		{ID: 109, Pid: 19, Type: "button", Title: "删除", Name: "auth/adminLog/del"},
		{ID: 110, Pid: 45, Type: "button", Title: "发送测试邮件", Name: "routine/config/sendtestmail"},
	}
	if len(rules) != len(wantRules) {
		t.Fatalf("seeded admin rule count for new rules=%d, want %d", len(rules), len(wantRules))
	}
	for i := range wantRules {
		if rules[i] != wantRules[i] {
			t.Fatalf("seeded admin rule %d=%+v, want %+v", i, rules[i], wantRules[i])
		}
	}
	var languages []struct {
		Lan    string
		Name   string
		Remark string
		Status int8
		Weigh  int32
	}
	if err := db.Table("go_country_language").Order("weigh DESC, id ASC").Find(&languages).Error; err != nil {
		t.Fatal(err)
	}
	wantLanguages := []struct {
		Lan    string
		Name   string
		Remark string
		Status int8
		Weigh  int32
	}{
		{Lan: "zh-cn", Name: "简体中文", Remark: "简体中文", Status: 1, Weigh: 2},
		{Lan: "en", Name: "English", Remark: "English", Status: 1, Weigh: 1},
	}
	if len(languages) != len(wantLanguages) {
		t.Fatalf("country language count=%d, want %d", len(languages), len(wantLanguages))
	}
	for i := range wantLanguages {
		if languages[i] != wantLanguages[i] {
			t.Fatalf("country language %d=%+v, want %+v", i, languages[i], wantLanguages[i])
		}
	}
	for _, table := range []string{"security_data_recycle", "security_sensitive_data"} {
		var seedCount int64
		if err := db.Raw("SELECT COUNT(*) FROM `go_" + table + "` WHERE data_table='user'").Scan(&seedCount).Error; err != nil {
			t.Fatal(err)
		}
		if seedCount != 1 {
			t.Fatalf("seed %s user rule count=%d", table, seedCount)
		}
	}
	var fields string
	if err := db.Table("go_security_sensitive_data").Where("id=2").Pluck("data_fields", &fields).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fields, "password") {
		t.Fatal("sensitive seed exposes password")
	}
}

func TestMigrationRegistry(t *testing.T) {
	official := OfficialMigrations()
	if err := ValidateOfficialMigrations(official); err != nil {
		t.Fatal(err)
	}
	want := []int64{20230622221507, 20230719211338, 20230905060702, 20231112093414, 20231229043002, 20250412134127}
	if len(official) != len(want) {
		t.Fatalf("migration count = %d", len(official))
	}
	for i, v := range want {
		if official[i].Key.Version != v {
			t.Fatalf("migration %d = %d, want %d", i, official[i].Key.Version, v)
		}
	}
}

func TestSeedMarker(t *testing.T) {
	if installDataVersion != 20230620180916 || installDataName != "InstallData" {
		t.Fatal("unexpected install marker")
	}
}
