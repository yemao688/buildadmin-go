package local

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func local0001Up(db *gorm.DB, config *conf.Configuration) error {
	if err := bridgeAdminStatusSchema(db, config); err != nil {
		return err
	}
	return version223(db, config)
}

func ApplyFreshOverlay(db *gorm.DB, config *conf.Configuration) error {
	if err := normalizeFreshOwnership(db, config); err != nil {
		return err
	}
	if err := normalizeFreshSecuritySeed(db, config); err != nil {
		return err
	}
	return EnsureAdminClosureSelfRows(db, config)
}

func normalizeFreshSecuritySeed(db *gorm.DB, config *conf.Configuration) error {
	if err := version232(db, config); err != nil {
		return err
	}
	recycle := core.TableName(config, "security_data_recycle")
	if core.TableExists(db, recycle) {
		if err := convergeSecurityRule(db, recycle, 1, 5, "会员", "user/User.php", "user", "id", "user/user", "auth/user", ""); err != nil {
			return err
		}
	}
	sensitive := core.TableName(config, "security_sensitive_data")
	if core.TableExists(db, sensitive) {
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
		result := db.Exec("DELETE FROM "+core.QuoteIdentifier(table)+" WHERE id = ? AND "+identity, sourceID, name, controller, dataTable, primaryKey)
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
	adminTable := core.TableName(config, "admin")
	if !core.TableExists(db, adminTable) {
		return nil
	}
	root, err := migrationRootID(db, config)
	if err != nil {
		return err
	}
	for _, logical := range []string{"user", "attachment", "admin_log", "security_data_recycle_log", "security_sensitive_data_log", "security_data_recycle", "security_sensitive_data", "crud_log"} {
		t := core.TableName(config, logical)
		if core.TableExists(db, t) && core.ColumnExists(db, t, "admin_id") {
			if err := repairMigrationOwners(db, t, adminTable, root); err != nil {
				return err
			}
		}
	}
	userTable := core.TableName(config, "user")
	for _, logical := range []string{"user_money_log", "user_score_log"} {
		t := core.TableName(config, logical)
		if core.TableExists(db, t) && core.TableExists(db, userTable) {
			if err := db.Exec("UPDATE " + core.QuoteIdentifier(t) + " l JOIN " + core.QuoteIdentifier(userTable) + " u ON u.id=l.user_id SET l.admin_id=u.admin_id").Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func localPostSeedVerify(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidateCurrentSchema(db, config); err != nil {
		return err
	}
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
	if err := rejectKnownLegacyInstallerRules(db, core.TableName(config, "security_data_recycle"), core.TableName(config, "security_sensitive_data")); err != nil {
		return err
	}
	return verifyCanonicalColumnOrder(db, config)
}

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

func verifySecuritySeedIdentity(db *gorm.DB, config *conf.Configuration, root int32) error {
	for _, check := range []struct {
		table, id, name, controllerAs, dataTable string
	}{{"security_data_recycle", "5", "会员", "user/user", "user"}, {"security_sensitive_data", "2", "会员数据", "user/user", "user"}} {
		t := core.TableName(config, check.table)
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
