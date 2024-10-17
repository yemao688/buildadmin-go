package crud_helper

import (
	"fmt"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestAlter(t *testing.T) {
	db, _ := gorm.Open(mysql.Open("root:root@(127.0.0.1:3306)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local"))
	comment := "test表"
	if err := db.Exec("ALTER TABLE `"+"ba_test5"+"` COMMENT = ?", comment).Error; err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("成功")
	}

	// if err := db.Exec("ALTER TABLE `ba_test5` COMMENT = ''").Error; err != nil {
	// 	fmt.Println(err)
	// } else {
	// 	fmt.Println("成功")
	// }
}
