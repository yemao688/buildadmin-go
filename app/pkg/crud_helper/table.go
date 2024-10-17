package crud_helper

import (
	"bytes"
	"fmt"
	"go-build-admin/app/admin/model"
	cErr "go-build-admin/app/pkg/error"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"gorm.io/gorm"
)

const SqlTempl = "CREATE TABLE `{{.TableName}}` (\n {{.Fields}} {{.Keys}}) {{.Engine}} {{.Charset}} {{.SortRule}} {{.RowFormat}} {{.Comment}};"

type SqlTemplData struct {
	TableName string
	Fields    string
	Keys      string
	Engine    string
	Charset   string
	SortRule  string
	RowFormat string
	Comment   string
}

const FieldTempl = "{{.Name}} {{.Type}} {{.Unsigned}} {{.Charset}} {{.SortRule}} {{.Null}} {{.DefaultValue}} {{.Increment}} {{.Comment}},\n"

type FieldTemplData struct {
	Name         string
	Type         string
	Unsigned     string
	Charset      string
	SortRule     string
	Null         string
	DataSet      string
	DefaultValue string
	Increment    string
	Comment      string
}

// 创建表或更新表
func HandleTableDesign(db *gorm.DB, fullTableName string, table model.Table, fields []model.Field) error {
	comment := table.Comment
	pk := getPk(fields)
	if db.Migrator().HasTable(fullTableName) {
		//更新表
		if err := db.Exec("ALTER TABLE `"+fullTableName+"` COMMENT = ?", comment).Error; err != nil {
			return err
		}
		designChange := table.DesignChange
		if len(designChange) == 0 {
			return nil
		}
		// 改名和删除操作优先
		for _, v := range designChange {
			if !v.Sync {
				continue
			}

			if slices.Contains([]string{"change-field-name", "del-field"}, v.Type) && !db.Migrator().HasColumn(fullTableName, v.OldName) {
				return cErr.BadRequest(v.OldName + " not exist")
			}

			if v.Type == "change-field-name" {
				oldField := searchField(fields, v.OldName)
				fieldData, err := getDDlFieldData(oldField)
				if err != nil {
					return err
				}
				fieldData = strings.ReplaceAll(fieldData, "'"+v.OldName+"'", "'"+v.OldName+"'"+" `"+v.NewName+"`")
				if err := db.Exec("ALTER TABLE `" + fullTableName + "` CHANGE " + fieldData).Error; err != nil {
					return err
				}

			} else if v.Type == "del-field" {
				if err := db.Exec("ALTER TABLE `" + fullTableName + "` DROP COLUMN `" + v.OldName + "`").Error; err != nil {
					return err
				}
			}
		}

		for _, v := range designChange {
			if !v.Sync {
				continue
			}

			if v.Type == "change-field-attr" {
				if !db.Migrator().HasColumn(fullTableName, v.OldName) {
					return cErr.BadRequest(v.OldName + " not exist")
				}
				oldField := searchField(fields, v.OldName)
				fieldData, err := getDDlFieldData(oldField)
				if err != nil {
					return err
				}
				if err := db.Exec("ALTER TABLE `" + fullTableName + "` MODIFY " + fieldData).Error; err != nil {
					return err
				}

			} else if v.Type == "add-field" {
				if db.Migrator().HasColumn(fullTableName, v.NewName) {
					return cErr.BadRequest(v.NewName + " is exist")
				}

				newField := searchField(fields, v.NewName)
				fieldData, err := getDDlFieldData(newField)
				if err != nil {
					return err
				}
				if err := db.Exec("ALTER TABLE `" + fullTableName + "` ADD  " + fieldData).Error; err != nil {
					return err
				}
			}
		}
		// 表更新结构完成再处理字段排序
		if err := updateFieldOrder(db, fullTableName, fields, designChange); err != nil {
			return err
		}
	} else {
		//创建表
		sqlData := SqlTemplData{
			TableName: fullTableName,
			Fields:    "",
			Keys:      "PRIMARY KEY (`" + pk + "`)",
			Engine:    "ENGINE=InnoDB",
			Charset:   "DEFAULT CHARSET=utf8mb4",
			SortRule:  "COLLATE=utf8mb4_unicode_ci",
			RowFormat: "row_format=DYNAMIC",
			Comment:   "",
		}
		if comment != "" {
			sqlData.Comment = "COMMENT='" + comment + "'"
		}

		for _, v := range fields {
			str, err := getDDlFieldData(v)
			if err != nil {
				return err
			}
			sqlData.Fields += str
		}

		var buf bytes.Buffer
		tpl, err := template.New(SqlTempl).Parse(SqlTempl)
		if err != nil {
			return err
		}
		if err := tpl.Execute(&buf, sqlData); err != nil {
			return err
		}

		err = db.Exec(buf.String()).Error
		return err
	}
	return nil
}

