package migrations

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

var localMigrations = []LocalMigration{
	{Sequence: 1, ID: "account-status-protocol", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260714120000, "Version223"}}, Up: local0001Up, VerifySchema: verifyStatusContract, VerifyUpgradeData: verifyStatusContract},
	{Sequence: 2, ID: "admin-hierarchy", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260714130000, "Version224"}}, Up: version224, VerifySchema: verifyHierarchyContract, VerifyUpgradeData: verifyHierarchyContract},
	{Sequence: 3, ID: "attachment-owner-index", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260715000000, "Version225"}}, Up: version225, VerifySchema: verifyAttachmentContract, VerifyUpgradeData: verifyAttachmentContract},
	{Sequence: 4, ID: "user-ownership", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260716000000, "Version226"}}, Up: version226, VerifySchema: verifyUserOwnerContract, VerifyUpgradeData: verifyUserOwnerContract},
	{Sequence: 5, ID: "security-ownership", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260717000000, "Version227"}}, Up: version227, VerifySchema: verifySecurityOwnerContract, VerifyUpgradeData: verifySecurityOwnerContract},
	{Sequence: 6, ID: "signed-balance-deltas", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260718000000, "Version228"}}, Up: version228, VerifySchema: verifySignedDeltaContract, VerifyUpgradeData: verifySignedDeltaContract},
	{Sequence: 7, ID: "security-target-owner", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260719000000, "Version229"}}, Up: version229, VerifySchema: verifyTargetContract, VerifyUpgradeData: verifyTargetContract},
	{Sequence: 8, ID: "legacy-target-state", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260720000000, "Version230"}}, Up: version230, VerifySchema: verifyLegacyTargetContract, VerifyUpgradeData: verifyLegacyTargetContract},
	{Sequence: 9, ID: "security-commit-state", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260721000000, "Version231"}}, Up: version231, VerifySchema: verifyCommitContract, VerifyUpgradeData: verifyCommitContract},
	{Sequence: 10, ID: "security-rule-normalization", Revision: 1, RequiresOfficial: officialKeysThrough(20250412134127), LegacyAliases: []OfficialKey{{20260722000000, "Version232"}}, Up: version232, VerifySchema: verifySecurityRuleContract, VerifyUpgradeData: verifySecurityRuleContract},
}

func init() {
	for i := range localMigrations {
		localMigrations[i].PostSeedVerify = localPostSeedVerify
	}
}

func officialKeysThrough(version int64) []OfficialKey {
	keys := make([]OfficialKey, 0, len(officialMigrations))
	for _, migration := range officialMigrations {
		if migration.Key.Version <= version {
			keys = append(keys, migration.Key)
		}
	}
	return keys
}

func LocalMigrations() []LocalMigration { return append([]LocalMigration(nil), localMigrations...) }

func local0001Up(db *gorm.DB, config *conf.Configuration) error {
	if err := bridgeAdminStatusSchema(db, config); err != nil {
		return err
	}
	return version223(db, config)
}

func localPostSeedVerify(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidateCurrentSchema(db, config); err != nil {
		return err
	}
	root, err := migrationRootID(db, config)
	if err != nil {
		return err
	}
	if err := validateMigrationOwners(db, tableName(config, "user"), tableName(config, "admin")); err != nil {
		return err
	}
	if err := validateClosureSelfRows(db, config); err != nil {
		return err
	}
	if err := verifySecuritySeedIdentity(db, config, root); err != nil {
		return err
	}
	if err := rejectKnownLegacyInstallerRules(db, tableName(config, "security_data_recycle"), tableName(config, "security_sensitive_data")); err != nil {
		return err
	}
	return nil
}

func verifyStatusContract(db *gorm.DB, config *conf.Configuration) error {
	for _, logical := range []string{"admin", "user"} {
		t := tableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		if err := requireColumn(db, t, "status"); err != nil {
			return err
		}
		def, ok, err := migrationColumnInfo(db, t, "status")
		if err != nil {
			return err
		}
		if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "varchar") || !strings.EqualFold(def.Nullable, "NO") {
			return fmt.Errorf("%s.status protocol schema invalid", t)
		}
		var invalid int64
		if err := db.Raw("SELECT COUNT(*) FROM " + quoteIdentifier(t) + " WHERE status IS NULL OR BINARY status NOT IN ('enable','disable')").Scan(&invalid).Error; err != nil {
			return err
		}
		if invalid != 0 {
			return fmt.Errorf("%s.status contains invalid values", t)
		}
	}
	return nil
}

