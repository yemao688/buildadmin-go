package migrations

import (
	"database/sql"
	"fmt"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations/business"
	"buildadmin-go/internal/migrations/internal/core"
	"gorm.io/gorm"
)

func validateMigrationLockRelease(released sql.NullInt64) error {
	return core.ValidateMigrationLockRelease(released)
}

func WithMigrationLock(db *gorm.DB, name string, timeout time.Duration, fn func(*gorm.DB) error) error {
	return core.WithMigrationLock(db, name, timeout, fn)
}

func RunFrameworkMigrations(db *gorm.DB, config *conf.Configuration, official []OfficialMigration, framework []FrameworkMigration) (int, error) {
	return core.RunFrameworkMigrations(db, config, official, framework)
}

type Report struct {
	Official  int
	Framework int
	Business  int
	Seeded    bool
}

const migrationOrchestratorLockPrefix = "migration-orchestrator-v1:"

func migrationOrchestratorLockName(config *conf.Configuration) string {
	database, prefix := "", ""
	if config != nil {
		database = config.Database.Database
		prefix = config.Database.Prefix
	}
	name := migrationOrchestratorLockPrefix + database + ":" + prefix
	return name[:min(len(name), 64)]
}

func Run(db *gorm.DB, config *conf.Configuration) (report Report, err error) {
	err = WithMigrationLock(db, migrationOrchestratorLockName(config), 120*time.Second, func(pinned *gorm.DB) error {
		if err := ValidatePrefix(config); err != nil {
			return err
		}
		if err := PrepareUpstreamNeutralSchema(pinned, config); err != nil {
			return fmt.Errorf("legacy schema migration: %w", err)
		}
		recovery, err := DecideInstallRecovery(pinned, config)
		if err != nil {
			return fmt.Errorf("database state check: %w", err)
		}
		if recovery != InstallStrictUpgrade {
			if err := BootstrapOfficialLedger(pinned, config); err != nil {
				return fmt.Errorf("official ledger bootstrap before snapshot: %w", err)
			}
			if err := MarkSeedPending(pinned, config); err != nil {
				return fmt.Errorf("database seed marker before snapshot: %w", err)
			}
			if err := pinned.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(core.CoreModels()...); err != nil {
				return fmt.Errorf("database fresh snapshot: %w", err)
			}
		}
		if err := BootstrapOfficialLedger(pinned, config); err != nil {
			return fmt.Errorf("official ledger bootstrap: %w", err)
		}
		if err := ValidateOfficialLedgerSchema(pinned, config); err != nil {
			return fmt.Errorf("official ledger schema: %w", err)
		}
		if err := BootstrapFrameworkLedger(pinned, config); err != nil {
			return fmt.Errorf("framework ledger bootstrap: %w", err)
		}
		if err := ValidateFrameworkLedgerSchema(pinned, config); err != nil {
			return fmt.Errorf("framework ledger schema: %w", err)
		}
		if err := BootstrapBusinessLedger(pinned, config); err != nil {
			return fmt.Errorf("business ledger bootstrap: %w", err)
		}
		if err := ValidateBusinessLedgerSchema(pinned, config); err != nil {
			return fmt.Errorf("business ledger schema: %w", err)
		}
		official, frameworks := OfficialMigrations(), FrameworkMigrations()
		businessMigrations, err := business.Migrations()
		if err != nil {
			return fmt.Errorf("business migration registry: %w", err)
		}
		report.Official, err = RunOfficialMigrations(pinned, config, official)
		if err != nil {
			return fmt.Errorf("official migration: %w", err)
		}
		if err := ReconcileLegacyData(pinned, config); err != nil {
			return fmt.Errorf("official reconciliation: %w", err)
		}
		pending, err := SeedPending(pinned, config)
		if err != nil {
			return fmt.Errorf("database seed state: %w", err)
		}
		if pending {
			if err := RunOfficialFreshSeed(pinned, config); err != nil {
				return err
			}
			report.Seeded = true
		}
		report.Framework, err = RunFrameworkMigrations(pinned, config, official, frameworks)
		if err != nil {
			return fmt.Errorf("framework migration: %w", err)
		}
		report.Business, err = RunBusinessMigrations(pinned, config, businessMigrations)
		if err != nil {
			return fmt.Errorf("business migration: %w", err)
		}
		if err := FrameworkVerifyCurrent(pinned, config); err != nil {
			return fmt.Errorf("framework current validation: %w", err)
		}
		return nil
	})
	return report, err
}

func Rollback(db *gorm.DB, config *conf.Configuration, options RollbackOptions) (report RollbackReport, err error) {
	err = WithMigrationLock(db, migrationOrchestratorLockName(config), 120*time.Second, func(pinned *gorm.DB) error {
		if err := ValidatePrefix(config); err != nil {
			return err
		}
		if err := BootstrapBusinessLedger(pinned, config); err != nil {
			return fmt.Errorf("business ledger bootstrap: %w", err)
		}
		if err := ValidateBusinessLedgerSchema(pinned, config); err != nil {
			return fmt.Errorf("business ledger schema: %w", err)
		}
		list, err := business.Migrations()
		if err != nil {
			return fmt.Errorf("business migration registry: %w", err)
		}
		report, err = RollbackBusinessMigrations(pinned, config, list, options)
		if err != nil {
			return err
		}
		return nil
	})
	return report, err
}

func SetBreakpoint(db *gorm.DB, config *conf.Configuration, version uint64) error {
	return WithMigrationLock(db, migrationOrchestratorLockName(config), 120*time.Second, func(pinned *gorm.DB) error {
		if err := ValidatePrefix(config); err != nil {
			return err
		}
		if err := BootstrapBusinessLedger(pinned, config); err != nil {
			return err
		}
		if err := ValidateBusinessLedgerSchema(pinned, config); err != nil {
			return err
		}
		return SetBusinessBreakpoint(pinned, config, version)
	})
}

func ClearBreakpoint(db *gorm.DB, config *conf.Configuration) error {
	return WithMigrationLock(db, migrationOrchestratorLockName(config), 120*time.Second, func(pinned *gorm.DB) error {
		if err := ValidatePrefix(config); err != nil {
			return err
		}
		if err := BootstrapBusinessLedger(pinned, config); err != nil {
			return err
		}
		if err := ValidateBusinessLedgerSchema(pinned, config); err != nil {
			return err
		}
		return ClearBusinessBreakpoint(pinned, config)
	})
}

func GetBreakpoint(db *gorm.DB, config *conf.Configuration) (breakpoint *BusinessBreakpoint, err error) {
	err = WithMigrationLock(db, migrationOrchestratorLockName(config), 120*time.Second, func(pinned *gorm.DB) error {
		if err := ValidatePrefix(config); err != nil {
			return err
		}
		if err := BootstrapBusinessLedger(pinned, config); err != nil {
			return err
		}
		if err := ValidateBusinessLedgerSchema(pinned, config); err != nil {
			return err
		}
		breakpoint, err = ReadBusinessBreakpoint(pinned, config)
		return err
	})
	return breakpoint, err
}
