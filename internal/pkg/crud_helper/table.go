package crud_helper

import (
	"bytes"
	"fmt"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	model "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
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
func HandleTableDesign(db *gorm.DB, fullTableName string, table crudmodel.Table, fields []crudmodel.Field) error {
	if err := ValidateGenerationInput(table, fields); err != nil {
		return err
	}
	if err := data_scope.ValidateIdentifier(fullTableName); err != nil {
		return err
	}
	comment := table.Comment
	if db.Migrator().HasTable(fullTableName) {
		//更新表
		if err := db.Exec("ALTER TABLE `" + fullTableName + "` COMMENT = '" + escapeSQLString(comment) + "'").Error; err != nil {
			return err
		}
		designChange := table.DesignChange
		if len(designChange) == 0 {
			pk := getPk(fields)
			ownerCol := resolveOwnerColumn(table.DataScope, fields)
			return EnsureDataScopeIndex(db, fullTableName, ownerCol, pk)
		}
		for _, v := range designChange {
			if !v.Sync {
				continue
			}

			if v.Type == "change-field-attr" {
				columnExists, err := hasColumn(db, fullTableName, v.OldName)
				if err != nil {
					return err
				}
				if !columnExists {
					return cErr.BadRequest(v.OldName + " not exist")
				}
				oldField := searchField(fields, v.OldName)
				fieldData, err := getDDlFieldData(oldField)
				if err != nil {
					return err
				}
				if err := db.Exec("ALTER TABLE `" + fullTableName + "` MODIFY " + trimDDLFragment(fieldData)).Error; err != nil {
					return err
				}

			} else if v.Type == "add-field" {
				columnExists, err := hasColumn(db, fullTableName, v.NewName)
				if err != nil {
					return err
				}
				if columnExists {
					return cErr.BadRequest(v.NewName + " is exist")
				}

				newField := searchField(fields, v.NewName)
				fieldData, err := getDDlFieldData(newField)
				if err != nil {
					return err
				}
				if err := db.Exec("ALTER TABLE `" + fullTableName + "` ADD  " + trimDDLFragment(fieldData)).Error; err != nil {
					return err
				}
			}
		}
	} else {
		//创建表
		ddl, err := createTableDDL(fullTableName, table, fields)
		if err != nil {
			return err
		}
		if err := db.Exec(ddl).Error; err != nil {
			return err
		}
	}
	// Ensure a data-scope index exists whenever the resolved owner column is
	// not the primary key. This makes the generated DDL self-consistent with
	// the fail-closed index proof required by ResolveDataScope.
	pk := getPk(fields)
	ownerCol := resolveOwnerColumn(table.DataScope, fields)
	return EnsureDataScopeIndex(db, fullTableName, ownerCol, pk)
}

func createTableDDL(fullTableName string, table crudmodel.Table, fields []crudmodel.Field) (string, error) {
	pk := getPk(fields)
	keysSQL := "PRIMARY KEY (`" + pk + "`)"
	for _, idx := range table.Indexes {
		columns := make([]string, 0, len(idx.Columns))
		for _, col := range idx.Columns {
			columns = append(columns, quoteIndexColumn(col))
		}
		keyWord := "KEY"
		if idx.Unique {
			keyWord = "UNIQUE KEY"
		}
		keysSQL += ",\n " + keyWord + " `" + idx.Name + "` (" + strings.Join(columns, ", ") + ")"
	}
	sqlData := SqlTemplData{
		TableName: fullTableName,
		Fields:    "",
		Keys:      keysSQL,
		Engine:    "ENGINE=InnoDB",
		Charset:   "DEFAULT CHARSET=utf8mb4",
		SortRule:  "COLLATE=utf8mb4_unicode_ci",
		RowFormat: "row_format=DYNAMIC",
		Comment:   "",
	}
	if table.Comment != "" {
		sqlData.Comment = formatComment(table.Comment)
	}
	for _, field := range fields {
		fieldData, err := getDDlFieldData(field)
		if err != nil {
			return "", err
		}
		sqlData.Fields += fieldData
	}
	var buf bytes.Buffer
	tpl, err := template.New(SqlTempl).Parse(SqlTempl)
	if err != nil {
		return "", err
	}
	if err := tpl.Execute(&buf, sqlData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func hasColumn(db *gorm.DB, fullTableName, column string) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database is required")
	}
	if err := data_scope.ValidateIdentifier(fullTableName); err != nil {
		return false, err
	}
	if err := data_scope.ValidateIdentifier(column); err != nil {
		return false, err
	}
	var count int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?",
		fullTableName, column,
	).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// EnsureDataScopeIndex creates idx_<ownerColumn> on fullTableName when the
// owner column is configured and is not the primary key.
func EnsureDataScopeIndex(db *gorm.DB, fullTableName, ownerColumn, pk string) error {
	if db == nil || ownerColumn == "" || ownerColumn == pk {
		return nil
	}
	if err := data_scope.ValidateIdentifier(fullTableName); err != nil {
		return err
	}
	if err := data_scope.ValidateIdentifier(ownerColumn); err != nil {
		return err
	}
	indexName := "idx_" + ownerColumn
	if err := ValidateIndexName(indexName); err != nil {
		return err
	}
	var sameNameFirstColumn string
	if err := db.Raw(
		"SELECT COLUMN_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ? AND SEQ_IN_INDEX = 1 LIMIT 1",
		fullTableName, indexName,
	).Scan(&sameNameFirstColumn).Error; err != nil {
		return err
	}
	if sameNameFirstColumn != "" && sameNameFirstColumn != ownerColumn {
		return fmt.Errorf("data_scope: index %q exists but its first column is %q, not owner column %q", indexName, sameNameFirstColumn, ownerColumn)
	}
	if sameNameFirstColumn == ownerColumn {
		return nil
	}
	var ownerLeadingIndex string
	if err := db.Raw(
		"SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ? AND SEQ_IN_INDEX = 1 LIMIT 1",
		fullTableName, ownerColumn,
	).Scan(&ownerLeadingIndex).Error; err != nil {
		return err
	}
	if ownerLeadingIndex != "" {
		return nil
	}
	return db.Exec("CREATE INDEX `" + indexName + "` ON `" + fullTableName + "` (`" + ownerColumn + "`)").Error
}

// trimDDLFragment removes the trailing comma and whitespace that FieldTempl
// appends for CREATE TABLE field lists. ALTER statements (CHANGE/MODIFY/ADD/
// FIRST/AFTER) require a bare fragment.
func trimDDLFragment(fieldData string) string {
	return strings.TrimSuffix(strings.TrimRight(fieldData, " \t\r\n"), ",")
}

func searchField(fields []crudmodel.Field, name string) crudmodel.Field {
	findField := crudmodel.Field{}
	for _, field := range fields {
		if field.Name == name {
			findField = field
			break
		}
	}
	return findField
}

func getDDlFieldData(field crudmodel.Field) (string, error) {
	normalizeFieldConfiguration(&field)
	if err := ValidateField(field); err != nil {
		return "", err
	}

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

	if !field.Null {
		fieldTemplData.Null = "NOT NULL"
	}

	if field.DataType != "" {
		fieldTemplData.DataSet = field.DataType
	}

	if field.DefaultType != "" {
		// 优先按上游 defaultType 语义生成默认值子句
		switch field.DefaultType {
		case "EMPTY STRING":
			if !noDefaultValueType(dateType) {
				fieldTemplData.DefaultValue = "DEFAULT ''"
			}
		case "NULL":
			if !noDefaultValueType(dateType) {
				fieldTemplData.DefaultValue = "DEFAULT NULL"
			}
		case "INPUT":
			if !noDefaultValueType(dateType) {
				fieldTemplData.DefaultValue = formatDefault(field.Default)
			}
		case "NONE":
			fieldTemplData.DefaultValue = ""
		}
	} else if field.Default != "" && field.Default != "none" {
		// 兼容旧的哨兵值写法(无 defaultType 的场景)
		if field.Default == "empty string" {
			if !noDefaultValueType(dateType) {
				fieldTemplData.DefaultValue = "DEFAULT ''"
			}
		} else if field.Default == "null" {
			if !noDefaultValueType(dateType) {
				fieldTemplData.DefaultValue = "DEFAULT NULL"
			}
		} else {
			fieldTemplData.DefaultValue = formatDefault(field.Default)
		}
	}

	if field.AutoIncrement && field.PrimaryKey {
		fieldTemplData.Increment = "AUTO_INCREMENT"
	}

	if field.Comment != "" {
		fieldTemplData.Comment = formatComment(field.Comment)
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

var noDefaultValueTypes = map[string]bool{
	"text": true, "blob": true, "geometry": true, "geometrycollection": true, "json": true,
	"linestring": true, "longblob": true, "longtext": true, "mediumblob": true, "mediumtext": true,
	"multilinestring": true, "multipoint": true, "multipolygon": true, "point": true, "polygon": true,
	"tinyblob": true,
}

func noDefaultValueType(dataType string) bool { return noDefaultValueTypes[strings.ToLower(dataType)] }

// 分析字段的完整数据类型定义
func analyseFieldDataType(field crudmodel.Field) string {
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
	if len(data) == 0 {
		return conciseType
	}
	return conciseType + "(" + data[0] + ")"
}

// 分析字段limit和精度
func analyseFieldLimit(conciseType string, field crudmodel.Field) (string, []string) {
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
func ParseTableColumns(columns []model.Column, analyseField bool) []crudmodel.Field {

	fields := []crudmodel.Field{}
	for _, v := range columns {
		field := crudmodel.Field{}
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

		// 默认值类型,语义对齐上游 BuildAdmin v2
		if isNullAble && !v.COLUMN_DEFAULT.Valid {
			field.DefaultType = "NULL"
		} else if v.COLUMN_DEFAULT.Valid && v.COLUMN_DEFAULT.String == "" && (v.DATA_TYPE == "varchar" || v.DATA_TYPE == "char") {
			field.DefaultType = "EMPTY STRING"
		} else if !isNullAble && !v.COLUMN_DEFAULT.Valid {
			field.DefaultType = "NONE"
		} else {
			field.DefaultType = "INPUT"
			field.Default = v.COLUMN_DEFAULT.String
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
