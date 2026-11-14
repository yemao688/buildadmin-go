package crudgen

import (
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type MenuAction string

const (
	MenuCreated   MenuAction = "created"
	MenuUpdated   MenuAction = "updated"
	MenuUnchanged MenuAction = "unchanged"
)

type MenuSyncResult struct {
	ID     int32
	Name   string
	Action MenuAction
}

type MenuSyncReport struct {
	Results    []MenuSyncResult
	CreatedIDs []int32
}

func GetMenuName(webDir WebDir) string {
	content := webDir.OriginalLastName
	if len(webDir.Path) > 0 {
		content = strings.Join(webDir.Path, "/") + "/" + content
	}
	return content
}

// CreateMenuWithOptionsAndRecord preserves the generator rollback contract:
// only newly created IDs are returned.
func CreateMenuWithOptionsAndRecord(db *gorm.DB, cfg *conf.Configuration, webViewsDir WebDir, tableComment string, options *MenuOptions) ([]int32, error) {
	report, err := SyncMenuWithOptionsAndRecord(db, cfg, webViewsDir, tableComment, options)
	return report.CreatedIDs, err
}

// SyncMenuWithOptionsAndRecord updates only fields owned by the generator.
// Downstream-owned icon/keepalive/extend/remark values are preserved.
func SyncMenuWithOptionsAndRecord(db *gorm.DB, cfg *conf.Configuration, webViewsDir WebDir, tableComment string, options *MenuOptions) (report MenuSyncReport, retErr error) {
	db = db.Table(cfg.Database.Prefix + "admin_rule")
	menuName := GetMenuName(webViewsDir)
	pid := int32(0)
	if options != nil {
		pid = options.Parent
	}
	if options == nil || options.Parent == 0 {
		for _, directory := range webViewsDir.Path {
			rule, result, err := upsertDirectory(db, pid, directory)
			if err != nil {
				return report, err
			}
			report.Results = append(report.Results, result)
			if result.Action == MenuCreated {
				report.CreatedIDs = append(report.CreatedIDs, rule.ID)
			}
			pid = rule.ID
		}
	}

	title := webViewsDir.OriginalLastName
	if tableComment != "" {
		title = tableComment
	}
	if options != nil && options.Title != "" {
		title = options.Title
	}
	component := strings.ReplaceAll(webViewsDir.Views, "\\", "/")
	component = strings.ReplaceAll(component, "web/src", "/src") + "/index.vue"
	menu, result, err := upsertMainMenu(db, pid, menuName, title, component, options)
	if err != nil {
		return report, err
	}
	report.Results = append(report.Results, result)
	if result.Action == MenuCreated {
		report.CreatedIDs = append(report.CreatedIDs, menu.ID)
	}

	for _, child := range menuChildren {
		name := menuName + child.Name
		button, result, err := upsertButton(db, menu.ID, name, child)
		if err != nil {
			return report, err
		}
		report.Results = append(report.Results, result)
		if result.Action == MenuCreated {
			report.CreatedIDs = append(report.CreatedIDs, button.ID)
		}
	}
	return report, nil
}

func upsertDirectory(db *gorm.DB, pid int32, name string) (model.AdminRule, MenuSyncResult, error) {
	rule := model.AdminRule{}
	result := menuDB(db).Where("pid=? AND name=? AND type=?", pid, name, "menu_dir").First(&rule)
	if result.Error == nil {
		return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuUnchanged}, nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return rule, MenuSyncResult{}, result.Error
	}
	rule = model.AdminRule{Pid: pid, Type: "menu_dir", Title: name, Name: name, Path: name, Status: "1"}
	if err := menuDB(db).Create(&rule).Error; err != nil {
		return rule, MenuSyncResult{}, err
	}
	return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuCreated}, nil
}

