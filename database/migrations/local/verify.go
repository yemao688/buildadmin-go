package local

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func verifyStatusContract(db *gorm.DB, config *conf.Configuration) error {
	for _, logical := range []string{"admin", "user"} {
		t := core.TableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		if err := requireColumn(db, t, "status"); err != nil {
			return err
		}
		def, ok, err := core.MigrationColumnInfo(db, t, "status")
		if err != nil {
			return err
		}
		if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "varchar") || !strings.EqualFold(def.Nullable, "NO") {
			return fmt.Errorf("%s.status protocol schema invalid", t)
		}
		var invalid int64
		if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(t) + " WHERE status IS NULL OR BINARY status NOT IN ('enable','disable')").Scan(&invalid).Error; err != nil {
			return err
		}
		if invalid != 0 {
			return fmt.Errorf("%s.status contains invalid values", t)
		}
	}
	return nil
}

func verifyHierarchyContract(db *gorm.DB, config *conf.Configuration) error {
	admin := core.TableName(config, "admin")
	if err := requireTable(db, admin); err != nil {
		return err
	}
	if err := requireColumn(db, admin, "parent_id"); err != nil {
		return err
	}
	if err := requireIndexColumns(db, admin, "idx_parent_id", []string{"parent_id"}); err != nil {
		return err
	}
	closure := core.TableName(config, "admin_closure")
	if err := requireTable(db, closure); err != nil {
		return err
	}
	for _, column := range []string{"ancestor_id", "descendant_id", "depth"} {
		if !core.ColumnExists(db, closure, column) {
			return fmt.Errorf("%s.%s missing", closure, column)
		}
	}
	for _, index := range []struct {
		name    string
		columns []string
	}{
		{"PRIMARY", []string{"ancestor_id", "descendant_id"}},
		{"idx_descendant_ancestor", []string{"descendant_id", "ancestor_id"}},
		{"idx_ancestor_depth", []string{"ancestor_id", "depth"}},
	} {
		if err := requireIndexColumns(db, closure, index.name, index.columns); err != nil {
			return err
		}
	}
	if err := validateClosureSelfRows(db, config); err != nil {
		return err
	}
	lock := core.TableName(config, "admin_hierarchy_lock")
	if !core.TableExists(db, lock) {
		return fmt.Errorf("%s missing", lock)
	}
	var count int64
	if err := db.Table(lock).Where("id = 1").Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%s lock row missing", lock)
	}
	return nil
}

func verifyAttachmentContract(db *gorm.DB, config *conf.Configuration) error {
	t := core.TableName(config, "attachment")
	if err := requireTable(db, t); err != nil {
		return err
	}
	def, ok, err := core.MigrationColumnInfo(db, t, "admin_id")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s.admin_id missing", t)
	}
	if err := validOwnerColumn(def, t+".admin_id"); err != nil {
		return err
	}
	return requireIndexColumns(db, t, "idx_admin_id", []string{"admin_id"})
}

func verifyUserOwnerContract(db *gorm.DB, config *conf.Configuration) error {
	return verifyOwnerColumns(db, config, []string{"user", "user_money_log", "user_score_log"})
}

func verifySecurityOwnerContract(db *gorm.DB, config *conf.Configuration) error {
	return verifyOwnerColumns(db, config, []string{"admin_log", "security_data_recycle_log", "security_sensitive_data_log", "security_data_recycle", "security_sensitive_data", "crud_log"})
}

