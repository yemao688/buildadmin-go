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
		&model.AdminClosure{},
		&model.AdminHierarchyLock{},
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
	var baselineOwner int32
	if err := db.Table("go_security_data_recycle").Where("id=1").Pluck("admin_id", &baselineOwner).Error; err != nil {
		t.Fatal(err)
	}
	if baselineOwner != 0 {
		t.Fatalf("upstream baseline wrote local owner %d", baselineOwner)
	}
	seedConfig := &conf.Configuration{Database: conf.Database{Prefix: "go_"}}
	if err := MarkSeedPending(db, seedConfig); err != nil {
		t.Fatal(err)
	}
	if err := RunFreshSeed(db, seedConfig, LocalMigrations()); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"security_data_recycle", "security_sensitive_data"} {
		var ownerCount int64
		if err := db.Raw("SELECT COUNT(*) FROM `go_" + table + "` r JOIN `go_admin` a ON a.id=r.admin_id WHERE r.data_table='user' AND r.admin_id > 0").Scan(&ownerCount).Error; err != nil {
			t.Fatal(err)
		}
		if ownerCount == 0 {
			t.Fatalf("seed %s user rule has no valid admin owner", table)
		}
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
