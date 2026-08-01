package framework

import "go-build-admin/database/migrations/internal/core"

func officialKeysThrough(official []core.OfficialMigration, version int64) []core.OfficialKey {
	keys := make([]core.OfficialKey, 0, len(official))
	for _, migration := range official {
		if migration.Key.Version <= version {
			keys = append(keys, migration.Key)
		}
	}
	return keys
}

func Migrations(official []core.OfficialMigration) []core.FrameworkMigration {
	requiresOfficial := []core.OfficialKey{}
	if len(official) > 0 {
		maxVersion := official[0].Key.Version
		for _, migration := range official[1:] {
			if migration.Key.Version > maxVersion {
				maxVersion = migration.Key.Version
			}
		}
		requiresOfficial = officialKeysThrough(official, maxVersion)
	}
	// An empty official list has no dependency, matching the old result.
	return []core.FrameworkMigration{{
		Version:           1,
		MigrationName:     "framework-final-seed-and-integrity",
		RequiresOfficial:  requiresOfficial,
		Up:                finalSeedAndIntegrity,
		VerifySchema:      verifyFinalTableContract,
		VerifyUpgradeData: verifyFinalDataContract,
	}}
}
