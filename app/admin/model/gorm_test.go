package model

import (
	"fmt"
	"go-build-admin/database/migrations/model"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func getDb() *gorm.DB {
	dsn := "root:root@tcp(localhost:3306)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   "ba_", // 表前缀
		},
		Logger: logger.Default.LogMode(logger.Info),
	})
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

func TestJoin(t *testing.T) {
	db := getDb()
	// list := []UserMoneyLog{}
	// err := db.Model(&UserMoneyLog{}).Preload("User").Find(&list).Error
	// fmt.Printf("%+v", err)
	// fmt.Printf("%+v", list)

	// groups := []struct {
	// 	Id   int32
	// 	Name string
	// }{}
	// err := db.Table("ba_admin_group_access").
	// 	Joins("left join ba_admin_group g on g.id=ba_admin_group_access.group_id").
	// 	Select("g.id as id,g.name as name").
	// 	Where("ba_admin_group_access.uid=?", 1).Scan(&groups).Error
	// fmt.Printf("%+v", err)
	// fmt.Printf("%+v", groups)

	var result map[string]interface{}
	db.Model(&model.Captcha{}).Where(" `key` = ? ", 1).Scan(&result)
	fmt.Printf("%+v", result)

	// var result1 map[string]interface{}
	// db.Table("ba_captcha").Where(" `key` = ? ", 1).Scan(&result1)
	// fmt.Printf("%+v", result1)

	// result2 := model.Captcha{}
	// db.Where(" `key` = ? ", 1).Scan(&result2)
	// fmt.Printf("%+v", result2)

}
