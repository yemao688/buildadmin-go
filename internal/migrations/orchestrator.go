// Package migrations 编排三轨迁移（official/framework/business）的执行顺序与状态台账。
package migrations

import (
	"database/sql"
	"fmt"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations/business"
	"buildadmin-go/internal/migrations/framework"
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
		if err := runSnapshotStage(pinned, config, recovery); err != nil {
			return err
		}
		if err := runLedgerBootstrapStage(pinned, config); err != nil {
			return err
		}
		return runTrackedStages(pinned, config, &report)
	})
	return report, err
}

// runSnapshotStage 在非 strict-upgrade 场景建立全新安装快照；strict-upgrade
// 只补建缺失的核心表。
func runSnapshotStage(pinned *gorm.DB, config *conf.Configuration, recovery InstallRecoveryState) error {
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
		if err := core.ApplyTableComments(pinned, config.Database.Prefix); err != nil {
			return fmt.Errorf("core table comments: %w", err)
		}
		return nil
	}
	if _, err := core.EnsureMissingCoreTables(pinned, config.Database.Prefix); err != nil {
		// strict-upgrade 只补建缺失的核心表（旧库可能没有 country_* 等
		// 后加表），绝不改动已存在表的结构。
		return fmt.Errorf("strict upgrade missing core tables: %w", err)
	}
	return nil
}

func runLedgerBootstrapStage(pinned *gorm.DB, config *conf.Configuration) error {
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
	return nil
}

func runTrackedStages(pinned *gorm.DB, config *conf.Configuration, recent *Report) error {
	official, frameworks := OfficialMigrations(), FrameworkMigrations()
	businessMigrations, err := business.Migrations()
	if err != nil {
		return fmt.Errorf("business migration registry: %w", err)
	}
	recent.Official, err = RunOfficialMigrations(pinned, config, official)
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
		recent.Seeded = true
	}
	// framework 迁移账本只记录一次，其 Up 里的种子因此不会在存量库重放；
	// 每次 migrate/serve 都幂等补种，使新版本新增的 framework seed 能到达
	// 已完成的库。必须在 RunFrameworkMigrations 之前执行：已记录迁移的
	// VerifySchema/VerifyUpgradeData 在 runner 内先于后续步骤运行，被删除的
	// 安全种子行要在常驻断言之前自愈。
	if err := framework.EnsureSeed(pinned, config); err != nil {
		return fmt.Errorf("framework seed: %w", err)
	}
	recent.Framework, err = RunFrameworkMigrations(pinned, config, official, frameworks)
	if err != nil {
		return fmt.Errorf("framework migration: %w", err)
	}
	recent.Business, err = RunBusinessMigrations(pinned, config, businessMigrations)
	if err != nil {
		return fmt.Errorf("business migration: %w", err)
	}
	if err := FrameworkVerifyCurrent(pinned, config); err != nil {
		return fmt.Errorf("framework current validation: %w", err)
	}
	return nil
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
