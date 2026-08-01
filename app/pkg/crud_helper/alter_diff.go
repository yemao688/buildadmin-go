package crud_helper

import (
	"database/sql"
	"go-build-admin/app/admin/model"
	crudmodel "go-build-admin/app/admin/model/crud"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// alter 差量的属性级比较。
//
// 浅比较（列存在即 change-field-attr）会让 alter 每次都对全字段发 MODIFY ——
// 对生成是噪音，对 crud:apply 的部署幂等性是破坏（MySQL MODIFY 可能整表重建）。
// 这里按 information_schema 的实际归一化形态逐属性对齐，只有真实漂移才产生差量。

var intDisplayWidthPattern = regexp.MustCompile(`(?i)^(bigint|int|mediumint|smallint|tinyint)\(\d+\)`)
var multiSpaceCommaPattern = regexp.MustCompile(`\s*,\s*`)

type DiffClass string

const (
	DiffSafeAuto         DiffClass = "safe-auto"
	DiffRequiresApproval DiffClass = "requires-approval"
	DiffRejected         DiffClass = "rejected"
	DiffUnmanaged        DiffClass = "unmanaged"
)

type ApprovalCategory string

const (
	ApprovalDefaults      ApprovalCategory = "defaults"
	ApprovalAutoIncrement ApprovalCategory = "auto-increment"
	ApprovalTypeWidening  ApprovalCategory = "type-widening"
	ApprovalAttributes    ApprovalCategory = "attributes"
)

var approvalCategories = []ApprovalCategory{
	ApprovalDefaults,
	ApprovalAutoIncrement,
	ApprovalTypeWidening,
	ApprovalAttributes,
}

type AlterDiff struct {
	Change    crudmodel.ChangeField
	Field     crudmodel.Field
	Column    *model.Column
	Class     DiffClass
	Category  ApprovalCategory
	Reason    string
	Unmanaged []string
}

// specColumnType 渲染 spec 字段对应的 MySQL COLUMN_TYPE（与 getDDlFieldData 同一类型来源）。
func specColumnType(field crudmodel.Field) string {
	columnType := analyseFieldDataType(field)
	columnType = strings.TrimSuffix(columnType, "(0)")
	if field.Unsigned {
		columnType += " unsigned"
	}
	return columnType
}

// normalizeColumnType 对齐 MySQL 8 归一化：整型去显示宽度、enum/set 去逗号空格、小写。
func normalizeColumnType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = intDisplayWidthPattern.ReplaceAllString(normalized, "$1")
	return multiSpaceCommaPattern.ReplaceAllString(normalized, ",")
}

func specFieldNullable(field crudmodel.Field) bool {
	return field.Null || strings.EqualFold(field.DefaultType, "NULL")
}

// normalizeDefaultValue 保留旧的无类型调用入口，同时避免 float64 精度损失。
func normalizeDefaultValue(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.Trim(normalized, "'")
	if integer, ok := new(big.Int).SetString(normalized, 10); ok {
		return integer.String()
	}
	if rational, ok := new(big.Rat).SetString(normalized); ok {
		return rational.RatString()
	}
	return normalized
}

func normalizeDefaultValueForField(field crudmodel.Field, value string) string {
	base := strings.ToLower(analyseFieldTypeForSpec(field))
	if base == "" {
		base = strings.ToLower(field.Type)
	}
	if isIntegerType(base) {
		return normalizeIntegerDefault(value)
	}
	if base == "decimal" || base == "numeric" {
		return normalizeDecimalDefault(value)
	}
	return value
}

func normalizeIntegerDefault(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "'")
	integer := new(big.Int)
	if _, ok := integer.SetString(value, 10); ok {
		return integer.String()
	}
	return value
}

func normalizeDecimalDefault(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "'")
	rational := new(big.Rat)
	if _, ok := rational.SetString(value); ok {
		return rational.RatString()
	}
	return value
}

