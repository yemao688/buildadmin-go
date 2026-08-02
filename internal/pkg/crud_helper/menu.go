package crud_helper

import (
	adminauth "go-build-admin/internal/admin/repository/auth"
	model "go-build-admin/internal/model"
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

func CreateMenu(adminRuleM *adminauth.AdminRuleRepository, webViewsDir WebDir, tableComment string) error {
	return CreateMenuWithOptions(adminRuleM, webViewsDir, tableComment, nil)
}

func CreateMenuWithOptions(adminRuleM *adminauth.AdminRuleRepository, webViewsDir WebDir, tableComment string, options *MenuOptions) error {
	_, err := CreateMenuWithOptionsAndRecord(adminRuleM, webViewsDir, tableComment, options)
	return err
}

// CreateMenuWithOptionsAndRecord preserves the generator rollback contract:
// only newly created IDs are returned.
func CreateMenuWithOptionsAndRecord(adminRuleM *adminauth.AdminRuleRepository, webViewsDir WebDir, tableComment string, options *MenuOptions) ([]int32, error) {
	report, err := SyncMenuWithOptionsAndRecord(adminRuleM, webViewsDir, tableComment, options)
	return report.CreatedIDs, err
}

// SyncMenuWithOptionsAndRecord updates only fields owned by the generator.
// Downstream-owned icon/keepalive/extend/remark values are preserved.
func SyncMenuWithOptionsAndRecord(adminRuleM *adminauth.AdminRuleRepository, webViewsDir WebDir, tableComment string, options *MenuOptions) (report MenuSyncReport, retErr error) {
	db := adminRuleM.DB().Table(adminRuleM.TableName)
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
	if result.Error != gorm.ErrRecordNotFound {
		return rule, MenuSyncResult{}, result.Error
	}
	rule = model.AdminRule{Pid: pid, Type: "menu_dir", Title: name, Name: name, Path: name, Status: "1"}
	if err := menuDB(db).Create(&rule).Error; err != nil {
		return rule, MenuSyncResult{}, err
	}
	return rule, MenuSyncResult{ID: rule.ID, Name: name, Action: MenuCreated}, nil
}

func upsertMainMenu(db *gorm.DB, pid int32, name, title, component string, options *MenuOptions) (model.AdminRule, MenuSyncResult, error) {
	rule := model.AdminRule{}
	result := menuDB(db).Where("pid=? AND name=? AND type=?", pid, name, "menu").First(&rule)
	if result.Error == gorm.ErrRecordNotFound {
		rule = model.AdminRule{Pid: pid, Type: "menu", Title: title, Name: name, Path: name, MenuType: "tab", Component: component, Status: "1"}
		if options != nil && options.Weigh != nil {
			rule.Weigh = *options.Weigh
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
	if options != nil && options.Weigh != nil {
		updates["weigh"] = *options.Weigh
	}
	changed := rule.Title != title || rule.Path != name || rule.Component != component || rule.MenuType != "tab"
	if options != nil && options.Weigh != nil && rule.Weigh != *options.Weigh {
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

func upsertButton(db *gorm.DB, pid int32, name string, child Menu) (model.AdminRule, MenuSyncResult, error) {
	rule := model.AdminRule{}
	result := menuDB(db).Where("pid=? AND name=? AND type=?", pid, name, child.Type).First(&rule)
	if result.Error == gorm.ErrRecordNotFound {
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