func upsertMainMenu(db *gorm.DB, pid int32, name, title, component string, options *MenuOptions) (model.AdminRule, MenuSyncResult, error) {
	weigh, hasWeigh := menuWeighOption(options)
	rule := model.AdminRule{}
	result := menuDB(db).Where("pid=? AND name=? AND type=?", pid, name, "menu").First(&rule)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		rule = model.AdminRule{Pid: pid, Type: "menu", Title: title, Name: name, Path: name, MenuType: "tab", Component: component, Status: "1"}
		if hasWeigh {
			rule.Weigh = weigh
		}
		if err := menuDB(db).Create(&rule).Error; err != nil {
			return rule, MenuSyncResult{}, err
		}
		return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuCreated}, nil
	}
	if result.Error != nil {
		return rule, MenuSyncResult{}, result.Error
	}
	updates := map[string]any{"title": title, "path": name, "component": component, "menu_type": "tab"}
	if hasWeigh {
		updates["weigh"] = weigh
	}
	changed := rule.Title != title || rule.Path != name || rule.Component != component || rule.MenuType != "tab"
	if hasWeigh && rule.Weigh != weigh {
		changed = true
	}
	if changed {
		if err := menuDB(db).Model(&model.AdminRule{}).Where("id=?", rule.ID).Updates(updates).Error; err != nil {
			return rule, MenuSyncResult{}, err
		}
		return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuUpdated}, nil
	}
	return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuUnchanged}, nil
}

// menuWeighOption 解出菜单权重选项：未指定时 hasWeigh 为 false，调用方保持
// 原权重不写回。
func menuWeighOption(options *MenuOptions) (weigh int32, hasWeigh bool) {
	if options == nil || options.Weigh == nil {
		return 0, false
	}
	return *options.Weigh, true
}

func upsertButton(db *gorm.DB, pid int32, name string, child Menu) (model.AdminRule, MenuSyncResult, error) {
	rule := model.AdminRule{}
	result := menuDB(db).Where("pid=? AND name=? AND type=?", pid, name, child.Type).First(&rule)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		rule = model.AdminRule{Pid: pid, Type: child.Type, Title: child.Title, Name: name, Status: child.Status}
		if err := menuDB(db).Create(&rule).Error; err != nil {
			return rule, MenuSyncResult{}, err
		}
		return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuCreated}, nil
	}
	if result.Error != nil {
		return rule, MenuSyncResult{}, result.Error
	}
	changed := rule.Pid != pid || rule.Type != child.Type || rule.Title != child.Title || rule.Status != child.Status
	if changed {
		if err := menuDB(db).Model(&model.AdminRule{}).Where("id=?", rule.ID).Updates(map[string]any{"pid": pid, "type": child.Type, "title": child.Title, "status": child.Status}).Error; err != nil {
			return rule, MenuSyncResult{}, err
		}
		return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuUpdated}, nil
	}
	return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuUnchanged}, nil
}

func menuDB(db *gorm.DB) *gorm.DB {
	table := db.Statement.Table
	return db.Session(&gorm.Session{NewDB: true}).Table(table)
}

// DeleteMenuRules 删除生成模块对应的菜单规则，可选递归删除子级并在父目录
// 变空时一并清理。它与原 AdminRuleRepository.Delete 语义一致，但只依赖
// *gorm.DB + 前缀，避免生成器反向 import admin/repository。
func DeleteMenuRules(db *gorm.DB, cfg *conf.Configuration, path string, recursion bool) error {
	table := cfg.Database.Prefix + "admin_rule"
	return db.Transaction(func(tx *gorm.DB) error {
		var deleteRule func(string) error
		deleteRule = func(name string) error {
			var adminRule model.AdminRule
			err := tx.Table(table).Where("name = ?", name).Take(&adminRule).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			var list []model.AdminRule
			if err := tx.Table(table).Where("pid = ?", adminRule.ID).Find(&list).Error; err != nil {
				return err
			}
			if recursion {
				for _, child := range list {
					if err := deleteRule(child.Name); err != nil {
						return err
					}
				}
			}
			if len(list) == 0 || recursion {
				if err := tx.Table(table).Where("id = ?", adminRule.ID).Delete(nil).Error; err != nil {
					return err
				}
				var parent model.AdminRule
				if err := tx.Table(table).Take(&parent, adminRule.Pid).Error; err == nil {
					var childCount int64
					if err := tx.Table(table).Where("pid = ?", parent.ID).Count(&childCount).Error; err != nil {
						return err
					}
					if childCount == 0 {
						return deleteRule(parent.Name)
					}
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			return nil
		}
		return deleteRule(path)
	})
}
