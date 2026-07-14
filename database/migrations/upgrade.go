package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/model"
	"gorm.io/gorm"
)

var safePrefix = regexp.MustCompile(`^[A-Za-z0-9_]*$`)

const installDataVersion int64 = 20230620180916
const installDataName = "InstallData"

func ValidatePrefix(config *conf.Configuration) error {
	if config == nil || !safePrefix.MatchString(config.Database.Prefix) {
		return fmt.Errorf("invalid database table prefix")
	}
	return nil
}

func quoteIdentifier(value string) string { return "`" + strings.ReplaceAll(value, "`", "``") + "`" }

func menuRuleBackupName(config *conf.Configuration) string {
	return tableName(config, "menu_rule_version200_backup")
}

func installDataTable(config *conf.Configuration) string {
	return quoteIdentifier(tableName(config, "migrations"))
}

func MarkSeedPending(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	table := tableName(config, "migrations")
	var record migrationRecord
	result := db.Table(table).Where("version = ?", installDataVersion).First(&record)
	if result.Error == nil {
		if record.MigrationName != installDataName {
			return fmt.Errorf("migration version %d name collision", installDataVersion)
		}
		if record.EndTime == nil {
			return nil
		}
		return nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}
	now := time.Now()
	return db.Exec("INSERT INTO "+installDataTable(config)+" (version, migration_name, start_time, end_time, breakpoint) VALUES (?, ?, ?, NULL, ?)", installDataVersion, installDataName, now, false).Error
}

func SeedPending(db *gorm.DB, config *conf.Configuration) (bool, error) {
	if err := ValidatePrefix(config); err != nil {
		return false, err
	}
	var record migrationRecord
	result := db.Table(tableName(config, "migrations")).Where("version = ?", installDataVersion).First(&record)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if result.Error != nil {
		return false, result.Error
	}
	if record.MigrationName != installDataName {
		return false, fmt.Errorf("migration version %d name collision", installDataVersion)
	}
	return record.EndTime == nil, nil
}

