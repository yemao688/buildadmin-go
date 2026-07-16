package migrations

import (
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/model"

	"gorm.io/gorm"
)

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
