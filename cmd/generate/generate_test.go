package main

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

const ModelTmpl = `
package {{.StructInfo.Package}}

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	{{range .ImportPkgPaths}}{{.}} ` + "\n" + `{{end}}
)

{{if .TableName -}}const TableName{{.ModelStructName}} = "{{.TableName}}"{{- end}}

// {{.ModelStructName}} {{.StructComment}}
type {{.ModelStructName}} struct {
    {{range .Fields}}
    {{if .MultilineComment -}}
	/*
{{.ColumnComment}}
    */
	{{end -}}
    {{.Name}} {{.Type}} ` + "`{{.Tags}}` " +
	"{{if not .MultilineComment}}{{if .ColumnComment}}// {{.ColumnComment}}{{end}}{{end}}" +
	`{{end}}
}

`

func TestGenerateModel(t *testing.T) {
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

	db, _ := gorm.Open(mysql.Open("root:root@(127.0.0.1:3306)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local"))
	g.UseDB(db)

	// tables, err := db.Migrator().GetTables()
	// if err != nil {
	// 	fmt.Printf("GORM migrator get all tables fail: %w", err)
	// }
	tables := []string{"ba_area"}

	for _, tableName := range tables {
		data := g.GenerateModel(tableName)

		var buf bytes.Buffer
		t, err := template.New(ModelTmpl).Parse(ModelTmpl)
		if err != nil {
			fmt.Printf("parse tmpl fail: %v", err)
		}

		if err := t.Execute(&buf, data); err != nil {
			fmt.Printf("parse tmpl fail: %v", err)
		}
		fmt.Print(buf.String())
	}

}
