package local

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"go-build-admin/database/migrations/model"

	"gorm.io/gorm"
)

const version232SensitiveUserOldFields = `{"username":"用户名","mobile":"手机号","password":"密码","status":"状态","email":"邮箱地址"}`

func version232(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	recycle := core.TableName(config, "security_data_recycle")
	if core.TableExists(db, recycle) {
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

	sensitive := core.TableName(config, "security_sensitive_data")
	if core.TableExists(db, sensitive) {
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
