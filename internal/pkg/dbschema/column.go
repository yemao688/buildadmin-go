// Package dbschema 提供数据库元数据的纯类型，供生成器与渠道仓库共用。
// 它只依赖标准库，不依赖任何渠道或 GORM，避免生成器为了一个列结构反向
// import admin/repository。
package dbschema

import "database/sql"

// Column 是 information_schema.columns 的一行。
type Column struct {
	COLUMN_NAME           string
	COLUMN_COMMENT        string
	IS_NULLABLE           string
	COLUMN_TYPE           string
	DATA_TYPE             string
	COLUMN_DEFAULT        sql.NullString
	COLUMN_KEY            string
	EXTRA                 string
	CHARACTER_SET_NAME    string
	COLLATION_NAME        string
	GENERATION_EXPRESSION string
}