func verifyOwnerColumns(db *gorm.DB, config *conf.Configuration, logicalTables []string) error {
	adminTable := core.TableName(config, "admin")
	if err := requireTable(db, adminTable); err != nil {
		return err
	}
	for _, logical := range logicalTables {
		t := core.TableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		def, ok, err := core.MigrationColumnInfo(db, t, "admin_id")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s.admin_id missing", t)
		}
		if err := validOwnerColumn(def, t+".admin_id"); err != nil {
			return err
		}
		has, first, err := core.MigrationIndexInfo(db, t, "idx_admin_id")
		if err != nil {
			return err
		}
		if !has || first != "admin_id" {
			return fmt.Errorf("%s.idx_admin_id invalid", t)
		}
		if core.TableExists(db, adminTable) {
			if err := validateMigrationOwners(db, t, adminTable); err != nil {
				return err
			}
		}
	}
	if core.TableExists(db, core.TableName(config, "user")) {
		for _, logical := range []string{"user_money_log", "user_score_log"} {
			if err := validateLogOwnerMatchesUser(db, core.TableName(config, logical), core.TableName(config, "user")); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifySignedDeltaContract(db *gorm.DB, config *conf.Configuration) error {
	for _, item := range []struct{ table, column string }{{core.TableName(config, "user_money_log"), "money"}, {core.TableName(config, "user_score_log"), "score"}} {
		if err := requireTable(db, item.table); err != nil {
			return err
		}
		if err := requireColumn(db, item.table, item.column); err != nil {
			return err
		}
		def, ok, err := core.MigrationColumnInfo(db, item.table, item.column)
		if err != nil {
			return err
		}
		if !ok || !isSignedDeltaColumn(def) {
			return fmt.Errorf("%s.%s has invalid Version228 signed delta schema", item.table, item.column)
		}
	}
	return nil
}

func isSignedDeltaColumn(def core.MigrationColumn) bool {
	typ := strings.ToLower(def.ColumnType)
	return (typ == "int" || typ == "int(11)") && strings.EqualFold(def.Nullable, "NO") && def.Default != nil && *def.Default == "0"
}

func verifyTargetContract(db *gorm.DB, config *conf.Configuration) error {
	if err := requireTable(db, core.TableName(config, "admin")); err != nil {
		return err
	}
	for _, logical := range []string{"security_data_recycle_log", "security_sensitive_data_log"} {
		t := core.TableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		def, ok, err := core.MigrationColumnInfo(db, t, "target_admin_id")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s.target_admin_id missing", t)
		}
		if err := validOwnerColumn(def, t+".target_admin_id"); err != nil {
			return err
		}
		has, first, err := core.MigrationIndexInfo(db, t, "idx_target_admin_id")
		if err != nil {
			return err
		}
		if !has || first != "target_admin_id" {
			return fmt.Errorf("%s.idx_target_admin_id invalid", t)
		}
		if err := validateTargetOwners(db, t, core.TableName(config, "admin")); err != nil {
			return err
		}
	}
	return nil
}

func verifyLegacyTargetContract(db *gorm.DB, config *conf.Configuration) error {
	if err := verifyTargetContract(db, config); err != nil {
		return err
	}
	for _, logical := range []string{"security_data_recycle_log", "security_sensitive_data_log"} {
		t := core.TableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		def, ok, err := core.MigrationColumnInfo(db, t, "legacy_unrecoverable")
		if err != nil {
			return err
		}
		if !ok || !isTinyUnsignedZero(def) {
			return fmt.Errorf("%s.legacy_unrecoverable invalid", t)
		}
	}
	return nil
}

func verifyCommitContract(db *gorm.DB, config *conf.Configuration) error {
	for _, logical := range []string{"security_data_recycle_log", "security_sensitive_data_log"} {
		t := core.TableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		def, ok, err := core.MigrationColumnInfo(db, t, "is_committed")
		if err != nil {
			return err
		}
		if !ok || !isTinyUnsignedZero(def) {
			return fmt.Errorf("%s.is_committed invalid", t)
		}
	}
	return nil
}

func isTinyUnsignedZero(def core.MigrationColumn) bool {
	typ := strings.ToLower(def.ColumnType)
	return strings.HasPrefix(typ, "tinyint") && strings.Contains(typ, "unsigned") && strings.EqualFold(def.Nullable, "NO") && def.Default != nil && *def.Default == "0"
}

func verifySecurityRuleContract(db *gorm.DB, config *conf.Configuration) error {
	if err := verifyLegacyTargetContract(db, config); err != nil {
		return err
	}
	if err := verifyCommitContract(db, config); err != nil {
		return err
	}
	recycle, sensitive := core.TableName(config, "security_data_recycle"), core.TableName(config, "security_sensitive_data")
	if err := requireTable(db, recycle); err != nil {
		return err
	}
	if err := requireTable(db, sensitive); err != nil {
		return err
	}
	if err := rejectKnownLegacyInstallerRules(db, recycle, sensitive); err != nil {
		return err
	}
	return nil
}

func verifyOwnerColumnContract(db *gorm.DB, config *conf.Configuration) error {
	for _, logical := range []string{"security_data_recycle", "security_sensitive_data"} {
		table := core.TableName(config, logical)
		if !core.TableExists(db, table) {
			continue
		}
		if !core.ColumnExists(db, table, "owner_column") {
			return fmt.Errorf("%s.owner_column missing", table)
		}
	}
	return nil
}

// VerifyCurrent validates the cross-table invariants after all local and business migrations.
func VerifyCurrent(db *gorm.DB, config *conf.Configuration) error {
	root, err := migrationRootID(db, config)
	if err != nil {
		return err
	}
	if err := validateMigrationOwners(db, core.TableName(config, "user"), core.TableName(config, "admin")); err != nil {
		return err
	}
	if err := validateClosureSelfRows(db, config); err != nil {
		return err
	}
	if err := verifySecuritySeedIdentity(db, config, root); err != nil {
		return err
	}
	return rejectKnownLegacyInstallerRules(db, core.TableName(config, "security_data_recycle"), core.TableName(config, "security_sensitive_data"))
}

func rejectKnownLegacyInstallerRules(db *gorm.DB, recycle, sensitive string) error {
	for _, rule := range []struct {
		id                             int
		name, controller, route, table string
	}{
		{1, "管理员", "auth/Admin.php", "auth/admin", "admin"}, {2, "管理员日志", "auth/AdminLog.php", "auth/adminlog", "admin_log"}, {3, "菜单规则", "auth/Menu.php", "auth/rule", "admin_rule"}, {4, "系统配置项", "routine/Config.php", "routine/config", "config"}, {5, "会员", "user/User.php", "auth/user", "user"}, {6, "数据回收规则", "security/DataRecycle.php", "auth/datarecycle", "security_data_recycle"},
	} {
		var count int64
		if err := db.Table(recycle).Where("id=? AND name=? AND controller=? AND controller_as=? AND data_table=? AND primary_key=?", rule.id, rule.name, rule.controller, rule.route, rule.table, "id").Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("known Version232 recycle installer rule remains: %s id=%d", recycle, rule.id)
		}
	}
	for _, rule := range []struct {
		id                                     int
		name, controller, route, table, fields string
	}{
		{1, "管理员数据", "auth/Admin.php", "auth/admin", "admin", `{"username":"用户名","mobile":"手机","password":"密码","status":"状态"}`}, {2, "会员数据", "user/User.php", "user/user", "user", version232SensitiveUserOldFields}, {2, "会员数据", "user/User.php", "auth/user", "user", version232SensitiveUserOldFields}, {3, "管理员权限", "auth/Group.php", "auth/group", "admin_group", `{"rules":"权限规则ID"}`},
	} {
		var count int64
		if err := db.Table(sensitive).Where("id=? AND name=? AND controller=? AND controller_as=? AND data_table=? AND primary_key=? AND data_fields=?", rule.id, rule.name, rule.controller, rule.route, rule.table, "id", rule.fields).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("known Version232 sensitive installer rule remains: %s id=%d", sensitive, rule.id)
		}
	}
	return nil
}