func MarkSeedCompleted(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	now := time.Now()
	result := db.Table(tableName(config, "migrations")).Where("version = ? AND migration_name = ?", installDataVersion, installDataName).Updates(map[string]any{"end_time": now, "start_time": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("pending %s marker not found", installDataName)
	}
	return nil
}

// legacyTableExists deliberately does not use Migrator.HasTable with a raw
// string.  Some GORM/MySQL combinations dereference model metadata for that
// form and can panic; information_schema also preserves prefixes verbatim.
func legacyTableExists(db *gorm.DB, name string) (bool, error) {
	var count int64
	result := db.Raw(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
		name,
	).Scan(&count)
	return count > 0, result.Error
}

func legacyColumnExists(db *gorm.DB, table, column string) (bool, error) {
	var count int64
	result := db.Raw(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
		table, column,
	).Scan(&count)
	return count > 0, result.Error
}

func tableExists(db *gorm.DB, name string) bool { ok, _ := legacyTableExists(db, name); return ok }
func columnExists(db *gorm.DB, table, column string) bool {
	ok, _ := legacyColumnExists(db, table, column)
	return ok
}

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

func normalizeLegacyColumns(db *gorm.DB, config *conf.Configuration, onlyTable string) error {
	for _, item := range version200Columns {
		if onlyTable != "" && item.table != onlyTable {
			continue
		}
		t := tableName(config, item.table)
		if !tableExists(db, t) || !columnExists(db, t, item.old) {
			continue
		}
		if columnExists(db, t, item.new) {
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

// PrepareLegacySchema must run before AutoMigrate.  It is deliberately made
// independent of the generated models so that it can also open very old
// BuildAdmin databases.
func PrepareLegacySchema(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
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
	admin := tableName(config, "admin_rule")
	menu := tableName(config, "menu_rule")
	if tableExists(db, menu) {
		backup := menuRuleBackupName(config)
		if tableExists(db, backup) {
			return fmt.Errorf("backup table %s already exists while menu_rule is present; refusing to overwrite", backup)
		}
		if !tableExists(db, admin) {
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
				if columnExists(db, menu, c) && columnExists(db, admin, c) {
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
			if err := db.Exec("RENAME TABLE " + quoteIdentifier(menu) + " TO " + quoteIdentifier(backup)).Error; err != nil {
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
		t := tableName(config, item.table)
		if tableExists(db, t) && columnExists(db, t, item.column) {
			if err := db.Exec("ALTER TABLE `" + t + "` MODIFY COLUMN `" + item.column + "` " + item.typ).Error; err != nil {
				return fmt.Errorf("alter %s.%s: %w", t, item.column, err)
			}
		}
	}
	return nil
}

func IsFreshDatabase(db *gorm.DB, config *conf.Configuration) (bool, error) {
	if err := ValidatePrefix(config); err != nil {
		return false, err
	}
	for _, name := range []string{"admin_group_access", "admin_group", "admin_log", "admin_rule", "menu_rule", "admin", "area", "attachment", "captcha", "config", "crud_log", "migrations", "security_data_recycle_log", "security_data_recycle", "security_sensitive_data_log", "security_sensitive_data", "test_build", "token", "user_group", "user_money_log", "user_rule", "user_score_log", "user"} {
		ok, err := legacyTableExists(db, tableName(config, name))
		if err != nil {
			return false, err
		}
		if ok {
			return false, nil
		}
	}
	return true, nil
}

// SeedCurrentData reports whether the install seed is complete. A schema can
// exist after a failed transaction, so table existence alone is insufficient.
func SeedCurrentData(db *gorm.DB, config *conf.Configuration) (bool, error) {
	if err := ValidatePrefix(config); err != nil {
		return false, err
	}
	checks := []struct{ table, column, value string }{
		{"admin_rule", "name", "dashboard"}, {"admin_rule", "name", "auth/rule"}, {"admin_rule", "name", "dashboard/index"},
		{"admin_group", "id", "1"}, {"admin", "id", "1"}, {"config", "id", "1"},
		{"user_group", "id", "1"}, {"user", "id", "1"},
	}
	for _, check := range checks {
		t := tableName(config, check.table)
		if !tableExists(db, t) {
			return false, nil
		}
		var count int64
		if err := db.Table(t).Where(quoteIdentifier(check.column)+" = ?", check.value).Count(&count).Error; err != nil {
			return false, err
		}
		if count != 1 {
			return false, nil
		}
	}
	return true, nil
}

func ValidateCurrentSchema(db *gorm.DB, config *conf.Configuration) error {
	t := tableName(config, "user_rule")
	if !tableExists(db, t) || !columnExists(db, t, "no_login_valid") {
		return fmt.Errorf("%s.no_login_valid is missing after AutoMigrate", t)
	}
	var columnType string
	if err := db.Raw("SELECT column_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='type'", t).Scan(&columnType).Error; err != nil {
		return err
	}
	if !strings.Contains(columnType, "menu_dir") || !strings.Contains(columnType, "button") {
		return fmt.Errorf("%s.type does not contain the current rule enum", t)
	}
	return nil
}

type migrationRecord struct {
	Version       int64      `gorm:"column:version"`
	MigrationName string     `gorm:"column:migration_name"`
	StartTime     time.Time  `gorm:"column:start_time"`
	EndTime       *time.Time `gorm:"column:end_time"`
	Breakpoint    bool       `gorm:"column:breakpoint"`
}

type MigrationFn func(*gorm.DB, *conf.Configuration) error

type VersionMigration struct {
	Version int64
	Name    string
	Up      MigrationFn
}

// tableName 获取带前缀的表名
func tableName(config *conf.Configuration, logicalName string) string {
	return config.Database.Prefix + logicalName
}

// ─── Version205: 配置快捷入口从 /admin/ 路径迁移到路由 name ───
func version205(db *gorm.DB, config *conf.Configuration) error {
	var cfgs []model.Config
	result := db.Table(tableName(config, "config")).Where("name = ?", "config_quick_entrance").Find(&cfgs)
	if result.Error != nil {
		return result.Error
	}
	if len(cfgs) == 0 {
		return nil
	}
	cfg := cfgs[0]
	if strings.TrimSpace(cfg.Value) == "" {
		return nil
	}

	// 使用 map[string]any 保留所有字段，避免重编码时丢失未知属性
	var entries []map[string]any
	if err := json.Unmarshal([]byte(cfg.Value), &entries); err != nil {
		return fmt.Errorf("parse config_quick_entrance: %w", err)
	}

	changed := false
	for i := range entries {
		val, ok := entries[i]["value"].(string)
		if !ok || !strings.HasPrefix(val, "/admin/") {
			continue
		}
		path := strings.TrimPrefix(val, "/admin/")
		var rules []model.AdminRule
		result := db.Table(tableName(config, "admin_rule")).Where("path = ?", path).Find(&rules)
		if result.Error != nil {
			return result.Error
		}
		if len(rules) == 0 {
			continue
		}
		rule := rules[0]
		if val != rule.Name {
			entries[i]["value"] = rule.Name
			changed = true
		}
	}
	if !changed {
		return nil
	}
	value, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return db.Table(tableName(config, "config")).Where("id = ?", cfg.ID).Update("value", string(value)).Error
}

func version200(db *gorm.DB, config *conf.Configuration) error {
	t := tableName(config, "admin_rule")
	if !tableExists(db, t) {
		return nil
	}
	for _, item := range []struct{ old, new string }{
		{"auth/menu", "auth/rule"},
		{"auth/menu/index", "auth/rule/index"}, {"auth/menu/add", "auth/rule/add"},
		{"auth/menu/edit", "auth/rule/edit"}, {"auth/menu/del", "auth/rule/del"}, {"auth/menu/sortable", "auth/rule/sortable"},
	} {
		var oldCount, newCount int64
		if err := db.Table(t).Where("name = ?", item.old).Count(&oldCount).Error; err != nil {
			return err
		}
		if err := db.Table(t).Where("name = ?", item.new).Count(&newCount).Error; err != nil {
			return err
		}
		if oldCount > 1 || newCount > 1 {
			return fmt.Errorf("duplicate rules for %s or %s", item.old, item.new)
		}
		if oldCount > 0 && newCount > 0 {
			var oldID, newID int32
			if err := db.Table(t).Where("name = ?", item.old).Pluck("id", &oldID).Error; err != nil {
				return err
			}
			if err := db.Table(t).Where("name = ?", item.new).Pluck("id", &newID).Error; err != nil {
				return err
			}
			if oldID != newID {
				return fmt.Errorf("%s and %s both exist", item.old, item.new)
			}
			continue
		}
		if oldCount == 0 {
			continue
		}
		var err error
		if item.old == "auth/menu" {
			err = db.Exec("UPDATE `"+t+"` SET name=?, path=?, component=? WHERE name=?", item.new, "auth/rule", "/src/views/backend/auth/rule/index.vue", item.old).Error
		} else {
			err = db.Exec("UPDATE `"+t+"` SET name=? WHERE name=?", item.new, item.old).Error
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func version201(db *gorm.DB, config *conf.Configuration) error {
	t := tableName(config, "user")
	if !tableExists(db, t) {
		return nil
	}
	rows, err := db.Raw("SELECT index_name, GROUP_CONCAT(column_name ORDER BY seq_in_index) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND non_unique=0 AND index_name <> 'PRIMARY' GROUP BY index_name", t).Rows()
	if err != nil {
		return err
	}
	var indexes []string
	for rows.Next() {
		var idx, cols string
		if err := rows.Scan(&idx, &cols); err != nil {
			return err
		}
		if cols == "email" || cols == "mobile" {
			indexes = append(indexes, idx)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, idx := range indexes {
		if err := db.Exec("ALTER TABLE " + quoteIdentifier(t) + " DROP INDEX " + quoteIdentifier(idx)).Error; err != nil {
			return err
		}
	}
	return nil
}

func version202(db *gorm.DB, config *conf.Configuration) error {
	t := tableName(config, "admin_rule")
	if !tableExists(db, t) {
		return nil
	}
	for _, item := range []struct{ old, new string }{{"dashboard/dashboard", "dashboard"}, {"buildadmin/buildadmin", "buildadmin"}} {
		var oldCount, newCount int64
		if err := db.Table(t).Where("name = ?", item.old).Count(&oldCount).Error; err != nil {
			return err
		}
		if err := db.Table(t).Where("name = ?", item.new).Count(&newCount).Error; err != nil {
			return err
		}
		if oldCount > 1 || newCount > 1 {
			return fmt.Errorf("duplicate rules for %s or %s", item.old, item.new)
		}
		if oldCount > 0 && newCount > 0 {
			return fmt.Errorf("%s and %s both exist", item.old, item.new)
		}
		if err := db.Exec("UPDATE `"+t+"` SET name=? WHERE name=?", item.new, item.old).Error; err != nil {
			return err
		}
	}
	var dashboards []model.AdminRule
	if err := db.Table(t).Where("name = ?", "dashboard").Find(&dashboards).Error; err != nil {
		return err
	}
	if err := validateDashboardRuleCount(len(dashboards)); err != nil {
		return err
	}
	if len(dashboards) == 0 {
		// A newly installed schema has an empty admin_rule table.  The install
		// seed creates dashboard and its button after version migrations run.
		// Also tolerate an existing rule set which simply has no console rule;
		// there is no safe parent to attach dashboard/index to in that case.
		return nil
	}
	dashboardID := dashboards[0].ID
	if dashboardID != 0 {
		var buttons []model.AdminRule
		result := db.Table(t).Where("name = ?", "dashboard/index").Find(&buttons)
		if result.Error != nil {
			return result.Error
		}
		if len(buttons) > 1 {
			return fmt.Errorf("multiple dashboard/index rules found")
		}
		var button model.AdminRule
		if len(buttons) == 1 {
			button = buttons[0]
		}
		if len(buttons) == 0 {
			if err := db.Table(t).Create(&model.AdminRule{Pid: dashboardID, Type: "button", Title: "查看", Name: "dashboard/index", Status: "1", UpdateTime: time.Now().Unix(), CreateTime: time.Now().Unix()}).Error; err != nil {
				return err
			}
			if err := db.Table(t).Where("name = ?", "dashboard/index").First(&button).Error; err != nil {
				return err
			}
		} else if button.Pid != dashboardID {
			return fmt.Errorf("dashboard/index has pid %d, want %d", button.Pid, dashboardID)
		} else if button.Type != "button" || button.Title != "查看" {
			if err := db.Table(t).Where("id = ?", button.ID).Updates(map[string]any{"type": "button", "title": "查看"}).Error; err != nil {
				return err
			}
		}
		groups := tableName(config, "admin_group")
		if tableExists(db, groups) {
			if err := db.Exec("UPDATE `"+groups+"` SET rules=CONCAT_WS(',', NULLIF(rules,''), ?) WHERE FIND_IN_SET(?, rules)>0 AND FIND_IN_SET(?, rules)=0", button.ID, dashboardID, button.ID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// validateDashboardRuleCount keeps the empty/new-install behavior explicit
// and makes the ambiguity rule independently testable without MySQL.
func validateDashboardRuleCount(count int) error {
	if count > 1 {
		return fmt.Errorf("multiple dashboard rules found")
	}
	return nil
}

// ReconcileLegacyData repairs data migrations that may already be recorded as
// complete (for example after an older installer seeded stale rules).
func ReconcileLegacyData(db *gorm.DB, config *conf.Configuration) error {
	if err := version200(db, config); err != nil {
		return err
	}
	if err := version202(db, config); err != nil {
		return err
	}
	return db.Table(tableName(config, "security_data_recycle")).Where("data_table = ? AND controller_as = ?", "menu_rule", "auth/menu").Updates(map[string]any{"data_table": "admin_rule", "controller_as": "auth/rule"}).Error
}

// ─── Version206: 5 张表新增 connection 字段（跳过 PHP 专属 backend_entrance） ───
func version206(db *gorm.DB, config *conf.Configuration) error {
	tables := []struct {
		name  string
		model any
	}{
		{"crud_log", &model.CrudLog{}},
		{"security_data_recycle", &model.SecurityDataRecycle{}},
		{"security_data_recycle_log", &model.SecurityDataRecycleLog{}},
		{"security_sensitive_data", &model.SecuritySensitiveData{}},
		{"security_sensitive_data_log", &model.SecuritySensitiveDataLog{}},
	}
	for _, item := range tables {
		fullTable := tableName(config, item.name)
		migrator := db.Table(fullTable).Migrator()
		if !migrator.HasTable(item.model) {
			continue
		}
		if migrator.HasColumn(item.model, "Connection") {
			continue
		}
		if err := migrator.AddColumn(item.model, "Connection"); err != nil {
			return fmt.Errorf("add connection column to %s: %w", fullTable, err)
		}
	}
	return nil
}

// ─── Version222: 列类型扩容 + crud_log 新增字段 + 历史回填 ───
// 跳过：status 类型迁移（0/1 → enable/disable）、7 张表 status 值映射
func version222(db *gorm.DB, config *conf.Configuration) error {
	// 列类型扩容
	type alterSpec struct {
		table string
		model any
		field string
	}
	alterSpecs := []alterSpec{
		{"attachment", &model.Attachment{}, "Name"},
		{"admin", &model.Admin{}, "Password"},
		{"user", &model.User{}, "Password"},
	}
	for _, spec := range alterSpecs {
		fullTable := tableName(config, spec.table)
		if err := db.Table(fullTable).Migrator().AlterColumn(spec.model, spec.field); err != nil {
			return fmt.Errorf("alter %s.%s: %w", fullTable, spec.field, err)
		}
	}

	// crud_log 新增 comment 和 sync（独立检查，幂等）
	crudLogTable := tableName(config, "crud_log")
	crudLogModel := &model.CrudLog{}
	crudMigrator := db.Table(crudLogTable).Migrator()
	if !crudMigrator.HasColumn(crudLogModel, "Comment") {
		if err := crudMigrator.AddColumn(crudLogModel, "Comment"); err != nil {
			return fmt.Errorf("add comment column to %s: %w", crudLogTable, err)
		}
	}
	if !crudMigrator.HasColumn(crudLogModel, "Sync") {
		if err := crudMigrator.AddColumn(crudLogModel, "Sync"); err != nil {
			return fmt.Errorf("add sync column to %s: %w", crudLogTable, err)
		}
	}

	// 历史回填：从 crud_log.table JSON 中提取 comment
	var logs []struct {
		ID    int32  `gorm:"column:id"`
		Table string `gorm:"column:table"`
	}
	if err := db.Table(crudLogTable).Where("comment = ?", "").Find(&logs).Error; err != nil {
		return fmt.Errorf("query crud_log for backfill: %w", err)
	}
	for _, log := range logs {
		var data struct {
			Comment string `json:"comment"`
		}
		if err := json.Unmarshal([]byte(log.Table), &data); err != nil {
			continue // 解析失败跳过单条
		}
		if data.Comment == "" {
			continue
		}
		if err := db.Table(crudLogTable).Where("id = ?", log.ID).Update("comment", data.Comment).Error; err != nil {
			return fmt.Errorf("backfill crud_log id=%d: %w", log.ID, err)
		}
	}
	return nil
}

// ─── 迁移注册 ───

var allMigrations = []VersionMigration{
	{Version: 20230622221507, Name: "Version200", Up: version200},
	{Version: 20230719211338, Name: "Version201", Up: version201},
	{Version: 20230905060702, Name: "Version202", Up: version202},
	{Version: 20231112093414, Name: "Version205", Up: version205},
	{Version: 20231229043002, Name: "Version206", Up: version206},
	{Version: 20250412134127, Name: "Version222", Up: version222},
}

// validateMigrations 验证迁移列表的版本号严格递增、名称非空
func validateMigrations() error {
	var previous int64
	for i, m := range allMigrations {
		if m.Version <= previous {
			return fmt.Errorf("migration versions must be strictly increasing: %d", m.Version)
		}
		if i > 0 && m.Version == allMigrations[i-1].Version {
			return fmt.Errorf("duplicate migration version: %d", m.Version)
		}
		if strings.TrimSpace(m.Name) == "" || m.Up == nil {
			return fmt.Errorf("invalid migration at version %d", m.Version)
		}
		previous = m.Version
	}
	return nil
}

// RunVersionMigrations 执行尚未完成的版本迁移
// 每个迁移必须幂等；成功后才写入 migration record
func RunVersionMigrations(db *gorm.DB, config *conf.Configuration) (int, error) {
	if err := validateMigrations(); err != nil {
		return 0, err
	}
	migrationsTable := tableName(config, "migrations")
	count := 0

	for _, migration := range allMigrations {
		// 查询该版本是否存在已完成记录
		var record migrationRecord
		recordDB := db.Session(&gorm.Session{})
		result := recordDB.Table(migrationsTable).Where("version = ?", migration.Version).Limit(1).Find(&record)
		if result.Error != nil {
			return count, fmt.Errorf("query migration version %d: %w", migration.Version, result.Error)
		}

		recordExists := result.RowsAffected > 0
		if recordExists && record.EndTime != nil {
			// 已完成：验证名称一致（collision 检测）
			if record.MigrationName != migration.Name {
				return count, fmt.Errorf("migration version %d: name collision (db=%s, code=%s)",
					migration.Version, record.MigrationName, migration.Name)
			}
			continue // 跳过已完成的迁移
		}

		// 记录开始时间
		start := time.Now()

		// 执行迁移
		migrationDB := db.Session(&gorm.Session{})
		if err := migration.Up(migrationDB, config); err != nil {
			return count, fmt.Errorf("migration %s failed: %w", migration.Name, err)
		}

		end := time.Now()

		// 成功后才写入/更新 migration record
		var writeErr error
		if recordExists {
			writeErr = db.Session(&gorm.Session{}).Table(migrationsTable).Where("version = ?", migration.Version).Updates(map[string]any{
				"migration_name": migration.Name,
				"start_time":     start,
				"end_time":       end,
			}).Error
		} else {
			writeErr = db.Session(&gorm.Session{}).Exec(
				"INSERT INTO `"+migrationsTable+"` (`version`, `migration_name`, `start_time`, `end_time`, `breakpoint`) VALUES (?, ?, ?, ?, ?)",
				migration.Version, migration.Name, start, end, false,
			).Error
		}
		if writeErr != nil {
			return count, fmt.Errorf("write migration record for %s: %w", migration.Name, writeErr)
		}
		count++
	}
	return count, nil
}
