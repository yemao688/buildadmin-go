package main

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

//通过注释写sql语句,生成代码是会生成相应的查询代码
// type QuseryRule interface {
// 	//SELECT ag.rules  FROM  yn_auth_group_access aga  LEFT JOIN yn_auth_group ag ON ag.id=aga.group_id where aga.uid=@id;
// 	GetRuleIds(id int32) ([]gen.M, error)
// }

func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath: "./",
		Mode:    gen.WithoutContext | gen.WithDefaultQuery,
		//if you want the nullable field generation property to be pointer type, set FieldNullable true
		// FieldNullable: true,
		//if you want to assign field which has default value in Create API, set FieldCoverable true, reference: https://gorm.io/docs/create.html#Default-Values
		/* FieldCoverable: true,*/
		// if you want generate field with unsigned integer type, set FieldSignable true
		/* FieldSignable: true,*/
		//if you want to generate index tags from database, set FieldWithIndexTag true
		/* FieldWithIndexTag: true,*/
		//if you want to generate type tags from database, set FieldWithTypeTag true
		/* FieldWithTypeTag: true,*/
		//if you need unit tests for query code, set WithUnitTest true
		// WithUnitTest: true,
	})

	db, _ := gorm.Open(mysql.Open("root:root@(127.0.0.1:3306)/promotion?charset=utf8mb4&parseTime=True&loc=Local"))
	g.UseDB(db)

	onlyModel := true
	// tables := []string{
	// 	"ba_admin",
	// 	"ba_admin_group",
	// 	"ba_admin_group_access",
	// 	"ba_admin_log",
	// 	"ba_test_build",
	// }
	tables := []string{}
	models, err := genModels(g, db, tables)
	if err != nil {
		log.Fatalln("get tables info fail:", err)
	}

	if !onlyModel {
		g.ApplyBasic(models...)
	}

	// g.ApplyInterface(func(QuseryRule) {}, Admin)
	g.Execute()
}

// genModels is gorm/gen generated models
func genModels(g *gen.Generator, db *gorm.DB, tables []string) (models []interface{}, err error) {
	if len(tables) == 0 {
		// Execute tasks for all tables in the database
		tables, err = db.Migrator().GetTables()
		if err != nil {
			return nil, fmt.Errorf("GORM migrator get all tables fail: %w", err)
		}
	}

	// Execute some data table tasks
	models = make([]interface{}, len(tables))
	for i, tableName := range tables {
		models[i] = g.GenerateModel(tableName)
	}
	return models, nil
}
