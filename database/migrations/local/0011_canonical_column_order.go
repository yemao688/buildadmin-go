package local

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

type canonicalColumn struct {
	table, column, after, definition          string
	ordinal                                   int
	typeName, nullable, defaultValue, comment string
}

var canonicalColumns = []canonicalColumn{
	{table: "admin", column: "parent_id", after: "id", ordinal: 2, definition: "int(11) unsigned DEFAULT NULL COMMENT '父级管理员ID'", typeName: "int(11) unsigned", nullable: "YES", comment: "父级管理员ID"},
	{table: "attachment", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '上传管理员ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "上传管理员ID"},
	{table: "attachment", column: "user_id", after: "admin_id", ordinal: 3, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '上传用户ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "上传用户ID"},
	{table: "attachment", column: "topic", after: "user_id", ordinal: 4, definition: "varchar(20) NOT NULL DEFAULT '' COMMENT '细目'", typeName: "varchar(20)", nullable: "NO", defaultValue: "", comment: "细目"},
	{table: "user", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "管理员ID"},
	{table: "user_money_log", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "管理员ID"},
	{table: "user_score_log", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "管理员ID"},
	{table: "admin_log", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "管理员ID"},
	{table: "security_data_recycle", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "管理员ID"},
	{table: "security_sensitive_data", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "管理员ID"},
	{table: "crud_log", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "管理员ID"},
	{table: "security_data_recycle_log", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '操作管理员'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "操作管理员"},
	{table: "security_data_recycle_log", column: "target_admin_id", after: "admin_id", ordinal: 3, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '目标数据管理员'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "目标数据管理员"},
	{table: "security_data_recycle_log", column: "legacy_unrecoverable", after: "target_admin_id", ordinal: 4, definition: "tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '历史目标管理员不可恢复'", typeName: "tinyint(1) unsigned", nullable: "NO", defaultValue: "0", comment: "历史目标管理员不可恢复"},
	{table: "security_data_recycle_log", column: "is_committed", after: "legacy_unrecoverable", ordinal: 5, definition: "tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '提交状态'", typeName: "tinyint(1) unsigned", nullable: "NO", defaultValue: "0", comment: "提交状态"},
	{table: "security_sensitive_data_log", column: "admin_id", after: "id", ordinal: 2, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '操作管理员'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "操作管理员"},
	{table: "security_sensitive_data_log", column: "target_admin_id", after: "admin_id", ordinal: 3, definition: "int(11) unsigned NOT NULL DEFAULT 0 COMMENT '目标数据管理员'", typeName: "int(11) unsigned", nullable: "NO", defaultValue: "0", comment: "目标数据管理员"},
	{table: "security_sensitive_data_log", column: "legacy_unrecoverable", after: "target_admin_id", ordinal: 4, definition: "tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '历史目标管理员不可恢复'", typeName: "tinyint(1) unsigned", nullable: "NO", defaultValue: "0", comment: "历史目标管理员不可恢复"},
	{table: "security_sensitive_data_log", column: "is_committed", after: "legacy_unrecoverable", ordinal: 5, definition: "tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '提交状态'", typeName: "tinyint(1) unsigned", nullable: "NO", defaultValue: "0", comment: "提交状态"},
}

func version0011(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	for _, spec := range canonicalColumns {
		if err := applyCanonicalColumn(db, config, spec); err != nil {
			return err
		}
	}
	return nil
}

func applyCanonicalColumn(db *gorm.DB, config *conf.Configuration, spec canonicalColumn) error {
	fullTable := core.TableName(config, spec.table)
	definition, ok, err := core.MigrationColumnInfo(db, fullTable, spec.column)
	if err != nil {
		return fmt.Errorf("inspect canonical column %s.%s: %w", fullTable, spec.column, err)
	}
	if !ok {
		return fmt.Errorf("required canonical column %s.%s is missing", fullTable, spec.column)
	}
	if !canonicalDefinitionSafe(definition, spec) {
		return fmt.Errorf("canonical column %s.%s has unsafe definition", fullTable, spec.column)
	}
	if definition.Ordinal == spec.ordinal {
		return nil
	}
	if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(fullTable) + " MODIFY COLUMN " + core.QuoteIdentifier(spec.column) + " " + spec.definition + " AFTER " + core.QuoteIdentifier(spec.after)).Error; err != nil {
		return fmt.Errorf("canonicalize column %s.%s: %w", fullTable, spec.column, err)
	}
	return nil
}

func verifyCanonicalColumnOrder(db *gorm.DB, config *conf.Configuration) error {
	if err := verifySecurityRuleContract(db, config); err != nil {
		return err
	}
	for _, spec := range canonicalColumns {
		definition, ok, err := core.MigrationColumnInfo(db, core.TableName(config, spec.table), spec.column)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("required canonical column %s.%s is missing", spec.table, spec.column)
		}
		if definition.Ordinal != spec.ordinal || !canonicalDefinitionSafe(definition, spec) {
			return fmt.Errorf("canonical column %s.%s definition or ordinal mismatch", spec.table, spec.column)
		}
	}
	return nil
}

func canonicalDefinitionSafe(definition core.MigrationColumn, spec canonicalColumn) bool {
	if normalizeCanonicalType(definition.ColumnType) != normalizeCanonicalType(spec.typeName) || !strings.EqualFold(definition.Nullable, spec.nullable) || !canonicalCommentAllowed(definition.Comment, spec) {
		return false
	}
	if spec.defaultValue == "" && spec.typeName == "int(11) unsigned" && spec.table == "admin" {
		return definition.Default == nil
	}
	return definition.Default != nil && *definition.Default == spec.defaultValue
}

func canonicalCommentAllowed(comment string, spec canonicalColumn) bool {
	if comment == spec.comment {
		return true
	}
	if spec.table == "security_data_recycle_log" || spec.table == "security_sensitive_data_log" {
		if spec.column == "admin_id" {
			return comment == "管理员ID" || comment == "操作管理员"
		}
		if spec.column == "target_admin_id" {
			return comment == "目标数据管理员ID" || comment == "目标数据管理员"
		}
	}
	return false
}

func normalizeCanonicalType(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "int(11)", "int")
	value = strings.ReplaceAll(value, "tinyint(1)", "tinyint")
	return value
}
