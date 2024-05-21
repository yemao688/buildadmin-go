package model

import (
	"fmt"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func getDb() *gorm.DB {
	dsn := "root:root@tcp(localhost:3306)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	return db
}

func TestBelong(t *testing.T) {
	db := getDb()
	log := UserMoneyLog{}
	db.Table("ba_user_money_log").Preload("ba_user").Where("id=1").Find(&log)

	fmt.Printf("%+v", log)
}
