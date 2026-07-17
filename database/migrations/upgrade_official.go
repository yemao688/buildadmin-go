package migrations

import (
	"go-build-admin/database/migrations/official"
)

func OfficialMigrations() []OfficialMigration {
	return official.Migrations()
}
