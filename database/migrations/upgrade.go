package migrations

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"go-build-admin/app/pkg/systemroot"
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
	db = db.Session(&gorm.Session{NewDB: true})
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
	db = db.Session(&gorm.Session{NewDB: true})
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
	db = db.Session(&gorm.Session{NewDB: true})
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

func adminStatusNeedsBridge(db *gorm.DB, table string) (bool, error) {
	var columnType string
	result := db.Raw("SELECT column_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='status'", table).Scan(&columnType)
	if result.Error != nil {
		return false, result.Error
	}
	return strings.EqualFold(columnType, "enum('0','1')"), nil
}

// bridgeAdminStatusSchema removes the legacy enum before AutoMigrate can
// coerce the new string defaults. Existing values are deliberately untouched.
func bridgeAdminStatusSchema(db *gorm.DB, config *conf.Configuration) error {
	t := tableName(config, "admin")
	if !tableExists(db, t) || !columnExists(db, t, "status") {
		return nil
	}
	needs, err := adminStatusNeedsBridge(db, t)
	if err != nil {
		return err
	}
	if needs {
		// Keep the old default until Version223 maps the data. If a later stage
		// fails, the legacy runtime can still create administrators safely.
		if err := db.Exec("ALTER TABLE " + quoteIdentifier(t) + " MODIFY COLUMN `status` VARCHAR(30) NOT NULL DEFAULT '1'").Error; err != nil {
			return fmt.Errorf("bridge %s.status: %w", t, err)
		}
	}
	return nil
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
// PrepareUpstreamNeutralSchema performs only compatibility normalization that
// does not change the local account-status protocol.
func PrepareUpstreamNeutralSchema(db *gorm.DB, config *conf.Configuration) error {
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

var allowedAccountStatuses = map[string]string{"0": "disable", "1": "enable", "enable": "enable", "disable": "disable"}

// mapAccountStatuses validates the complete input before returning any
// replacements, allowing migrations to preflight both tables atomically.
func mapAccountStatuses(values []string) ([]string, error) {
	result := make([]string, len(values))
	for i, value := range values {
		mapped, ok := allowedAccountStatuses[value]
		if !ok {
			return nil, fmt.Errorf("invalid account status %q", value)
		}
		result[i] = mapped
	}
	return result, nil
}

func accountStatusValues(db *gorm.DB, table string) ([]string, error) {
	var nullCount int64
	if err := db.Raw("SELECT COUNT(*) FROM " + quoteIdentifier(table) + " WHERE status IS NULL").Scan(&nullCount).Error; err != nil {
		return nil, err
	}
	if nullCount > 0 {
		return nil, fmt.Errorf("null status")
	}
	var values []string
	if err := db.Raw("SELECT DISTINCT CAST(status AS BINARY) AS status FROM " + quoteIdentifier(table)).Scan(&values).Error; err != nil {
		return nil, err
	}
	return values, nil
}

func alterAccountStatusColumn(db *gorm.DB, table string) error {
	return db.Exec("ALTER TABLE " + quoteIdentifier(table) + " MODIFY COLUMN `status` VARCHAR(30) NOT NULL DEFAULT 'enable' COMMENT '状态:enable=启用,disable=禁用'").Error
}

// ─── Version223: admin/user status protocol 0/1 → disable/enable ───
func version223(db *gorm.DB, config *conf.Configuration) error {
	tables := []string{tableName(config, "admin"), tableName(config, "user")}
	values := make([][]string, len(tables))
	present := make([]bool, len(tables))
	for i, table := range tables {
		if !tableExists(db, table) || !columnExists(db, table, "status") {
			continue
		}
		present[i] = true
		var err error
		values[i], err = accountStatusValues(db, table)
		if err != nil {
			return fmt.Errorf("preflight %s.status: %w", table, err)
		}
		if _, err := mapAccountStatuses(values[i]); err != nil {
			return fmt.Errorf("preflight %s.status: %w", table, err)
		}
	}
	for i, table := range tables {
		if !present[i] {
			continue
		}
		if err := alterAccountStatusColumn(db, table); err != nil {
			return fmt.Errorf("alter %s.status: %w", table, err)
		}
		if err := db.Table(table).Where("status = ?", "0").Update("status", "disable").Error; err != nil {
			return err
		}
		if err := db.Table(table).Where("status = ?", "1").Update("status", "enable").Error; err != nil {
			return err
		}
		var invalid int64
		if err := db.Raw("SELECT COUNT(*) FROM " + quoteIdentifier(table) + " WHERE status IS NULL OR BINARY status NOT IN ('enable', 'disable')").Scan(&invalid).Error; err != nil {
			return err
		}
		if invalid != 0 {
			return fmt.Errorf("%s.status contains invalid values after migration", table)
		}
	}
	return nil
}

type InstallRecoveryState string

const (
	InstallFresh         InstallRecoveryState = "fresh"
	InstallInterrupted   InstallRecoveryState = "interrupted_install"
	InstallStrictUpgrade InstallRecoveryState = "strict_upgrade"
)

// DecideInstallRecovery reads only ledger/table state. It is intentionally
// separate from AutoMigrate so the handler cannot choose a destructive path
// before it knows whether an install snapshot was interrupted.
func DecideInstallRecovery(db *gorm.DB, config *conf.Configuration) (InstallRecoveryState, error) {
	if err := ValidatePrefix(config); err != nil {
		return "", err
	}
	ledgerExists, err := legacyTableExists(db, tableName(config, "migrations"))
	if err != nil {
		return "", err
	}
	markerPending := false
	markerFound := false
	if ledgerExists {
		var marker migrationRecord
		result := db.Table(tableName(config, "migrations")).Where("version = ?", installDataVersion).First(&marker)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return "", result.Error
		}
		if result.Error == nil {
			markerFound = true
			if marker.MigrationName != installDataName {
				return "", fmt.Errorf("migration version %d name collision", installDataVersion)
			}
			markerPending = marker.EndTime == nil
		}
	}
	businessTables := []string{"admin_group_access", "admin_group", "admin_log", "admin_rule", "menu_rule", "admin", "area", "attachment", "captcha", "config", "crud_log", "security_data_recycle_log", "security_data_recycle", "security_sensitive_data_log", "security_sensitive_data", "test_build", "token", "user_group", "user_money_log", "user_rule", "user_score_log", "user"}
	businessExists := false
	for _, name := range businessTables {
		ok, err := legacyTableExists(db, tableName(config, name))
		if err != nil {
			return "", err
		}
		if ok {
			businessExists = true
			break
		}
	}
	if markerPending || (ledgerExists && !businessExists && !markerFound) {
		return InstallInterrupted, nil
	}
	if ledgerExists && !businessExists && markerFound {
		return "", fmt.Errorf("completed InstallData marker has no business schema")
	}
	if !ledgerExists && !businessExists {
		return InstallFresh, nil
	}
	return InstallStrictUpgrade, nil
}

func IsFreshDatabase(db *gorm.DB, config *conf.Configuration) (bool, error) {
	state, err := DecideInstallRecovery(db, config)
	return state != InstallStrictUpgrade, err
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
// 账号 status 转换延后至 Version223；7 张布尔状态表保持 Go 的 0/1 语义。
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

func indexExists(db *gorm.DB, table, index string) bool {
	var count int64
	result := db.Raw(
		"SELECT COUNT(*) FROM information_schema.statistics "+
			"WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
		table, index,
	).Scan(&count)
	return result.Error == nil && count > 0
}

func indexFirstColumn(db *gorm.DB, table, index string) (string, error) {
	var column string
	err := db.Raw("SELECT column_name FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ? AND seq_in_index = 1", table, index).Scan(&column).Error
	return column, err
}

// ─── Version225: attachment owner-leading index ───
func version225(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	table := tableName(config, "attachment")
	if !tableExists(db, table) {
		return nil
	}
	if indexExists(db, table, "idx_admin_id") {
		column, err := indexFirstColumn(db, table, "idx_admin_id")
		if err != nil {
			return fmt.Errorf("inspect idx_admin_id on %s: %w", table, err)
		}
		if column != "admin_id" {
			return fmt.Errorf("idx_admin_id on %s starts with %q, want admin_id", table, column)
		}
		return nil
	}
	if err := db.Exec("CREATE INDEX `idx_admin_id` ON " + quoteIdentifier(table) + " (`admin_id`)").Error; err != nil {
		return fmt.Errorf("add idx_admin_id to %s: %w", table, err)
	}
	return nil
}

// EnsureAdminClosureSelfRows inserts the mandatory (id,id,0) self-row for every
// administrator that does not already have one. It is idempotent and safe to
// call after both fresh seed and upgrade paths.
func EnsureAdminClosureSelfRows(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	closureTable := tableName(config, "admin_closure")
	if !tableExists(db, adminTable) {
		return nil
	}
	if !tableExists(db, closureTable) {
		return fmt.Errorf("%s does not exist", closureTable)
	}
	if err := db.Exec(
		"INSERT IGNORE INTO " + quoteIdentifier(closureTable) + " (ancestor_id, descendant_id, depth) " +
			"SELECT id, id, 0 FROM " + quoteIdentifier(adminTable),
	).Error; err != nil {
		return fmt.Errorf("backfill %s self rows: %w", closureTable, err)
	}
	return nil
}

// ─── Version224: 管理员层级 parent_id 与闭包表 ───
func version224(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	closureTable := tableName(config, "admin_closure")

	if tableExists(db, adminTable) {
		if !columnExists(db, adminTable, "parent_id") {
			if err := db.Exec(
				"ALTER TABLE " + quoteIdentifier(adminTable) +
					" ADD COLUMN `parent_id` int(11) unsigned DEFAULT NULL COMMENT '父级管理员ID'",
			).Error; err != nil {
				return fmt.Errorf("add parent_id to %s: %w", adminTable, err)
			}
		}
		if !indexExists(db, adminTable, "idx_parent_id") {
			if err := db.Exec(
				"CREATE INDEX `idx_parent_id` ON " + quoteIdentifier(adminTable) + " (`parent_id`)",
			).Error; err != nil {
				return fmt.Errorf("add idx_parent_id to %s: %w", adminTable, err)
			}
		}
	}

	if !tableExists(db, closureTable) {
		if err := db.Exec(
			"CREATE TABLE IF NOT EXISTS " + quoteIdentifier(closureTable) + " (" +
				"`ancestor_id` int(11) unsigned NOT NULL," +
				"`descendant_id` int(11) unsigned NOT NULL," +
				"`depth` int(11) unsigned NOT NULL DEFAULT 0," +
				"PRIMARY KEY (`ancestor_id`,`descendant_id`)," +
				"KEY `idx_descendant_ancestor` (`descendant_id`,`ancestor_id`)," +
				"KEY `idx_ancestor_depth` (`ancestor_id`,`depth`)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		).Error; err != nil {
			return fmt.Errorf("create %s: %w", closureTable, err)
		}
	}

	for _, idx := range []struct{ name, cols string }{
		{"idx_descendant_ancestor", "`descendant_id`,`ancestor_id`"},
		{"idx_ancestor_depth", "`ancestor_id`,`depth`"},
	} {
		if !indexExists(db, closureTable, idx.name) {
			if err := db.Exec(
				"CREATE INDEX " + quoteIdentifier(idx.name) + " ON " + quoteIdentifier(closureTable) + " (" + idx.cols + ")",
			).Error; err != nil {
				return fmt.Errorf("add %s to %s: %w", idx.name, closureTable, err)
			}
		}
	}

	lockTable := tableName(config, "admin_hierarchy_lock")
	if !tableExists(db, lockTable) {
		if err := db.Exec(
			"CREATE TABLE IF NOT EXISTS " + quoteIdentifier(lockTable) + " (" +
				"`id` tinyint(3) unsigned NOT NULL," +
				"PRIMARY KEY (`id`)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		).Error; err != nil {
			return fmt.Errorf("create %s: %w", lockTable, err)
		}
	}
	if err := db.Exec("INSERT IGNORE INTO " + quoteIdentifier(lockTable) + " (id) VALUES (1)").Error; err != nil {
		return fmt.Errorf("seed %s: %w", lockTable, err)
	}

	return EnsureAdminClosureSelfRows(db, config)
}

type migrationColumn struct {
	ColumnType string  `gorm:"column:column_type"`
	Nullable   string  `gorm:"column:is_nullable"`
	Default    *string `gorm:"column:column_default"`
}

func migrationColumnInfo(db *gorm.DB, table, column string) (migrationColumn, bool, error) {
	var columnType, nullable string
	var defaultValue sql.NullString
	err := db.Raw("SELECT column_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, column).Row().Scan(&columnType, &nullable, &defaultValue)
	if errors.Is(err, sql.ErrNoRows) {
		return migrationColumn{}, false, nil
	}
	if err != nil {
		return migrationColumn{}, false, err
	}
	var defaultPtr *string
	if defaultValue.Valid {
		value := defaultValue.String
		defaultPtr = &value
	}
	return migrationColumn{ColumnType: columnType, Nullable: nullable, Default: defaultPtr}, true, nil
}

func migrationIndexInfo(db *gorm.DB, table, index string) (bool, string, error) {
	var column string
	err := db.Raw("SELECT column_name FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND seq_in_index=1", table, index).Row().Scan(&column)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, column, nil
}

func validOwnerColumn(def migrationColumn, label string) error {
	typ := strings.ToLower(def.ColumnType)
	if !strings.Contains(typ, "int") || !strings.Contains(typ, "unsigned") || !strings.EqualFold(def.Nullable, "NO") || def.Default == nil || *def.Default != "0" {
		return fmt.Errorf("%s has invalid owner column schema", label)
	}
	return nil
}

func addAdminOwnerColumnAndIndex(db *gorm.DB, table string) error {
	exists, err := legacyTableExists(db, table)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	def, ok, err := migrationColumnInfo(db, table, "admin_id")
	if err != nil {
		return err
	}
	if !ok {
		if err := db.Exec("ALTER TABLE " + quoteIdentifier(table) + " ADD COLUMN `admin_id` int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'").Error; err != nil {
			return err
		}
		def, ok, err = migrationColumnInfo(db, table, "admin_id")
		if err != nil {
			return err
		}
	}
	if !ok {
		return fmt.Errorf("%s.admin_id was not created", table)
	}
	if err := validOwnerColumn(def, table+".admin_id"); err != nil {
		return err
	}
	has, first, err := migrationIndexInfo(db, table, "idx_admin_id")
	if err != nil {
		return fmt.Errorf("inspect idx_admin_id on %s: %w", table, err)
	}
	if has {
		if first != "admin_id" {
			return fmt.Errorf("idx_admin_id on %s starts with %q, want admin_id", table, first)
		}
		return nil
	}
	return db.Exec("CREATE INDEX `idx_admin_id` ON " + quoteIdentifier(table) + " (`admin_id`)").Error
}

func migrationRootID(db *gorm.DB, config *conf.Configuration) (int32, error) {
	adminTable := tableName(config, "admin")
	if ok, err := legacyTableExists(db, adminTable); err != nil {
		return 0, err
	} else if !ok {
		return 0, fmt.Errorf("admin table %s does not exist", adminTable)
	}
	return (systemroot.Resolver{DB: db, AdminTable: adminTable}).Resolve()
}

func repairMigrationOwners(db *gorm.DB, table, adminTable string, rootID int32) error {
	return db.Exec("UPDATE "+quoteIdentifier(table)+" t LEFT JOIN "+quoteIdentifier(adminTable)+" a ON a.id=t.admin_id SET t.admin_id=? WHERE t.admin_id=0 OR t.admin_id IS NULL OR a.id IS NULL", rootID).Error
}

func validateMigrationOwners(db *gorm.DB, table, adminTable string) error {
	var invalid int64
	if err := db.Raw("SELECT COUNT(*) FROM " + quoteIdentifier(table) + " t LEFT JOIN " + quoteIdentifier(adminTable) + " a ON a.id=t.admin_id WHERE t.admin_id=0 OR a.id IS NULL").Scan(&invalid).Error; err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("%s contains %d invalid admin owner(s)", table, invalid)
	}
	return nil
}

func migrationTablesHaveRows(db *gorm.DB, tables []string) (bool, error) {
	for _, table := range tables {
		if !tableExists(db, table) {
			continue
		}
		var count int64
		if err := db.Table(table).Limit(1).Count(&count).Error; err != nil {
			return false, err
		}
		if count != 0 {
			return true, nil
		}
	}
	return false, nil
}

func validateLogOwnerMatchesUser(db *gorm.DB, logTable, userTable string) error {
	if !tableExists(db, logTable) || !tableExists(db, userTable) {
		return nil
	}
	var invalid int64
	if err := db.Raw("SELECT COUNT(*) FROM " + quoteIdentifier(logTable) + " l JOIN " + quoteIdentifier(userTable) + " u ON u.id=l.user_id WHERE l.admin_id<>u.admin_id").Scan(&invalid).Error; err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("%s contains %d owner mismatch(es)", logTable, invalid)
	}
	return nil
}

func version226(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	userTable := tableName(config, "user")
	logTables := []string{tableName(config, "user_money_log"), tableName(config, "user_score_log")}
	allTables := append([]string{userTable}, logTables...)
	for _, table := range allTables {
		if err := addAdminOwnerColumnAndIndex(db, table); err != nil {
			return err
		}
	}
	hasRows, err := migrationTablesHaveRows(db, allTables)
	if err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	hasAdmin, err := legacyTableExists(db, adminTable)
	if err != nil {
		return err
	}
	if !hasRows {
		return nil
	}
	if !hasAdmin {
		return fmt.Errorf("admin table %s does not exist while ownership backfill is required", adminTable)
	}
	rootID, err := migrationRootID(db, config)
	if err != nil {
		return err
	}
	if tableExists(db, userTable) {
		if err := repairMigrationOwners(db, userTable, adminTable, rootID); err != nil {
			return err
		}
		if err := validateMigrationOwners(db, userTable, adminTable); err != nil {
			return err
		}
	}
	for _, table := range logTables {
		if !tableExists(db, table) {
			continue
		}
		if tableExists(db, userTable) {
			if err := db.Exec("UPDATE " + quoteIdentifier(table) + " l JOIN " + quoteIdentifier(userTable) + " u ON u.id=l.user_id SET l.admin_id=u.admin_id").Error; err != nil {
				return err
			}
		}
		if err := db.Exec("UPDATE "+quoteIdentifier(table)+" l LEFT JOIN "+quoteIdentifier(adminTable)+" a ON a.id=l.admin_id SET l.admin_id=? WHERE l.admin_id=0 OR a.id IS NULL", rootID).Error; err != nil {
			return err
		}
		if err := validateMigrationOwners(db, table, adminTable); err != nil {
			return err
		}
		if err := validateLogOwnerMatchesUser(db, table, userTable); err != nil {
			return err
		}
	}
	return nil
}

func version228(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	for _, item := range []struct{ table, column string }{
		{tableName(config, "user_money_log"), "money"},
		{tableName(config, "user_score_log"), "score"},
	} {
		if !tableExists(db, item.table) {
			continue
		}
		def, ok, err := migrationColumnInfo(db, item.table, item.column)
		if err != nil {
			return err
		}
		if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "unsigned") {
			continue
		}
		if err := db.Exec("ALTER TABLE " + quoteIdentifier(item.table) + " MODIFY COLUMN " + quoteIdentifier(item.column) + " int(11) NOT NULL DEFAULT 0").Error; err != nil {
			return fmt.Errorf("sign %s.%s: %w", item.table, item.column, err)
		}
	}
	return nil
}