func verifyHierarchyContract(db *gorm.DB, config *conf.Configuration) error {
	admin := tableName(config, "admin")
	if err := requireTable(db, admin); err != nil {
		return err
	}
	if err := requireColumn(db, admin, "parent_id"); err != nil {
		return err
	}
	if err := requireIndexColumns(db, admin, "idx_parent_id", []string{"parent_id"}); err != nil {
		return err
	}
	closure := tableName(config, "admin_closure")
	if err := requireTable(db, closure); err != nil {
		return err
	}
	for _, column := range []string{"ancestor_id", "descendant_id", "depth"} {
		if !columnExists(db, closure, column) {
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
	lock := tableName(config, "admin_hierarchy_lock")
	if !tableExists(db, lock) {
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
	t := tableName(config, "attachment")
	if err := requireTable(db, t); err != nil {
		return err
	}
	def, ok, err := migrationColumnInfo(db, t, "admin_id")
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
	adminTable := tableName(config, "admin")
	if err := requireTable(db, adminTable); err != nil {
		return err
	}
	for _, logical := range logicalTables {
		t := tableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		def, ok, err := migrationColumnInfo(db, t, "admin_id")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s.admin_id missing", t)
		}
		if err := validOwnerColumn(def, t+".admin_id"); err != nil {
			return err
		}
		has, first, err := migrationIndexInfo(db, t, "idx_admin_id")
		if err != nil {
			return err
		}
		if !has || first != "admin_id" {
			return fmt.Errorf("%s.idx_admin_id invalid", t)
		}
		if tableExists(db, adminTable) {
			if err := validateMigrationOwners(db, t, adminTable); err != nil {
				return err
			}
		}
	}
	if tableExists(db, tableName(config, "user")) {
		for _, logical := range []string{"user_money_log", "user_score_log"} {
			if err := validateLogOwnerMatchesUser(db, tableName(config, logical), tableName(config, "user")); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifySignedDeltaContract(db *gorm.DB, config *conf.Configuration) error {
	for _, item := range []struct{ table, column string }{{tableName(config, "user_money_log"), "money"}, {tableName(config, "user_score_log"), "score"}} {
		if err := requireTable(db, item.table); err != nil {
			return err
		}
		if err := requireColumn(db, item.table, item.column); err != nil {
			return err
		}
		def, ok, err := migrationColumnInfo(db, item.table, item.column)
		if err != nil {
			return err
		}
		if !ok || !isSignedDeltaColumn(def) {
			return fmt.Errorf("%s.%s has invalid Version228 signed delta schema", item.table, item.column)
		}
	}
	return nil
}

func isSignedDeltaColumn(def migrationColumn) bool {
	typ := strings.ToLower(def.ColumnType)
	return (typ == "int" || typ == "int(11)") && strings.EqualFold(def.Nullable, "NO") && def.Default != nil && *def.Default == "0"
}

func verifyTargetContract(db *gorm.DB, config *conf.Configuration) error {
	if err := requireTable(db, tableName(config, "admin")); err != nil {
		return err
	}
	for _, logical := range []string{"security_data_recycle_log", "security_sensitive_data_log"} {
		t := tableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		def, ok, err := migrationColumnInfo(db, t, "target_admin_id")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s.target_admin_id missing", t)
		}
		if err := validOwnerColumn(def, t+".target_admin_id"); err != nil {
			return err
		}
		has, first, err := migrationIndexInfo(db, t, "idx_target_admin_id")
		if err != nil {
			return err
		}
		if !has || first != "target_admin_id" {
			return fmt.Errorf("%s.idx_target_admin_id invalid", t)
		}
		if err := validateTargetOwners(db, t, tableName(config, "admin")); err != nil {
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
		t := tableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		def, ok, err := migrationColumnInfo(db, t, "legacy_unrecoverable")
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
		t := tableName(config, logical)
		if err := requireTable(db, t); err != nil {
			return err
		}
		def, ok, err := migrationColumnInfo(db, t, "is_committed")
		if err != nil {
			return err
		}
		if !ok || !isTinyUnsignedZero(def) {
			return fmt.Errorf("%s.is_committed invalid", t)
		}
	}
	return nil
}

func isTinyUnsignedZero(def migrationColumn) bool {
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
	recycle, sensitive := tableName(config, "security_data_recycle"), tableName(config, "security_sensitive_data")
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

// 0010 is an upgrade verifier. Seed rows are intentionally checked only by
// post-seed verification; an upgraded database may have no default rules or
// may contain unrelated user-created rules.
func rejectKnownLegacyInstallerRules(db *gorm.DB, recycle, sensitive string) error {
	recycleRules := []struct {
		id                             int
		name, controller, route, table string
	}{
		{1, "管理员", "auth/Admin.php", "auth/admin", "admin"}, {2, "管理员日志", "auth/AdminLog.php", "auth/adminlog", "admin_log"},
		{3, "菜单规则", "auth/Menu.php", "auth/rule", "admin_rule"}, {4, "系统配置项", "routine/Config.php", "routine/config", "config"},
		{5, "会员", "user/User.php", "auth/user", "user"}, {6, "数据回收规则", "security/DataRecycle.php", "security/datarecycle", "security_data_recycle"},
	}
	for _, rule := range recycleRules {
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
		{1, "管理员数据", "auth/Admin.php", "auth/admin", "admin", `{"username":"用户名","mobile":"手机","password":"密码","status":"状态"}`},
		{2, "会员数据", "user/User.php", "user/user", "user", version232SensitiveUserOldFields},
		// Keep rejecting the pre-232 local route as a defensive compatibility check.
		{2, "会员数据", "user/User.php", "auth/user", "user", version232SensitiveUserOldFields},
		{3, "管理员权限", "auth/Group.php", "auth/group", "admin_group", `{"rules":"权限规则ID"}`},
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

func requireTable(db *gorm.DB, table string) error {
	ok, err := legacyTableExists(db, table)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("required migration table %s is missing", table)
	}
	return nil
}

func requireColumn(db *gorm.DB, table, column string) error {
	ok, err := legacyColumnExists(db, table, column)
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

func validateClosureSelfRows(db *gorm.DB, config *conf.Configuration) error {
	closure, admin := tableName(config, "admin_closure"), tableName(config, "admin")
	if !tableExists(db, closure) || !tableExists(db, admin) {
		return nil
	}
	var missing int64
	if err := db.Raw("SELECT COUNT(*) FROM " + quoteIdentifier(admin) + " a LEFT JOIN " + quoteIdentifier(closure) + " c ON c.ancestor_id=a.id AND c.descendant_id=a.id AND c.depth=0 WHERE c.ancestor_id IS NULL").Scan(&missing).Error; err != nil {
		return err
	}
	if missing != 0 {
		return fmt.Errorf("admin closure missing %d self row(s)", missing)
	}
	return nil
}

func verifySecuritySeedIdentity(db *gorm.DB, config *conf.Configuration, root int32) error {
	for _, check := range []struct {
		table, id, name, controllerAs, dataTable string
	}{{"security_data_recycle", "5", "会员", "user/user", "user"}, {"security_sensitive_data", "2", "会员数据", "user/user", "user"}} {
		t := tableName(config, check.table)
		if err := requireTable(db, t); err != nil {
			return err
		}
		var row struct {
			AdminID                       int32
			Name, ControllerAs, DataTable string
		}
		if err := db.Table(t).Where("id = ?", check.id).First(&row).Error; err != nil {
			return err
		}
		var duplicates int64
		if err := db.Table(t).Where("name=? AND controller=? AND controller_as=? AND data_table=? AND primary_key=?", check.name, "user/User.php", check.controllerAs, check.dataTable, "id").Count(&duplicates).Error; err != nil {
			return err
		}
		if duplicates != 1 {
			return fmt.Errorf("%s final installer identity count=%d", t, duplicates)
		}
		if row.AdminID != root || row.Name != check.name || row.ControllerAs != check.controllerAs || row.DataTable != check.dataTable {
			return fmt.Errorf("%s seed %s has unexpected identity or owner", t, check.id)
		}
		if check.table == "security_sensitive_data" {
			var fields string
			if err := db.Table(t).Where("id=?", check.id).Pluck("data_fields", &fields).Error; err != nil {
				return err
			}
			if strings.Contains(fields, "password") {
				return fmt.Errorf("%s final seed still exposes password", t)
			}
		}
	}
	return nil
}
