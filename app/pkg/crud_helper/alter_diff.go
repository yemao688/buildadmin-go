package crud_helper

import (
	"database/sql"
	"go-build-admin/app/admin/model"
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

// specColumnType 渲染 spec 字段对应的 MySQL COLUMN_TYPE（与 getDDlFieldData 同一类型来源）。
func specColumnType(field model.Field) string {
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

func specFieldNullable(field model.Field) bool {
	return field.Null || strings.EqualFold(field.DefaultType, "NULL")
}

// normalizeDefaultValue 去引号并按数值修剪尾零：'1'、1、1.00000000 视为同一默认值。
func normalizeDefaultValue(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.Trim(normalized, "'")
	if numeric, err := strconv.ParseFloat(normalized, 64); err == nil {
		return strconv.FormatFloat(numeric, 'f', -1, 64)
	}
	return normalized
}

func defaultsMatchColumn(field model.Field, actual sql.NullString) bool {
	switch strings.ToUpper(strings.TrimSpace(field.DefaultType)) {
	case "INPUT":
		return actual.Valid && normalizeDefaultValue(actual.String) == normalizeDefaultValue(field.Default)
	case "EMPTY STRING":
		return actual.Valid && actual.String == ""
	default:
		// NONE / NULL / 未指定：information_schema 对"无默认值"和 DEFAULT NULL 都报 NULL，
		// 无法区分，宽容视为一致；列上存在默认值才算漂移
		return !actual.Valid
	}
}

// specFieldMatchesColumn 判断 spec 字段与实际列是否完全一致（无漂移）。
func specFieldMatchesColumn(field model.Field, column model.Column) bool {
	if normalizeColumnType(specColumnType(field)) != normalizeColumnType(column.COLUMN_TYPE) {
		return false
	}
	wantNullable := "NO"
	if specFieldNullable(field) {
		wantNullable = "YES"
	}
	if !strings.EqualFold(column.IS_NULLABLE, wantNullable) {
		return false
	}
	if !defaultsMatchColumn(field, column.COLUMN_DEFAULT) {
		return false
	}
	wantAutoIncrement := field.AutoIncrement && field.PrimaryKey
	hasAutoIncrement := strings.Contains(strings.ToLower(column.EXTRA), "auto_increment")
	if wantAutoIncrement != hasAutoIncrement {
		return false
	}
	return column.COLUMN_COMMENT == field.Comment
}
