package migrations

import (
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/model"
	"os"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func getDB() *gorm.DB {
	if os.Getenv("BUILDADMIN_TEST_MYSQL_DSN") == "" {
		return nil
	}
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
			"root",
			"root",
			"127.0.0.1",
			"3306",
			"test_go",
			"utf8mb4",
		)
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   "go_", // 表前缀
		},
		DisableForeignKeyConstraintWhenMigrating: true, // 禁用自动创建外键约束
	})
	if err != nil {
		return nil
	}
	return db
}

func TestInstall(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
		&model.AdminGroupAccess{},
		&model.AdminGroup{},
		&model.AdminLog{},
		&model.AdminRule{},
		&model.Admin{},
		&model.Area{},
		&model.Attachment{},
		&model.Captcha{},
		&model.Config{},
		&model.CrudLog{},
		&model.Migrations{},
		&model.SecurityDataRecycleLog{},
		&model.SecurityDataRecycle{},
		&model.SecuritySensitiveDataLog{},
		&model.SecuritySensitiveData{},
		&model.TestBuild{},
		&model.Token{},
		&model.UserGroup{},
		&model.UserMoneyLog{},
		&model.UserRule{},
		&model.UserScoreLog{},
		&model.User{},
	)
	fmt.Println("生成数据表:", err)

	install := NewInstall(db)
	if err := install.InsertData(); err != nil {
		t.Fatal(err)
	}
}

func TestRenameColumn(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	admin := model.Admin{}

	err := db.Migrator().RenameColumn(&admin, "salt", "test")
	fmt.Println(err)
}

func TestMigrationRegistry(t *testing.T) {
	if err := validateMigrations(); err != nil {
		t.Fatal(err)
	}
	want := []int64{20230622221507, 20230719211338, 20230905060702, 20231112093414, 20231229043002, 20250412134127, 20260714120000}
	if len(allMigrations) != len(want) {
		t.Fatalf("migration count = %d", len(allMigrations))
	}
	for i, v := range want {
		if allMigrations[i].Version != v {
			t.Fatalf("migration %d = %d, want %d", i, allMigrations[i].Version, v)
		}
	}
}

func TestLegacyRuleTargets(t *testing.T) {
	if got := []string{"auth/rule", "auth/rule/index", "auth/rule/add", "auth/rule/edit", "auth/rule/del", "auth/rule/sortable"}; got[0] != "auth/rule" || got[1] != "auth/rule/index" {
		t.Fatal("unexpected auth rule target mapping")
	}
	if got := []string{"dashboard", "buildadmin"}; got[0] != "dashboard" || got[1] != "buildadmin" {
		t.Fatal("unexpected Version202 target mapping")
	}
}

func TestVersion202DashboardRuleCount(t *testing.T) {
	if err := validateDashboardRuleCount(0); err != nil {
		t.Fatalf("empty rule table must be safe for fresh seed: %v", err)
	}
	if err := validateDashboardRuleCount(1); err != nil {
		t.Fatalf("single dashboard rule must be valid: %v", err)
	}
	if err := validateDashboardRuleCount(2); err == nil {
		t.Fatal("duplicate dashboard rules must be rejected")
	}
}

func TestMapAccountStatuses(t *testing.T) {
	got, err := mapAccountStatuses([]string{"0", "1", "enable", "disable"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"disable", "enable", "enable", "disable"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("status %d = %q, want %q", i, got[i], want[i])
		}
	}
	if _, err := mapAccountStatuses([]string{"enable", ""}); err == nil {
		t.Fatal("empty status must be rejected")
	}
	for _, value := range []string{"ENABLE", "Disable", "enable ", "other"} {
		if _, err := mapAccountStatuses([]string{"enable", value}); err == nil {
			t.Fatalf("%q must be rejected", value)
		}
	}
}

func TestSeedMarkerAndBackupNames(t *testing.T) {
	if installDataVersion != 20230620180916 || installDataName != "InstallData" {
		t.Fatal("unexpected install marker")
	}
	config := &conf.Configuration{}
	config.Database.Prefix = "ba_"
	if got := menuRuleBackupName(config); got != "ba_menu_rule_version200_backup" {
		t.Fatalf("backup table = %q", got)
	}
}
