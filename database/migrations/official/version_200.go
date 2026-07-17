package official

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"go-build-admin/database/migrations/model"

	"gorm.io/gorm"
)

func version200(db *gorm.DB, config *conf.Configuration) error {
	t := core.TableName(config, "admin_rule")
	if !core.TableExists(db, t) {
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
	t := core.TableName(config, "user")
	if !core.TableExists(db, t) {
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
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(t) + " DROP INDEX " + core.QuoteIdentifier(idx)).Error; err != nil {
			return err
		}
	}
	return nil
}

func version202(db *gorm.DB, config *conf.Configuration) error {
	t := core.TableName(config, "admin_rule")
	if !core.TableExists(db, t) {
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
		groups := core.TableName(config, "admin_group")
		if core.TableExists(db, groups) {
			if err := db.Exec("UPDATE `"+groups+"` SET rules=CONCAT_WS(',', NULLIF(rules,''), ?) WHERE FIND_IN_SET(?, rules)>0 AND FIND_IN_SET(?, rules)=0", button.ID, dashboardID, button.ID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func version205(db *gorm.DB, config *conf.Configuration) error {
	var cfgs []model.Config
	result := db.Table(core.TableName(config, "config")).Where("name = ?", "config_quick_entrance").Find(&cfgs)
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
		result := db.Table(core.TableName(config, "admin_rule")).Where("path = ?", path).Find(&rules)
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
	return db.Table(core.TableName(config, "config")).Where("id = ?", cfg.ID).Update("value", string(value)).Error
}

func validateDashboardRuleCount(count int) error {
	if count > 1 {
		return fmt.Errorf("multiple dashboard rules found")
	}
	return nil
}