func searchField(fields []model.Field, name string) model.Field {
	findField := model.Field{}
	for _, field := range fields {
		if field.Name == name {
			findField = field
			break
		}
	}
	return findField
}

func getDDlFieldData(field model.Field) (string, error) {

	dateType := analyseFieldDataType(field)
	dateType = strings.TrimSuffix(dateType, "(0)")
	fieldTemplData := FieldTemplData{
		Name:     "`" + field.Name + "`",
		Type:     dateType,
		Charset:  "",
		SortRule: "",
		DataSet:  "",
	}

	if field.Unsigned {
		fieldTemplData.Unsigned = "unsigned"
	}

	// if field.Null != "1" {
	// 	fieldTemplData.Charset = "NOT NULL"
	// 	fieldTemplData.SortRule = "NOT NULL"
	// }

	if field.Null {
		fieldTemplData.Null = "NOT NULL"
	}

	if field.DataType != "" {
		fieldTemplData.DataSet = field.DataType
	}

	if field.Default != "" && field.Default != "none" {
		if field.Default == "empty string" {
			if dateType != "text" {
				fieldTemplData.DefaultValue = "DEFAULT ''"
			}
		} else if field.Default == "null" {
			fieldTemplData.DefaultValue = "DEFAULT NULL"
		} else {
			fieldTemplData.DefaultValue = "DEFAULT '" + field.Default + "'"
		}
	}

	if field.AutoIncrement && field.PrimaryKey {
		fieldTemplData.Increment = "AUTO_INCREMENT"
	}

	if field.Comment != "" {
		fieldTemplData.Comment = "COMMENT '" + field.Comment + "'"
	}

	var buf bytes.Buffer
	tpl, err := template.New(FieldTempl).Parse(FieldTempl)
	if err != nil {
		return "", err
	}
	if err := tpl.Execute(&buf, fieldTemplData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func updateFieldOrder(db *gorm.DB, fullTableName string, fields []model.Field, designChange []model.ChangeField) error {
	if len(designChange) == 0 {
		return nil
	}

	for _, v := range designChange {
		if !v.Sync {
			continue
		}

		if v.After != "" {
			fieldName := v.OldName
			if slices.Contains([]string{"add-field", "change-field-name"}, v.Type) {
				fieldName = v.NewName
			}

			field := searchField(fields, fieldName)
			fieldData, err := getDDlFieldData(field)
			if err != nil {
				return err
			}

			if v.After == "FIRST FIELD" {
				fieldData += " FIRST"
			} else {
				fieldData += " FIRST  AFTER `" + v.After + "`"
			}
			if err := db.Exec("ALTER TABLE `" + fullTableName + "` MODIFY " + fieldData).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// 分析字段的完整数据类型定义
func analyseFieldDataType(field model.Field) string {
	if field.DataType != "" {
		return field.DataType
	}

	conciseType := analyseFieldType(field)
	limitType, data := analyseFieldLimit(conciseType, field)

	if limitType == "decimalType" {
		return conciseType + "(" + data[0] + "," + data[1] + ")"
	} else if limitType == "valuesType" {
		return conciseType + "(" + strings.Join(data, ",") + ")"
	}
	return conciseType + "(" + data[0] + ")"
}

// 分析字段limit和精度
func analyseFieldLimit(conciseType string, field model.Field) (string, []string) {
	decimalType := []string{"decimal", "double", "float"}
	valuesType := []string{"enum", "set"}

	dataTL := dataTypeLimit(field.DataType)
	if slices.Contains(decimalType, conciseType) {
		if len(dataTL) == 1 {
			return "decimalType", []string{dataTL[0], "0"}
		}
		if len(dataTL) == 2 {
			return "decimalType", []string{dataTL[0], dataTL[1]}
		}
		precision := 10
		if field.Length != 0 {
			precision = field.Length
		}
		scale := 0
		if field.Precision != 0 {
			scale = field.Precision
		}
		return "decimalType", []string{fmt.Sprintf("%v", precision), fmt.Sprintf("%v", scale)}
	}

	if slices.Contains(valuesType, conciseType) {
		values := []string{}
		for _, v := range dataTL {
			v = strings.ReplaceAll(v, "\"", "")
			v = strings.ReplaceAll(v, "'", "")
			values = append(values, v)
		}
		return "valuesType", values
	}

	if len(dataTL) > 0 {
		return "limitType", []string{dataTL[0]}
	}

	if field.Length != 0 {
		return "limitType", []string{fmt.Sprintf("%v", field.Length)}
	}
	return "", nil
}

func dataTypeLimit(dataType string) []string {
	content := []string{}

	re := regexp.MustCompile(`\((.*?)\)`)
	matches := re.FindStringSubmatch(dataType)

	// 检查是否有匹配项
	if len(matches) > 1 {
		// 分割匹配到的内容
		group := matches[1]
		group = strings.Trim(group, ",")
		content = strings.Split(group, ",")
	}
	return content
}

// 根据数据表解析字段数据
func ParseTableColumns(columns []model.Column, analyseField bool) []model.Field {

	fields := []model.Field{}
	for _, v := range columns {
		field := model.Field{}
		field.Name = v.COLUMN_NAME
		field.Type = v.DATA_TYPE

		dataType := ""
		if strings.Contains(v.COLUMN_TYPE, "(") {
			position := strings.Index(v.COLUMN_TYPE, "(")
			dataType = v.COLUMN_TYPE[:position]
		} else {
			dataType = strings.ReplaceAll(v.COLUMN_TYPE, " unsigned", "")
		}
		field.DataType = dataType

		isNullAble := false
		if v.IS_NULLABLE == "YES" {
			isNullAble = true
		}
		field.Null = isNullAble
		if isNullAble {
			field.Default = "null"
		} else {
			field.Default = v.COLUMN_DEFAULT
		}

		primaryKey := false
		if v.COLUMN_KEY == "PRI" {
			primaryKey = true
		}
		field.PrimaryKey = primaryKey

		unsigned := false
		if strings.Contains(v.COLUMN_TYPE, "unsigned") {
			unsigned = true
		}
		field.Unsigned = unsigned

		autoIncrement := false
		if strings.Contains(v.EXTRA, "auto_increment") {
			autoIncrement = true
		}
		field.AutoIncrement = autoIncrement
		field.Comment = v.COLUMN_COMMENT
		field.DesignType = getTableColumnsDataType(v)

		fields = append(fields, field)
	}
	return fields
}

func getTableColumnsDataType(column model.Column) string {
	if strings.Contains(column.COLUMN_NAME, "id") && strings.Contains(column.EXTRA, "auto_increment") {
		return "pk"
	} else if column.COLUMN_NAME == "weigh" {
		return "weigh"
	} else if slices.Contains([]string{"createtime", "updatetime", "create_time", "update_time"}, column.COLUMN_NAME) {
		return "timestamp"
	}

	for _, v := range inputTypeRule {
		typeBool := true
		suffixBool := true
		columnTypeBool := true
		if v.Type != nil && len(v.Type) > 0 && !slices.Contains(v.Type, column.DATA_TYPE) {
			typeBool = false
		}

		if v.Suffix != nil && len(v.Suffix) > 0 {
			suffixBool = isMatchSuffix(column.COLUMN_NAME, v.Suffix)
		}

		if v.ColumnType != nil && len(v.ColumnType) > 0 && !slices.Contains(v.ColumnType, column.COLUMN_TYPE) {
			columnTypeBool = false
		}

		if typeBool && suffixBool && columnTypeBool {
			return v.Value
		}
	}
	return "string"
}

func isMatchSuffix(name string, suffixArr []string) bool {
	for _, v := range suffixArr {
		if strings.HasSuffix(name, v) {
			return true
		}
	}
	return false
}

// 解析到的表字段的额外处理
// func handleTableColumn() {
// 	// 预留
// }
