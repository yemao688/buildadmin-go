package local

import (
	"database/sql"
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

const userMoneyDecimalType = "decimal(12,2)"

var userMoneyDecimalColumns = []struct {
	table  string
	column string
}{
	{table: "user", column: "money"},
	{table: "user_money_log", column: "money"},
	{table: "user_money_log", column: "before"},
	{table: "user_money_log", column: "after"},
}

func userMoneyDecimalUp(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	for _, item := range userMoneyDecimalColumns {
		if err := convertUserMoneyColumn(db, config, item.table, item.column); err != nil {
			return err
		}
	}
	return nil
}

func convertUserMoneyColumn(db *gorm.DB, config *conf.Configuration, logicalTable, column string) error {
	table := core.TableName(config, logicalTable)
	convert := func(conn *gorm.DB) error {
		definition, ok, err := core.MigrationColumnInfo(conn, table, column)
		if err != nil {
			return fmt.Errorf("inspect %s.%s: %w", table, column, err)
		}
		if !ok {
			return fmt.Errorf("missing column %s.%s", table, column)
		}
		columnType := strings.ToLower(strings.TrimSpace(definition.ColumnType))
		if columnType == userMoneyDecimalType {
			return nil
		}
		baseType := userMoneyColumnBaseType(columnType)
		convertCents := false
		switch baseType {
		case "int", "tinyint", "smallint", "mediumint", "bigint":
			convertCents = true
		case "decimal", "double", "float":
			// Business repositories may have already converted this column to yuan;
			// this path only normalizes their decimal precision and must not divide.
		default:
			return fmt.Errorf("%s.%s has unexpected money column type %q", table, column, columnType)
		}

		quotedTable := core.QuoteIdentifier(table)
		quotedColumn := core.QuoteIdentifier(column)
		comment := strings.ReplaceAll(definition.Comment, "'", "''")
		if err := conn.Exec("ALTER TABLE " + quotedTable + " MODIFY COLUMN " + quotedColumn + " " + userMoneyDecimalType + " NOT NULL DEFAULT 0.00 COMMENT '" + comment + "'").Error; err != nil {
			return fmt.Errorf("convert %s.%s to %s: %w", table, column, userMoneyDecimalType, err)
		}
		if !convertCents {
			return nil
		}
		// ALTER and UPDATE are intentionally adjacent on the same connection. If
		// the process dies in this sub-millisecond window, the type guard skips
		// the column on retry and its data remains in cents; repair it manually by
		// dividing that column by 100 before rerunning the migration.
		if err := conn.Exec("UPDATE " + quotedTable + " SET " + quotedColumn + " = " + quotedColumn + " / 100").Error; err != nil {
			return fmt.Errorf("convert %s.%s data from cents to yuan: %w", table, column, err)
		}
		return nil
	}

	// gorm's Connection() resolves a *sql.DB from the handle and returns
	// ErrInvalidDB for any other ConnPool — e.g. the *sql.Conn pinned by the
	// migration advisory lock. A pinned handle already executes every
	// statement on that one connection, so only route through Connection()
	// when this handle wraps a pool.
	if _, ok := db.Statement.ConnPool.(*sql.DB); ok {
		return db.Connection(convert)
	}
	return convert(db)
}

func userMoneyColumnBaseType(columnType string) string {
	fields := strings.Fields(columnType)
	if len(fields) == 0 {
		return ""
	}
	baseType := fields[0]
	if index := strings.IndexByte(baseType, '('); index >= 0 {
		baseType = baseType[:index]
	}
	return strings.TrimSpace(baseType)
}

func verifyUserMoneyDecimalBaseline(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	for _, item := range userMoneyDecimalColumns {
		table := core.TableName(config, item.table)
		definition, ok, err := core.MigrationColumnInfo(db, table, item.column)
		if err != nil {
			return fmt.Errorf("inspect %s.%s: %w", table, item.column, err)
		}
		if !ok {
			return fmt.Errorf("missing column %s.%s", table, item.column)
		}
		columnType := strings.ToLower(strings.TrimSpace(definition.ColumnType))
		if columnType != userMoneyDecimalType {
			return fmt.Errorf("%s.%s has type %q, want %s", table, item.column, columnType, userMoneyDecimalType)
		}
	}
	return nil
}