func version227(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	tables := []string{
		tableName(config, "admin_log"),
		tableName(config, "security_data_recycle_log"),
		tableName(config, "security_sensitive_data_log"),
		tableName(config, "security_data_recycle"),
		tableName(config, "security_sensitive_data"),
		tableName(config, "crud_log"),
	}
	for _, table := range tables {
		if err := addAdminOwnerColumnAndIndex(db, table); err != nil {
			return err
		}
	}
	hasRows, err := migrationTablesHaveRows(db, tables)
	if err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	hasAdmin, err := legacyTableExists(db, adminTable)
	if err != nil {
		return err
	}
	if !hasRows {
		return nil
	}
	if !hasAdmin {
		return fmt.Errorf("admin table %s does not exist while ownership backfill is required", adminTable)
	}
	rootID, err := migrationRootID(db, config)
	if err != nil {
		return err
	}
	for _, table := range tables {
		if tableExists(db, table) {
			if err := repairMigrationOwners(db, table, adminTable, rootID); err != nil {
				return err
			}
			if err := validateMigrationOwners(db, table, adminTable); err != nil {
				return err
			}
		}
	}
	return nil
}

func addTargetOwnerColumnAndIndex(db *gorm.DB, table string) error {
	exists, err := legacyTableExists(db, table)
	if err != nil || !exists {
		return err
	}
	def, ok, err := migrationColumnInfo(db, table, "target_admin_id")
	if err != nil {
		return err
	}
	if !ok {
		if err := db.Exec("ALTER TABLE " + quoteIdentifier(table) + " ADD COLUMN `target_admin_id` int(11) unsigned NOT NULL DEFAULT 0 COMMENT '目标数据管理员ID'").Error; err != nil {
			return err
		}
		def, ok, err = migrationColumnInfo(db, table, "target_admin_id")
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
	has, first, err := migrationIndexInfo(db, table, "idx_target_admin_id")
	if err != nil {
		return fmt.Errorf("inspect idx_target_admin_id on %s: %w", table, err)
	}
	if has {
		if first != "target_admin_id" {
			return fmt.Errorf("idx_target_admin_id on %s starts with %q, want target_admin_id", table, first)
		}
		return nil
	}
	return db.Exec("CREATE INDEX `idx_target_admin_id` ON " + quoteIdentifier(table) + " (`target_admin_id`)").Error
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
	if ok, err := legacyTableExists(db, adminTable); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("admin table %s does not exist for nonzero target_admin_id values", adminTable)
	}
	var invalid int64
	if err := db.Raw("SELECT COUNT(*) FROM " + quoteIdentifier(table) + " t LEFT JOIN " + quoteIdentifier(adminTable) + " a ON a.id=t.target_admin_id WHERE t.target_admin_id<>0 AND a.id IS NULL").Scan(&invalid).Error; err != nil {
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
	if err := db.Table(table).Select("id, " + quoteIdentifier(jsonColumn) + " AS payload").Where("target_admin_id=0").Find(&rows).Error; err != nil {
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

func version229(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	tables := []struct{ table, jsonColumn string }{
		{tableName(config, "security_data_recycle_log"), "data"},
		{tableName(config, "security_sensitive_data_log"), "before"},
	}
	for _, item := range tables {
		if err := addTargetOwnerColumnAndIndex(db, item.table); err != nil {
			return err
		}
		if tableExists(db, item.table) && columnExists(db, item.table, "target_admin_id") {
			if err := backfillTargetOwnerFromJSON(db, item.table, item.jsonColumn, adminTable); err != nil {
				return err
			}
			if err := validateTargetOwners(db, item.table, adminTable); err != nil {
				return err
			}
		}
	}
	return nil
}

func addLegacyTargetFlagColumn(db *gorm.DB, table string) error {
	if !tableExists(db, table) {
		return nil
	}
	def, ok, err := migrationColumnInfo(db, table, "legacy_unrecoverable")
	if err != nil {
		return err
	}
	if !ok {
		if err := db.Exec("ALTER TABLE " + quoteIdentifier(table) + " ADD COLUMN `legacy_unrecoverable` tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '历史目标管理员不可恢复'").Error; err != nil {
			return err
		}
		def, ok, err = migrationColumnInfo(db, table, "legacy_unrecoverable")
		if err != nil {
			return err
		}
	}
	if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "tinyint") || !strings.Contains(strings.ToLower(def.ColumnType), "unsigned") || !strings.EqualFold(def.Nullable, "NO") || def.Default == nil || *def.Default != "0" {
		return fmt.Errorf("%s.legacy_unrecoverable has invalid schema", table)
	}
	return nil
}

func version230(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	for _, table := range []string{tableName(config, "security_data_recycle_log"), tableName(config, "security_sensitive_data_log")} {
		if err := addLegacyTargetFlagColumn(db, table); err != nil {
			return err
		}
		if !tableExists(db, table) {
			continue
		}
		if err := db.Table(table).Where("target_admin_id=0").Update("legacy_unrecoverable", 1).Error; err != nil {
			return err
		}
		if err := validateTargetOwners(db, table, adminTable); err != nil {
			return err
		}
	}
	return nil
}

func addCommittedColumn(db *gorm.DB, table string) error {
	if !tableExists(db, table) {
		return nil
	}
	def, ok, err := migrationColumnInfo(db, table, "is_committed")
	if err != nil {
		return fmt.Errorf("inspect %s.is_committed: %w", table, err)
	}
	if !ok {
		if err := db.Exec("ALTER TABLE " + quoteIdentifier(table) + " ADD COLUMN `is_committed` tinyint(1) unsigned NOT NULL DEFAULT 0 COMMENT '提交状态'").Error; err != nil {
			return fmt.Errorf("add is_committed to %s: %w", table, err)
		}
		def, ok, err = migrationColumnInfo(db, table, "is_committed")
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

func version231(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	for _, table := range []string{tableName(config, "security_data_recycle_log"), tableName(config, "security_sensitive_data_log")} {
		if err := addCommittedColumn(db, table); err != nil {
			return err
		}
	}
	// Existing records intentionally remain is_committed=0. The migration must
	// not infer whether an old security operation completed successfully.
	return nil
}

const version232SensitiveUserOldFields = `{"username":"用户名","mobile":"手机号","password":"密码","status":"状态","email":"邮箱地址"}`

// version232 removes only the security rules shipped by the old installer.
// User-created rules are left untouched by matching the complete seed shape.
func version232(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	recycle := tableName(config, "security_data_recycle")
	if tableExists(db, recycle) {
		for _, seed := range []struct {
			id                             int
			name, controller, route, table string
		}{
			{1, "管理员", "auth/Admin.php", "auth/admin", "admin"},
			{2, "管理员日志", "auth/AdminLog.php", "auth/adminlog", "admin_log"},
			{3, "菜单规则", "auth/Menu.php", "auth/rule", "admin_rule"},
			{4, "系统配置项", "routine/Config.php", "routine/config", "config"},
			{6, "数据回收规则", "security/DataRecycle.php", "security/datarecycle", "security_data_recycle"},
		} {
			if err := db.Table(recycle).Where("id = ? AND name = ? AND controller = ? AND controller_as = ? AND data_table = ? AND primary_key = ?", seed.id, seed.name, seed.controller, seed.route, seed.table, "id").Delete(&model.SecurityDataRecycle{}).Error; err != nil {
				return err
			}
		}
		// The historical user row used id=5; normalize it only when it is
		// still the installer row, so a reused id cannot alter custom data.
		if err := db.Table(recycle).Where("id = ? AND name = ? AND controller = ? AND controller_as = ? AND data_table = ? AND primary_key = ?", 5, "会员", "user/User.php", "auth/user", "user", "id").Update("controller_as", "user/user").Error; err != nil {
			return err
		}
	}

	sensitive := tableName(config, "security_sensitive_data")
	if tableExists(db, sensitive) {
		for _, seed := range []struct {
			id                                     int
			name, controller, route, table, fields string
		}{
			{1, "管理员数据", "auth/Admin.php", "auth/admin", "admin", `{"username":"用户名","mobile":"手机","password":"密码","status":"状态"}`},
			{3, "管理员权限", "auth/Group.php", "auth/group", "admin_group", `{"rules":"权限规则ID"}`},
		} {
			if err := db.Table(sensitive).Where("id = ? AND name = ? AND controller = ? AND controller_as = ? AND data_table = ? AND primary_key = ? AND data_fields = ?", seed.id, seed.name, seed.controller, seed.route, seed.table, "id", seed.fields).Delete(&model.SecuritySensitiveData{}).Error; err != nil {
				return err
			}
		}
		newFields := `{"username":"用户名","mobile":"手机号","status":"状态","email":"邮箱地址"}`
		if err := db.Table(sensitive).Where("id = ? AND name = ? AND controller = ? AND controller_as = ? AND data_table = ? AND primary_key = ? AND data_fields = ?", 2, "会员数据", "user/User.php", "user/user", "user", "id", version232SensitiveUserOldFields).Updates(map[string]any{"data_fields": newFields}).Error; err != nil {
			return err
		}
	}
	return nil
}

// normalizeFreshSecuritySeed is the local overlay for the upstream installer
// seed. It preserves user-created rows while normalizing only the known
// installer identities whose IDs are part of the local compatibility contract.
func normalizeFreshSecuritySeed(db *gorm.DB, config *conf.Configuration) error {
	if err := version232(db, config); err != nil {
		return err
	}
	recycle := tableName(config, "security_data_recycle")
	if tableExists(db, recycle) {
		if err := convergeSecurityRule(db, recycle, 1, 5, "会员", "user/User.php", "user", "id", "user/user", "auth/user", ""); err != nil {
			return err
		}
	}
	sensitive := tableName(config, "security_sensitive_data")
	if tableExists(db, sensitive) {
		fields := `{"username":"用户名","mobile":"手机号","status":"状态","email":"邮箱地址"}`
		if err := convergeSecurityRule(db, sensitive, 1, 2, "会员数据", "user/User.php", "user", "id", "user/user", "auth/user", fields); err != nil {
			return err
		}
	}
	return nil
}

func convergeSecurityRule(db *gorm.DB, table string, sourceID, targetID int, name, controller, dataTable, primaryKey, finalRoute, sourceRoute, finalFields string) error {
	identity := "name = ? AND controller = ? AND data_table = ? AND primary_key = ?"
	var sourceCount, targetCount, duplicateCount int64
	if err := db.Table(table).Where("id = ? AND (controller_as = ? OR controller_as = ?)", sourceID, sourceRoute, finalRoute).Where(identity, name, controller, dataTable, primaryKey).Count(&sourceCount).Error; err != nil {
		return err
	}
	if err := db.Table(table).Where("id = ?", targetID).Count(&targetCount).Error; err != nil {
		return err
	}
	if err := db.Table(table).Where("id <> ? AND "+identity, targetID, name, controller, dataTable, primaryKey).Count(&duplicateCount).Error; err != nil {
		return err
	}
	if duplicateCount > 1 || (duplicateCount == 1 && sourceCount == 0) {
		return fmt.Errorf("duplicate installer identity in %s", table)
	}
	if targetCount == 0 && sourceCount == 1 {
		result := db.Table(table).Where("id = ? AND "+identity, sourceID, name, controller, dataTable, primaryKey).Update("id", targetID)
		if result.Error != nil {
			return result.Error
		}
	} else if targetCount == 1 && sourceCount == 1 && sourceID != targetID {
		result := db.Exec("DELETE FROM "+quoteIdentifier(table)+" WHERE id = ? AND "+identity, sourceID, name, controller, dataTable, primaryKey)
		if result.Error != nil {
			return result.Error
		}
	}
	if targetCount == 0 && sourceCount == 0 {
		return fmt.Errorf("installer identity missing in %s", table)
	}
	updates := map[string]any{"controller_as": finalRoute}
	if finalFields != "" {
		updates["data_fields"] = finalFields
	}
	return db.Table(table).Where("id = ? AND "+identity, targetID, name, controller, dataTable, primaryKey).Updates(updates).Error
}

func normalizeFreshOwnership(db *gorm.DB, config *conf.Configuration) error {
	adminTable := tableName(config, "admin")
	if !tableExists(db, adminTable) {
		return nil
	}
	root, err := migrationRootID(db, config)
	if err != nil {
		return err
	}
	for _, logical := range []string{"user", "attachment", "admin_log", "security_data_recycle_log", "security_sensitive_data_log", "security_data_recycle", "security_sensitive_data", "crud_log"} {
		t := tableName(config, logical)
		if tableExists(db, t) && columnExists(db, t, "admin_id") {
			if err := repairMigrationOwners(db, t, adminTable, root); err != nil {
				return err
			}
		}
	}
	userTable := tableName(config, "user")
	for _, logical := range []string{"user_money_log", "user_score_log"} {
		t := tableName(config, logical)
		if tableExists(db, t) && tableExists(db, userTable) {
			if err := db.Exec("UPDATE " + quoteIdentifier(t) + " l JOIN " + quoteIdentifier(userTable) + " u ON u.id=l.user_id SET l.admin_id=u.admin_id").Error; err != nil {
				return err
			}
		}
	}
	return nil
}
