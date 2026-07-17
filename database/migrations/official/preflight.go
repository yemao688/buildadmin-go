package official

import (
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

type legacyColumn struct{ table, old, new, typ string }

var version200Columns = []legacyColumn{
	{"admin", "loginfailure", "login_failure", "TINYINT(4) UNSIGNED NOT NULL DEFAULT 0"},
	{"admin", "lastlogintime", "last_login_time", "BIGINT(16) UNSIGNED NULL"}, {"admin", "lastloginip", "last_login_ip", "VARCHAR(50) NOT NULL DEFAULT ''"},
	{"admin", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"}, {"admin", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"admin_group", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"}, {"admin_group", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"admin_log", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"attachment", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"}, {"attachment", "lastuploadtime", "last_upload_time", "BIGINT(16) UNSIGNED NULL"},
	{"captcha", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"}, {"captcha", "expiretime", "expire_time", "BIGINT(16) UNSIGNED NULL"},
	{"menu_rule", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"}, {"menu_rule", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"admin_rule", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"}, {"admin_rule", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"security_data_recycle", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"}, {"security_data_recycle", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"security_data_recycle_log", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"security_sensitive_data", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"}, {"security_sensitive_data", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"security_sensitive_data_log", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"token", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"}, {"token", "expiretime", "expire_time", "BIGINT(16) UNSIGNED NULL"},
	{"user_group", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"}, {"user_group", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"user_money_log", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"}, {"user_rule", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"},
	{"user_rule", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"}, {"user_score_log", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
	{"user", "lastlogintime", "last_login_time", "BIGINT(16) UNSIGNED NULL"}, {"user", "lastloginip", "last_login_ip", "VARCHAR(50) NOT NULL DEFAULT ''"},
	{"user", "loginfailure", "login_failure", "TINYINT(4) UNSIGNED NOT NULL DEFAULT 0"}, {"user", "joinip", "join_ip", "VARCHAR(50) NOT NULL DEFAULT ''"},
	{"user", "jointime", "join_time", "BIGINT(16) UNSIGNED NULL"}, {"user", "updatetime", "update_time", "BIGINT(16) UNSIGNED NULL"}, {"user", "createtime", "create_time", "BIGINT(16) UNSIGNED NULL"},
}

func menuRuleBackupName(config *conf.Configuration) string {
	return core.TableName(config, "menu_rule_version200_backup")
}

func normalizeLegacyColumns(db *gorm.DB, config *conf.Configuration, onlyTable string) error {
	for _, item := range version200Columns {
		if onlyTable != "" && item.table != onlyTable {
			continue
		}
		t := core.TableName(config, item.table)
		if !core.TableExists(db, t) || !core.ColumnExists(db, t, item.old) {
			continue
		}
		if core.ColumnExists(db, t, item.new) {
			var n int64
			q := "SELECT COUNT(*) FROM `" + t + "` WHERE `" + item.old + "` IS NOT NULL AND `" + item.old + "` <> '' AND `" + item.new + "` IS NOT NULL AND `" + item.new + "` <> '' AND NOT (`" + item.old + "` <=> `" + item.new + "`)"
			if err := db.Raw(q).Scan(&n).Error; err != nil {
				return err
			}
			if n != 0 {
				return fmt.Errorf("conflicting columns %s.%s and %s", t, item.old, item.new)
			}
			if err := db.Exec("UPDATE `" + t + "` SET `" + item.new + "`=`" + item.old + "` WHERE (`" + item.new + "` IS NULL OR `" + item.new + "`='') AND `" + item.old + "` IS NOT NULL").Error; err != nil {
				return err
			}
			if err := db.Exec("ALTER TABLE `" + t + "` DROP COLUMN `" + item.old + "`").Error; err != nil {
				return err
			}
		} else if err := db.Exec("ALTER TABLE `" + t + "` CHANGE COLUMN `" + item.old + "` `" + item.new + "` " + item.typ).Error; err != nil {
			return err
		}
	}
	return nil
}

func PrepareUpstreamNeutralSchema(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	// Normalize both rule table variants first; this also makes a later merge
	// compare the same logical columns.
	if err := normalizeLegacyColumns(db, config, "menu_rule"); err != nil {
		return err
	}
	if err := normalizeLegacyColumns(db, config, "admin_rule"); err != nil {
		return err
	}
	admin := core.TableName(config, "admin_rule")
	menu := core.TableName(config, "menu_rule")
	if core.TableExists(db, menu) {
		backup := menuRuleBackupName(config)
		if core.TableExists(db, backup) {
			return fmt.Errorf("backup table %s already exists while menu_rule is present; refusing to overwrite", backup)
		}
		if !core.TableExists(db, admin) {
			if err := db.Exec("RENAME TABLE `" + menu + "` TO `" + admin + "`").Error; err != nil {
				return fmt.Errorf("rename menu_rule: %w", err)
			}
		} else {
			// IDs are stable and are also referenced by admin_group.rules.  Never
			// use INSERT IGNORE here: an equal ID with different data is unsafe.
			cols := []string{"id", "pid", "type", "title", "name", "path", "icon", "menu_type", "url", "component", "keepalive", "extend", "remark", "weigh", "status", "update_time", "create_time"}
			var actual []string
			if err := db.Raw("SELECT column_name FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=?", menu).Pluck("column_name", &actual).Error; err != nil {
				return err
			}
			allowed := map[string]bool{}
			for _, c := range cols {
				allowed[c] = true
			}
			for _, c := range actual {
				if !allowed[c] {
					return fmt.Errorf("menu_rule has unknown column %s; source table retained", c)
				}
			}
			for _, c := range cols {
				if core.ColumnExists(db, menu, c) && core.ColumnExists(db, admin, c) {
					var n int64
					q := "SELECT COUNT(*) FROM `" + menu + "` m JOIN `" + admin + "` a ON m.id=a.id WHERE NOT (m.`" + c + "` <=> a.`" + c + "`)"
					if err := db.Raw(q).Scan(&n).Error; err != nil {
						return err
					}
					if n != 0 {
						return fmt.Errorf("menu_rule/admin_rule conflict in column %s", c)
					}
				}
			}
			var rows []map[string]any
			if err := db.Table(menu).Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				id, ok := row["id"]
				if !ok {
					continue
				}
				var exists int64
				if err := db.Table(admin).Where("id = ?", id).Count(&exists).Error; err != nil {
					return err
				}
				if exists == 0 {
					fields := map[string]any{}
					for _, c := range cols {
						if v, ok := row[c]; ok {
							fields[c] = v
						}
					}
					if err := db.Table(admin).Create(fields).Error; err != nil {
						return fmt.Errorf("merge menu_rule id %v: %w", id, err)
					}
				}
			}
			var broken int64
			if err := db.Raw("SELECT COUNT(*) FROM `" + menu + "` m WHERE m.pid <> 0 AND NOT EXISTS (SELECT 1 FROM `" + admin + "` a WHERE a.id=m.pid)").Scan(&broken).Error; err != nil {
				return err
			}
			if broken != 0 {
				return fmt.Errorf("menu_rule contains unresolved pid references")
			}
			if err := db.Exec("RENAME TABLE " + core.QuoteIdentifier(menu) + " TO " + core.QuoteIdentifier(backup)).Error; err != nil {
				return fmt.Errorf("backup menu_rule: %w", err)
			}
		}
	}
	if err := normalizeLegacyColumns(db, config, ""); err != nil {
		return err
	}
	for _, item := range []struct{ table, column, typ string }{
		{"admin_log", "data", "LONGTEXT"}, {"captcha", "captcha", "TEXT"},
	} {
		t := core.TableName(config, item.table)
		if core.TableExists(db, t) && core.ColumnExists(db, t, item.column) {
			if err := db.Exec("ALTER TABLE `" + t + "` MODIFY COLUMN `" + item.column + "` " + item.typ).Error; err != nil {
				return fmt.Errorf("alter %s.%s: %w", t, item.column, err)
			}
		}
	}
	return nil
}
