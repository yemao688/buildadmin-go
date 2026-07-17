package migrations

import (
	"go-build-admin/database/migrations/official"
	"gorm.io/gorm"
)

type Install = official.Install

func NewInstall(sqlDB *gorm.DB) *Install {
	return official.NewInstall(sqlDB)
}
