package local

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func addTargetOwnerColumnAndIndex(db *gorm.DB, table string) error {
	exists, err := core.LegacyTableExists(db, table)
	if err != nil || !exists {
		return err
	}
	def, ok, err := core.MigrationColumnInfo(db, table, "target_admin_id")
	if err != nil {
		return err
	}
	if !ok {
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(table) + " ADD COLUMN `target_admin_id` int(11) unsigned NOT NULL DEFAULT 0 COMMENT '目标数据管理员ID'").Error; err != nil {
			return err
		}
		def, ok, err = core.MigrationColumnInfo(db, table, "target_admin_id")
		if err != nil {
			return err
		}
	}
	if !ok {
		return fmt.Errorf("%s.target_admin_id was not created", table)
	}
	if err := validOwnerColumn(def, table+".target_admin_id"); err != nil {
		return err
	}
	has, first, err := core.MigrationIndexInfo(db, table, "idx_target_admin_id")
	if err != nil {
		return fmt.Errorf("inspect idx_target_admin_id on %s: %w", table, err)
	}
	if has {
		if first != "target_admin_id" {
			return fmt.Errorf("idx_target_admin_id on %s starts with %q, want target_admin_id", table, first)
		}
		return nil
	}
	return db.Exec("CREATE INDEX `idx_target_admin_id` ON " + core.QuoteIdentifier(table) + " (`target_admin_id`)").Error
}

func parseTargetAdminID(raw string) (int32, bool) {
	var data map[string]json.RawMessage
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &data) != nil {
		return 0, false
	}
	value, ok := data["admin_id"]
	if !ok {
		return 0, false
	}
	var id int64
	if json.Unmarshal(value, &id) != nil || id <= 0 || id > math.MaxInt32 {
		return 0, false
	}
	return int32(id), true
}

func targetAdminExists(db *gorm.DB, adminTable string, id int32) (bool, error) {
	var count int64
	if err := db.Table(adminTable).Where("id=?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 1, nil
}

func validateTargetOwners(db *gorm.DB, table, adminTable string) error {
	var nonzero int64
	if err := db.Table(table).Where("target_admin_id<>0").Count(&nonzero).Error; err != nil {
		return err
	}
	if nonzero == 0 {
		return nil
	}
	if ok, err := core.LegacyTableExists(db, adminTable); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("admin table %s does not exist for nonzero target_admin_id values", adminTable)
	}
	var invalid int64
	if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(table) + " t LEFT JOIN " + core.QuoteIdentifier(adminTable) + " a ON a.id=t.target_admin_id WHERE t.target_admin_id<>0 AND a.id IS NULL").Scan(&invalid).Error; err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("%s contains invalid target_admin_id values", table)
	}
	return nil
}

func backfillTargetOwnerFromJSON(db *gorm.DB, table, jsonColumn, adminTable string) error {
	var rows []struct {
		ID   int32  `gorm:"column:id"`
		JSON string `gorm:"column:payload"`
	}
	if err := db.Table(table).Select("id, " + core.QuoteIdentifier(jsonColumn) + " AS payload").Where("target_admin_id=0").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		candidate, ok := parseTargetAdminID(row.JSON)
		if !ok {
			continue
		}
		exists, err := targetAdminExists(db, adminTable, candidate)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if err := db.Table(table).Where("id=? AND target_admin_id=0", row.ID).Update("target_admin_id", candidate).Error; err != nil {
			return err
		}
	}
	return nil
}

func addLegacyTargetFlagColumn(db *gorm.DB, table string) error {
	if !core.TableExists(db, table) {
		return nil
	}
	def, ok, err := core.MigrationColumnInfo(db, table, "legacy_unrecoverable")
	if err != nil {
		return err
	}
	if !ok {
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(table) + " ADD COLUMN `legacy_unrecoverable` tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '历史目标管理员不可恢复'").Error; err != nil {
			return err
		}
		def, ok, err = core.MigrationColumnInfo(db, table, "legacy_unrecoverable")
		if err != nil {
			return err
		}
	}
	if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "tinyint") || !strings.Contains(strings.ToLower(def.ColumnType), "unsigned") || !strings.EqualFold(def.Nullable, "NO") || def.Default == nil || *def.Default != "0" {
		return fmt.Errorf("%s.legacy_unrecoverable has invalid schema", table)
	}
	return nil
}

func addCommittedColumn(db *gorm.DB, table string) error {
	if !core.TableExists(db, table) {
		return nil
	}
	def, ok, err := core.MigrationColumnInfo(db, table, "is_committed")
	if err != nil {
		return fmt.Errorf("inspect %s.is_committed: %w", table, err)
	}
	if !ok {
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(table) + " ADD COLUMN `is_committed` tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '提交状态'").Error; err != nil {
			return fmt.Errorf("add is_committed to %s: %w", table, err)
		}
		def, ok, err = core.MigrationColumnInfo(db, table, "is_committed")
		if err != nil {
			return fmt.Errorf("inspect %s.is_committed after add: %w", table, err)
		}
	}
	if !ok {
		return fmt.Errorf("%s.is_committed was not created", table)
	}
	typ := strings.ToLower(def.ColumnType)
	if !strings.Contains(typ, "tinyint") || !strings.Contains(typ, "unsigned") || !strings.EqualFold(def.Nullable, "NO") || def.Default == nil || *def.Default != "0" {
		return fmt.Errorf("%s.is_committed has invalid schema", table)
	}
	return nil
}