func verifySecuritySeedIdentity(db *gorm.DB, config *conf.Configuration, root int32) error {
	for _, check := range []struct{ table, id, name, controllerAs, dataTable string }{{"security_data_recycle", "5", "会员", "user/user", "user"}, {"security_sensitive_data", "2", "会员数据", "user/user", "user"}} {
		table := core.TableName(config, check.table)
		if err := requireTable(db, table); err != nil {
			return err
		}
		var row struct {
			AdminID                       int32
			Name, ControllerAs, DataTable string
		}
		if err := db.Table(table).Where("id = ?", check.id).First(&row).Error; err != nil {
			return err
		}
		var duplicates int64
		if err := db.Table(table).Where("name=? AND controller=? AND controller_as=? AND data_table=? AND primary_key=?", check.name, "user/User.php", check.controllerAs, check.dataTable, "id").Count(&duplicates).Error; err != nil {
			return err
		}
		if duplicates != 1 {
			return fmt.Errorf("%s final installer identity count=%d", table, duplicates)
		}
		if row.AdminID != root || row.Name != check.name || row.ControllerAs != check.controllerAs || row.DataTable != check.dataTable {
			return fmt.Errorf("%s seed %s has unexpected identity or owner", table, check.id)
		}
		if check.table == "security_sensitive_data" {
			var fields string
			if err := db.Table(table).Where("id=?", check.id).Pluck("data_fields", &fields).Error; err != nil {
				return err
			}
			if strings.Contains(fields, "password") {
				return fmt.Errorf("%s final seed still exposes password", table)
			}
		}
	}
	return nil
}

func requireTable(db *gorm.DB, table string) error {
	ok, err := core.LegacyTableExists(db, table)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("required migration table %s is missing", table)
	}
	return nil
}

func requireColumn(db *gorm.DB, table, column string) error {
	ok, err := core.LegacyColumnExists(db, table, column)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("required migration column %s.%s is missing", table, column)
	}
	return nil
}

func requireIndexColumns(db *gorm.DB, table, index string, want []string) error {
	var rows []string
	if err := db.Raw("SELECT column_name FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? ORDER BY seq_in_index", table, index).Scan(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(want) {
		return fmt.Errorf("required migration index %s.%s has wrong columns", table, index)
	}
	for i := range want {
		if rows[i] != want[i] {
			return fmt.Errorf("required migration index %s.%s has wrong columns", table, index)
		}
	}
	return nil
}
