package migrations

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

type LegacyAliasStatus = core.LegacyAliasStatus

const (
	LegacyAliasMissing       = core.LegacyAliasMissing
	LegacyAliasPending       = core.LegacyAliasPending
	LegacyAliasCompleted     = core.LegacyAliasCompleted
	LegacyAliasNameCollision = core.LegacyAliasNameCollision
)

type LegacyAliasInspection = core.LegacyAliasInspection

// LegacyVersionAliases flattens aliases from the active local registry.  The
// facade owns this default so core remains independent of local migrations.
func LegacyVersionAliases() []OfficialKey {
	var aliases []OfficialKey
	for _, local := range LocalMigrations() {
		aliases = append(aliases, local.LegacyAliases...)
	}
	return append([]OfficialKey(nil), aliases...)
}

func InspectLegacyAliases(db *gorm.DB, config *conf.Configuration, aliases []OfficialKey) ([]LegacyAliasInspection, error) {
	if aliases == nil {
		aliases = LegacyVersionAliases()
	}
	return core.InspectLegacyAliases(db, config, aliases)
}

func ValidateLegacyAliases(aliases []OfficialKey) error {
	return core.ValidateLegacyAliases(aliases)
}

func PreflightLegacyAliases(db *gorm.DB, config *conf.Configuration, aliases []OfficialKey) error {
	if aliases == nil {
		aliases = LegacyVersionAliases()
	}
	return core.PreflightLegacyAliases(db, config, aliases)
}

func AdoptCompletedLegacyAliases(db *gorm.DB, config *conf.Configuration, locals []LocalMigration) (int, error) {
	return core.AdoptCompletedLegacyAliases(db, config, locals)
}

func ResolveOfficialAliasCollisions(db *gorm.DB, config *conf.Configuration, official []OfficialMigration, locals []LocalMigration) (int, error) {
	return core.ResolveOfficialAliasCollisions(db, config, official, locals)
}
