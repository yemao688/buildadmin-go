package core

import (
	"fmt"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

type LegacyAliasStatus string

const (
	LegacyAliasMissing       LegacyAliasStatus = "missing"
	LegacyAliasPending       LegacyAliasStatus = "pending"
	LegacyAliasCompleted     LegacyAliasStatus = "completed"
	LegacyAliasNameCollision LegacyAliasStatus = "name_collision"
)

type LegacyAliasInspection struct {
	Key    OfficialKey
	Status LegacyAliasStatus
}

func InspectLegacyAliases(db *gorm.DB, config *conf.Configuration, aliases []OfficialKey) ([]LegacyAliasInspection, error) {
	if err := ValidatePrefix(config); err != nil {
		return nil, err
	}
	var out []LegacyAliasInspection
	for _, key := range aliases {
		var r MigrationRecord
		q := db.Table(TableName(config, "migrations")).Where("version = ?", key.Version).First(&r)
		if q.Error == gorm.ErrRecordNotFound {
			out = append(out, LegacyAliasInspection{key, LegacyAliasMissing})
			continue
		}
		if q.Error != nil {
			return nil, q.Error
		}
		status := LegacyAliasPending
		if r.MigrationName != key.Name {
			status = LegacyAliasNameCollision
		} else if r.EndTime != nil {
			status = LegacyAliasCompleted
		}
		out = append(out, LegacyAliasInspection{key, status})
	}
	return out, nil
}

func ValidateLegacyAliases(aliases []OfficialKey) error {
	seen := map[int64]string{}
	for _, a := range aliases {
		if a.Version == 0 || a.Name == "" {
			return fmt.Errorf("invalid legacy alias")
		}
		if old, ok := seen[a.Version]; ok && old != a.Name {
			return fmt.Errorf("legacy alias version %d collision", a.Version)
		}
		seen[a.Version] = a.Name
	}
	return nil
}

func aliasLabel(key OfficialKey) string { return fmt.Sprintf("%d/%s", key.Version, key.Name) }

func PreflightLegacyAliases(db *gorm.DB, config *conf.Configuration, aliases []OfficialKey) error {
	inspections, err := InspectLegacyAliases(db, config, aliases)
	if err != nil {
		return err
	}
	for _, inspection := range inspections {
		if inspection.Status == LegacyAliasNameCollision {
			return fmt.Errorf("legacy alias %s name collision", aliasLabel(inspection.Key))
		}
	}
	return nil
}

func AdoptCompletedLegacyAliases(db *gorm.DB, config *conf.Configuration, locals []LocalMigration) (int, error) {
	if err := ValidatePrefix(config); err != nil {
		return 0, err
	}
	byAlias := map[OfficialKey]LocalMigration{}
	for _, local := range locals {
		for _, alias := range local.LegacyAliases {
			byAlias[alias] = local
		}
	}
	count := 0
	for alias, local := range byAlias {
		var old MigrationRecord
		q := db.Table(TableName(config, "migrations")).Where("version = ?", alias.Version).First(&old)
		if q.Error == gorm.ErrRecordNotFound || q.Error != nil && old.MigrationName == "" {
			if q.Error != nil && q.Error != gorm.ErrRecordNotFound {
				return count, q.Error
			}
			continue
		}
		if q.Error != nil {
			return count, q.Error
		}
		if old.MigrationName != alias.Name {
			return count, fmt.Errorf("legacy alias %s name collision", aliasLabel(alias))
		}
		if old.EndTime == nil {
			continue
		}
		if local.VerifySchema != nil {
			if err := local.VerifySchema(db, config); err != nil {
				return count, fmt.Errorf("verify alias %s schema: %w", aliasLabel(alias), err)
			}
		}
		if local.VerifyUpgradeData != nil {
			if err := local.VerifyUpgradeData(db, config); err != nil {
				return count, fmt.Errorf("verify alias %s data: %w", aliasLabel(alias), err)
			}
		}
		adopted := aliasLabel(alias)
		if err := db.Transaction(func(tx *gorm.DB) error {
			var existing LocalMigrationRecord
			r := tx.Table(TableName(config, "go_migrations")).Where("sequence = ?", local.Sequence).First(&existing)
			if r.Error == nil {
				if existing.MigrationID != local.ID || existing.Revision != local.Revision {
					return fmt.Errorf("local sequence %d collision during adoption", local.Sequence)
				}
				if existing.EndTime == nil {
					return fmt.Errorf("local migration %s is pending during adoption", local.ID)
				}
				return nil
			}
			if r.Error != gorm.ErrRecordNotFound {
				return r.Error
			}
			return tx.Table(TableName(config, "go_migrations")).Create(&LocalMigrationRecord{Sequence: local.Sequence, MigrationID: local.ID, Revision: local.Revision, StartTime: old.StartTime, EndTime: old.EndTime, AdoptedFrom: &adopted}).Error
		}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func ResolveOfficialAliasCollisions(db *gorm.DB, config *conf.Configuration, official []OfficialMigration, locals []LocalMigration) (int, error) {
	localByAlias := map[OfficialKey]LocalMigration{}
	for _, local := range locals {
		for _, alias := range local.LegacyAliases {
			localByAlias[alias] = local
		}
	}
	count := 0
	for _, migration := range official {
		local, ok := localByAlias[migration.Key]
		if !ok {
			continue
		}
		var old MigrationRecord
		q := db.Table(TableName(config, "migrations")).Where("version = ?", migration.Key.Version).First(&old)
		if q.Error == gorm.ErrRecordNotFound {
			continue
		}
		if q.Error != nil {
			return count, q.Error
		}
		if old.MigrationName != migration.Key.Name {
			return count, fmt.Errorf("official alias %s name collision", aliasLabel(migration.Key))
		}
		if old.EndTime == nil {
			return count, fmt.Errorf("official alias %s is pending", aliasLabel(migration.Key))
		}
		if local.VerifySchema != nil {
			if err := local.VerifySchema(db, config); err != nil {
				return count, err
			}
		}
		if local.VerifyUpgradeData != nil {
			if err := local.VerifyUpgradeData(db, config); err != nil {
				return count, err
			}
		}
		adopted := aliasLabel(migration.Key)
		if err := db.Transaction(func(tx *gorm.DB) error {
			var existing LocalMigrationRecord
			lookup := tx.Table(TableName(config, "go_migrations")).Where("sequence = ?", local.Sequence).First(&existing)
			if lookup.Error == nil {
				if existing.MigrationID != local.ID || existing.Revision != local.Revision || existing.EndTime == nil {
					return fmt.Errorf("local migration %s collision during alias adoption", local.ID)
				}
			} else if lookup.Error != gorm.ErrRecordNotFound {
				return lookup.Error
			} else if err := tx.Table(TableName(config, "go_migrations")).Create(&LocalMigrationRecord{Sequence: local.Sequence, MigrationID: local.ID, Revision: local.Revision, StartTime: old.StartTime, EndTime: old.EndTime, AdoptedFrom: &adopted}).Error; err != nil {
				return err
			}
			result := tx.Table(TableName(config, "migrations")).Where("version = ? AND migration_name = ?", migration.Key.Version, migration.Key.Name).Delete(&MigrationRecord{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("official alias %s delete affected %d rows", aliasLabel(migration.Key), result.RowsAffected)
			}
			return nil
		}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
