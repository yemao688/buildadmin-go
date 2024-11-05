package migrations

import (
	"fmt"
	"go-build-admin/database/migrations/model"
	"log"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func getDB() *gorm.DB {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		"root",
		"root",
		"127.0.0.1",
		"3306",
		"test_go",
		"utf8mb4",
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   "go_", // 表前缀
		},
		DisableForeignKeyConstraintWhenMigrating: true, // 禁用自动创建外键约束
	})
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}
	return db
}

func TestInstall(t *testing.T) {
	db := getDB()
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
	install.InsertData()
}

func TestRenameColumn(t *testing.T) {
	db := getDB()
	admin := model.Admin{}

	err := db.Migrator().RenameColumn(&admin, "salt", "test")
	fmt.Println(err)
}
