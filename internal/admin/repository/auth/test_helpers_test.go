package auth

import (
	"testing"

	adminmodel "go-build-admin/internal/admin/repository"
	"go-build-admin/internal/conf"
	"go-build-admin/internal/model"

	"gorm.io/gorm"
)

func hierarchyConfig(prefix string) *conf.Configuration {
	return &conf.Configuration{Database: conf.Database{Prefix: prefix}}
}

func newAdminHierarchy(prefix string) *adminmodel.AdminHierarchy {
	return adminmodel.NewAdminHierarchy(hierarchyConfig(prefix))
}

func createAdminForHierarchy(t *testing.T, db *gorm.DB, username string) model.Admin {
	t.Helper()
	a := model.Admin{Username: username, Status: "enable"}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("create admin %s: %v", username, err)
	}
	return a
}

func ensureAdminDefaults(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("ALTER TABLE `ba_admin` ALTER COLUMN `login_failure` SET DEFAULT 0").Error; err != nil {
		t.Fatalf("set login_failure default: %v", err)
	}
	if err := db.Exec("ALTER TABLE `ba_admin` MODIFY COLUMN `last_login_ip` VARCHAR(50) NOT NULL DEFAULT ''").Error; err != nil {
		t.Fatalf("set last_login_ip default: %v", err)
	}
}

func countClosureRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&model.AdminClosure{}).Count(&count).Error; err != nil {
		t.Fatalf("count closure: %v", err)
	}
	return count
}