func defaultsMatchColumn(field crudmodel.Field, actual sql.NullString) bool {
	switch strings.ToUpper(strings.TrimSpace(field.DefaultType)) {
	case "INPUT":
		return actual.Valid && normalizeDefaultValueForField(field, actual.String) == normalizeDefaultValueForField(field, field.Default)
	case "EMPTY STRING":
		return actual.Valid && actual.String == ""
	default:
		// NONE / NULL / 未指定：information_schema 对"无默认值"和 DEFAULT NULL 都报 NULL，
		// 无法区分，宽容视为一致；列上存在默认值才算漂移
		return !actual.Valid
	}
}

func isIntegerType(base string) bool {
	switch base {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return true
	default:
		return false
	}
}

func typeParts(value string) (base string, args []string) {
	value = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(value, " unsigned")))
	open := strings.IndexByte(value, '(')
	if open < 0 {
		return value, nil
	}
	base = strings.TrimSpace(value[:open])
	close := strings.LastIndexByte(value, ')')
	if close < open {
		return base, nil
	}
	for _, arg := range strings.Split(value[open+1:close], ",") {
		args = append(args, strings.Trim(strings.TrimSpace(arg), "'"))
	}
	return base, args
}

func columnUnsigned(column model.Column) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(column.COLUMN_TYPE)), " unsigned")
}

func fieldTypeChanged(field crudmodel.Field, column model.Column) bool {
	return normalizeColumnType(specColumnType(field)) != normalizeColumnType(column.COLUMN_TYPE)
}

func nullableChanged(field crudmodel.Field, column model.Column) bool {
	want := "NO"
	if specFieldNullable(field) {
		want = "YES"
	}
	return !strings.EqualFold(column.IS_NULLABLE, want)
}

func autoIncrementChanged(field crudmodel.Field, column model.Column) bool {
	want := field.AutoIncrement && field.PrimaryKey
	has := strings.Contains(strings.ToLower(column.EXTRA), "auto_increment")
	return want != has
}

func defaultChanged(field crudmodel.Field, column model.Column) bool {
	return !defaultsMatchColumn(field, column.COLUMN_DEFAULT)
}

func enumSetMembers(value string) []string {
	_, args := typeParts(value)
	return args
}

var integerTypeRank = map[string]int{"tinyint": 1, "smallint": 2, "mediumint": 3, "int": 4, "integer": 4, "bigint": 5}
var textTypeRank = map[string]int{"tinytext": 1, "text": 2, "mediumtext": 3, "longtext": 4}

// explicitWideningMatrix is deliberately narrow. A type change outside this
// matrix is rejected instead of guessing that MySQL will preserve data.
func explicitWideningMatrix(actual, desired string) bool {
	actualBase, actualArgs := typeParts(actual)
	desiredBase, desiredArgs := typeParts(desired)
	if actualBase == desiredBase {
		switch actualBase {
		case "varchar", "char", "varbinary", "binary":
			return len(actualArgs) == 1 && len(desiredArgs) == 1 && integerValue(desiredArgs[0]) > integerValue(actualArgs[0])
		case "decimal", "numeric":
			return len(actualArgs) == 2 && len(desiredArgs) == 2 && integerValue(desiredArgs[0]) >= integerValue(actualArgs[0]) && integerValue(desiredArgs[1]) >= integerValue(actualArgs[1]) && (actualArgs[0] != desiredArgs[0] || actualArgs[1] != desiredArgs[1])
		case "enum", "set":
			return membersContain(enumSetMembers(desired), enumSetMembers(actual)) && len(enumSetMembers(desired)) > len(enumSetMembers(actual))
		}
	}
	if actualRank, ok := integerTypeRank[actualBase]; ok {
		if desiredRank, ok := integerTypeRank[desiredBase]; ok {
			return desiredRank > actualRank
		}
	}
	if actualRank, ok := textTypeRank[actualBase]; ok {
		if desiredRank, ok := textTypeRank[desiredBase]; ok {
			return desiredRank > actualRank
		}
	}
	return false
}

