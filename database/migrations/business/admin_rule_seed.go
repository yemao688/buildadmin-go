package business

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

// AdminRuleSeed describes the seedable columns of admin_rule. ID and timestamp
// columns are intentionally omitted; the database owns those values.
type AdminRuleSeed struct {
	Pid       int32  `gorm:"column:pid"`
	Type      string `gorm:"column:type"`
	Title     string `gorm:"column:title"`
	Name      string `gorm:"column:name"`
	Path      string `gorm:"column:path"`
	Icon      string `gorm:"column:icon"`
	MenuType  string `gorm:"column:menu_type"`
	URL       string `gorm:"column:url"`
	Component string `gorm:"column:component"`
	Keepalive int32  `gorm:"column:keepalive"`
	Extend    string `gorm:"column:extend"`
	Remark    string `gorm:"column:remark"`
	Weigh     int32  `gorm:"column:weigh"`
	Status    string `gorm:"column:status"`

	// Children is traversal metadata, not an admin_rule column. Child Pid
	// values are replaced with the ID of the seeded parent.
	Children []AdminRuleSeed `gorm:"-"`
}

// SeedAdminRule inserts a rule and its children if their name does not exist.
// Existing rows are left unchanged, matching the framework seed contract.
func SeedAdminRule(db *gorm.DB, config *conf.Configuration, seed AdminRuleSeed) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	if strings.TrimSpace(seed.Name) == "" {
		return fmt.Errorf("admin_rule seed name must not be empty")
	}

	table := core.QuoteIdentifier(core.TableName(config, "admin_rule"))
	return seedAdminRule(db, table, seed)
}

func seedAdminRule(db *gorm.DB, table string, seed AdminRuleSeed) error {
	id, err := ensureAdminRule(db, table, seed)
	if err != nil {
		return fmt.Errorf("seed admin_rule %s: %w", seed.Name, err)
	}
	for _, child := range seed.Children {
		child.Pid = id
		if err := seedAdminRule(db, table, child); err != nil {
			return err
		}
	}
	return nil
}

func ensureAdminRule(db *gorm.DB, table string, seed AdminRuleSeed) (int32, error) {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE name = ?", seed.Name).Scan(&count).Error; err != nil {
		return 0, err
	}
	if count == 0 {
		ruleType := seed.Type
		if ruleType == "" {
			ruleType = "menu"
		}
		extend := seed.Extend
		if extend == "" {
			extend = "none"
		}
		status := seed.Status
		if status == "" {
			status = "1"
		}
		if err := db.Exec(
			"INSERT INTO "+table+" (pid, type, title, name, path, icon, menu_type, url, component, keepalive, extend, remark, weigh, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			seed.Pid, ruleType, seed.Title, seed.Name, seed.Path, seed.Icon, seed.MenuType, seed.URL, seed.Component, seed.Keepalive, extend, seed.Remark, seed.Weigh, status,
		).Error; err != nil {
			return 0, err
		}
	}

	var id int32
	if err := db.Raw("SELECT id FROM "+table+" WHERE name = ? ORDER BY id LIMIT 1", seed.Name).Scan(&id).Error; err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, fmt.Errorf("admin_rule %s was not seeded", seed.Name)
	}
	return id, nil
}
