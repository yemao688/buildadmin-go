package local

import (
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func getDB() *gorm.DB {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		return nil
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil
	}
	return db
}
