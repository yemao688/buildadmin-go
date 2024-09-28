package migrations

import (
	"fmt"
	"go-build-admin/database/migrations/model"
	"log"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func getDB() *gorm.DB {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		"root",
		"root",
		"127.0.0.1",
		"3306",
		"te",
		"utf8mb4",
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, // 禁用自动创建外键约束
	})
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}
	return db
}

func TestParseWebDirNameData(t *testing.T) {
	db := getDB()
	install := NewInstall(db)
	install.CreateTable()
}

func TestAddColumn(t *testing.T) {
	db := getDB()
	admin := model.Admin{}

	err := db.Migrator().RenameColumn(&admin, "salt", "test")
	fmt.Println(err)
}
