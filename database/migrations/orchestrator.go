package migrations

import (
	"fmt"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/business"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

type Report struct {
	Official int
	Local    int
	Business int
	Seeded   bool
}

func Run(db *gorm.DB, config *conf.Configuration) (report Report, err error) {
	err = WithMigrationLock(db, "migration-orchestrator-v1", 120*time.Second, func(pinned *gorm.DB) error {
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
		if err := BootstrapLocalLedger(pinned, config); err != nil {
			return fmt.Errorf("local ledger bootstrap: %w", err)
		}
		if err := ValidateLocalLedgerSchema(pinned, config); err != nil {
			return fmt.Errorf("local ledger schema: %w", err)
		}
		if err := BootstrapBusinessLedger(pinned, config); err != nil {
			return fmt.Errorf("business ledger bootstrap: %w", err)
		}
		if err := ValidateBusinessLedgerSchema(pinned, config); err != nil {
			return fmt.Errorf("business ledger schema: %w", err)
		}
		if err := ValidateBusinessBreakpointSchema(pinned, config); err != nil {
			return fmt.Errorf("business breakpoint schema: %w", err)
		}
		official, locals := OfficialMigrations(), LocalMigrations()
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
		report.Local, err = RunLocalMigrations(pinned, config, official, locals)
		if err != nil {
			return fmt.Errorf("local migration: %w", err)
		}
		report.Business, err = RunBusinessMigrations(pinned, config, businessMigrations)
		if err != nil {
			return fmt.Errorf("business migration: %w", err)
		}
		if err := LocalVerifyCurrent(pinned, config); err != nil {
			return fmt.Errorf("local current validation: %w", err)
		}
		if err := ValidateCurrentSchema(pinned, config); err != nil {
			return fmt.Errorf("database schema validation: %w", err)
		}
		return nil
	})
	return report, err
}

func Rollback(db *gorm.DB, config *conf.Configuration, options RollbackOptions) (report RollbackReport, err error) {
	err = WithMigrationLock(db, "migration-orchestrator-v1", 120*time.Second, func(pinned *gorm.DB) error {
		if err := ValidatePrefix(config); err != nil {
			return err
		}
		if err := BootstrapBusinessLedger(pinned, config); err != nil {
			return fmt.Errorf("business ledger bootstrap: %w", err)
		}
		if err := ValidateBusinessLedgerSchema(pinned, config); err != nil {
			return fmt.Errorf("business ledger schema: %w", err)
		}
		if err := ValidateBusinessBreakpointSchema(pinned, config); err != nil {
			return fmt.Errorf("business breakpoint schema: %w", err)
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

func SetBreakpoint(db *gorm.DB, config *conf.Configuration, sequence uint64) error {
	return WithMigrationLock(db, "migration-orchestrator-v1", 120*time.Second, func(pinned *gorm.DB) error {
		if err := ValidatePrefix(config); err != nil {
			return err
		}
		if err := BootstrapBusinessLedger(pinned, config); err != nil {
			return err
		}
		if err := ValidateBusinessLedgerSchema(pinned, config); err != nil {
			return err
		}
		return SetBusinessBreakpoint(pinned, config, sequence)
	})
}

func ClearBreakpoint(db *gorm.DB, config *conf.Configuration) error {
	return WithMigrationLock(db, "migration-orchestrator-v1", 120*time.Second, func(pinned *gorm.DB) error {
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
	err = WithMigrationLock(db, "migration-orchestrator-v1", 120*time.Second, func(pinned *gorm.DB) error {
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