func integerValue(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func membersContain(have, want []string) bool {
	set := make(map[string]bool, len(have))
	for _, value := range have {
		set[value] = true
	}
	for _, value := range want {
		if !set[value] {
			return false
		}
	}
	return true
}

func typeNarrowing(actual, desired string) bool {
	if explicitWideningMatrix(actual, desired) {
		return false
	}
	actualBase, actualArgs := typeParts(actual)
	desiredBase, desiredArgs := typeParts(desired)
	if actualBase == desiredBase {
		switch actualBase {
		case "varchar", "char", "varbinary", "binary":
			return len(actualArgs) == 1 && len(desiredArgs) == 1 && integerValue(desiredArgs[0]) < integerValue(actualArgs[0])
		case "decimal", "numeric":
			return len(actualArgs) == 2 && len(desiredArgs) == 2 && (integerValue(desiredArgs[0]) < integerValue(actualArgs[0]) || integerValue(desiredArgs[1]) < integerValue(actualArgs[1]))
		case "enum", "set":
			return !membersContain(enumSetMembers(desired), enumSetMembers(actual))
		}
	}
	if actualRank, ok := integerTypeRank[actualBase]; ok {
		if desiredRank, ok := integerTypeRank[desiredBase]; ok {
			return desiredRank < actualRank
		}
	}
	if actualRank, ok := textTypeRank[actualBase]; ok {
		if desiredRank, ok := textTypeRank[desiredBase]; ok {
			return desiredRank < actualRank
		}
	}
	return true
}

func unmanagedColumnAttributes(column model.Column) []string {
	attributes := make([]string, 0, 4)
	if column.CHARACTER_SET_NAME != "" {
		attributes = append(attributes, "charset")
	}
	if column.COLLATION_NAME != "" {
		attributes = append(attributes, "collation")
	}
	if column.GENERATION_EXPRESSION != "" {
		attributes = append(attributes, "generated-expression")
	}
	extra := strings.TrimSpace(strings.ReplaceAll(strings.ToLower(column.EXTRA), "auto_increment", ""))
	if extra != "" {
		attributes = append(attributes, "extra:"+extra)
	}
	return attributes
}

func riskForNewField(field crudmodel.Field) (DiffClass, string) {
	if field.PrimaryKey && field.AutoIncrement {
		return DiffSafeAuto, "new auto-increment primary key column"
	}
	if field.Null || strings.EqualFold(field.DefaultType, "NULL") {
		return DiffSafeAuto, "new nullable column"
	}
	switch strings.ToUpper(strings.TrimSpace(field.DefaultType)) {
	case "INPUT", "EMPTY STRING":
		if !legalDefaultForNewField(field) {
			return DiffRejected, "new NOT NULL column has no legal default for its type"
		}
		return DiffSafeAuto, "new NOT NULL column has a default"
	default:
		return DiffRejected, "new NOT NULL column has no legal default"
	}
}

func legalDefaultForNewField(field crudmodel.Field) bool {
	base := strings.ToLower(analyseFieldTypeForSpec(field))
	if noDefaultValueType(base) {
		return false
	}
	if strings.EqualFold(field.DefaultType, "EMPTY STRING") {
		return base == "varchar" || base == "char" || base == "enum" || base == "set"
	}
	if isIntegerType(base) {
		_, ok := new(big.Int).SetString(strings.Trim(strings.TrimSpace(field.Default), "'"), 10)
		return ok
	}
	if base == "decimal" || base == "numeric" {
		_, ok := new(big.Rat).SetString(strings.Trim(strings.TrimSpace(field.Default), "'"))
		return ok
	}
	return field.Default != ""
}

func classifyFieldDiff(field crudmodel.Field, column model.Column, primary bool) (DiffClass, ApprovalCategory, string) {
	typeChanged := fieldTypeChanged(field, column)
	unsignedChanged := field.Unsigned != columnUnsigned(column)
	nullableChangedValue := nullableChanged(field, column)
	defaultChangedValue := defaultChanged(field, column)
	autoChanged := autoIncrementChanged(field, column)
	commentChanged := column.COLUMN_COMMENT != field.Comment
	if primary && (typeChanged || unsignedChanged || nullableChangedValue || autoChanged || defaultChangedValue) {
		return DiffRejected, "", "primary key column attribute drift requires a business migration"
	}
	if unsignedChanged {
		return DiffRejected, "", "unsigned attribute changes are rejected"
	}
	if nullableChangedValue && strings.EqualFold(column.IS_NULLABLE, "YES") && !specFieldNullable(field) {
		return DiffRejected, "", "nullable to NOT NULL may reject existing rows"
	}
	if typeChanged {
		actual := column.COLUMN_TYPE
		desired := specColumnType(field)
		if typeNarrowing(actual, desired) {
			return DiffRejected, "", "type narrowing may truncate existing data"
		}
		if explicitWideningMatrix(actual, desired) {
			return DiffRequiresApproval, ApprovalTypeWidening, "type widening is covered by the explicit safety matrix"
		}
		return DiffRejected, "", "type change is outside the explicit safety matrix"
	}
	if defaultChangedValue {
		return DiffRequiresApproval, ApprovalDefaults, "default value change requires approval"
	}
	if autoChanged {
		return DiffRequiresApproval, ApprovalAutoIncrement, "auto_increment change requires approval"
	}
	if commentChanged {
		if primary {
			return DiffSafeAuto, "", "comment-only change on primary key"
		}
		return DiffSafeAuto, "", "comment-only change"
	}
	if nullableChangedValue {
		return DiffSafeAuto, "", "nullable expansion"
	}
	return DiffRequiresApproval, ApprovalAttributes, "column attribute change requires approval"
}

// specFieldMatchesColumn 判断 spec 字段与实际列是否完全一致（无漂移）。
func specFieldMatchesColumn(field crudmodel.Field, column model.Column) bool {
	if fieldTypeChanged(field, column) {
		return false
	}
	if nullableChanged(field, column) {
		return false
	}
	if defaultChanged(field, column) {
		return false
	}
	if field.Unsigned != columnUnsigned(column) {
		return false
	}
	if autoIncrementChanged(field, column) {
		return false
	}
	return column.COLUMN_COMMENT == field.Comment
}

func deriveAlterDiff(columns []model.Column, fields []crudmodel.Field) []AlterDiff {
	existing := make(map[string]model.Column, len(columns))
	for _, column := range columns {
		existing[strings.ToLower(column.COLUMN_NAME)] = column
	}
	primary := make(map[string]bool)
	for _, field := range fields {
		if field.PrimaryKey {
			primary[strings.ToLower(field.Name)] = true
		}
	}
	changes := make([]AlterDiff, 0, len(fields))
	for _, field := range fields {
		column, ok := existing[strings.ToLower(field.Name)]
		if !ok {
			class, reason := riskForNewField(field)
			changes = append(changes, AlterDiff{
				Change: crudmodel.ChangeField{Type: "add-field", OldName: field.Name, NewName: field.Name, Sync: class == DiffSafeAuto, Risk: string(class), Reason: reason},
				Field:  field, Class: class, Reason: reason,
			})
			continue
		}
		if specFieldMatchesColumn(field, column) {
			continue
		}
		class, category, reason := classifyFieldDiff(field, column, primary[strings.ToLower(field.Name)])
		changes = append(changes, AlterDiff{
			Change: crudmodel.ChangeField{Type: "change-field-attr", OldName: field.Name, NewName: field.Name, Sync: class == DiffSafeAuto, Risk: string(class), Reason: reason},
			Field:  field, Column: &column, Class: class, Category: category, Reason: reason, Unmanaged: unmanagedColumnAttributes(column),
		})
	}
	return changes
}
