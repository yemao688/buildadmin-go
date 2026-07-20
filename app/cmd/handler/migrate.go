package handler

import (
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/database/migrations"
	"go-build-admin/database/migrations/model"
	"go-build-admin/service/db"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MigrateHandler struct {
	logger *zap.Logger
	db     *gorm.DB
	config *conf.Configuration
}

func NewMigrateHandler(logger *zap.Logger, config *conf.Configuration) *MigrateHandler {
	return &MigrateHandler{
		logger: logger,
		db:     db.NewDB(config, logger),
		config: config,
	}
}

func (h *MigrateHandler) Run(cmd *cobra.Command, args []string) {
	if err := migrations.ValidatePrefix(h.config); err != nil {
		cmd.Println("database prefix error:", err)
		return
	}
	err := migrations.WithMigrationLock(h.db, h.config.Database.Prefix+"dual-track-migrations", 120000000000, func(pinned *gorm.DB) error {
		if err := migrations.PrepareUpstreamNeutralSchema(pinned, h.config); err != nil {
			return fmt.Errorf("legacy schema migration: %w", err)
		}
		recovery, err := migrations.DecideInstallRecovery(pinned, h.config)
		if err != nil {
			return fmt.Errorf("database state check: %w", err)
		}
		snapshot := recovery != migrations.InstallStrictUpgrade
		if snapshot {
			if err := migrations.BootstrapOfficialLedger(pinned, h.config); err != nil {
				return fmt.Errorf("official ledger bootstrap before snapshot: %w", err)
			}
			if err := migrations.MarkSeedPending(pinned, h.config); err != nil {
				return fmt.Errorf("database seed marker before snapshot: %w", err)
			}
			if err := pinned.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
				&model.AdminGroupAccess{}, &model.AdminGroup{}, &model.AdminLog{}, &model.AdminRule{}, &model.Admin{}, &model.AdminClosure{}, &model.AdminHierarchyLock{}, &model.Area{}, &model.Attachment{}, &model.Captcha{}, &model.Config{}, &model.CountryLanguage{}, &model.CountryLanguageContent{}, &model.CountryCurrency{}, &model.CrudLog{}, &model.Migrations{}, &model.SecurityDataRecycleLog{}, &model.SecurityDataRecycle{}, &model.SecuritySensitiveDataLog{}, &model.SecuritySensitiveData{}, &model.TestBuild{}, &model.Token{}, &model.UserGroup{}, &model.UserMoneyLog{}, &model.UserRule{}, &model.UserScoreLog{}, &model.User{}); err != nil {
				return fmt.Errorf("database fresh snapshot: %w", err)
			}
		}
		if err := migrations.BootstrapOfficialLedger(pinned, h.config); err != nil {
			return fmt.Errorf("official ledger bootstrap: %w", err)
		}
		if err := migrations.ValidateOfficialLedgerSchema(pinned, h.config); err != nil {
			return fmt.Errorf("official ledger schema: %w", err)
		}
		if err := migrations.BootstrapLocalLedger(pinned, h.config); err != nil {
			return fmt.Errorf("local ledger bootstrap: %w", err)
		}
		if err := migrations.ValidateLocalLedgerSchema(pinned, h.config); err != nil {
			return fmt.Errorf("local ledger schema: %w", err)
		}
		official, local := migrations.OfficialMigrations(), migrations.LocalMigrations()
		if err := migrations.PreflightLegacyAliases(pinned, h.config, migrations.LegacyVersionAliases()); err != nil {
			return fmt.Errorf("legacy alias preflight: %w", err)
		}
		if _, err := migrations.ResolveOfficialAliasCollisions(pinned, h.config, official, local); err != nil {
			return fmt.Errorf("legacy alias collision: %w", err)
		}
		officialCount, err := migrations.RunOfficialMigrations(pinned, h.config, official)
		if err != nil {
			return fmt.Errorf("official migration: %w", err)
		}
		if err := migrations.ReconcileLegacyData(pinned, h.config); err != nil {
			return fmt.Errorf("official reconciliation: %w", err)
		}
		adopted, err := migrations.AdoptCompletedLegacyAliases(pinned, h.config, local)
		if err != nil {
			return fmt.Errorf("legacy adoption: %w", err)
		}
		localCount, err := migrations.RunLocalMigrations(pinned, h.config, official, local)
		if err != nil {
			return fmt.Errorf("local migration: %w", err)
		}
		if err := migrations.ValidateCurrentSchema(pinned, h.config); err != nil {
			return fmt.Errorf("database schema validation: %w", err)
		}
		pending, err := migrations.SeedPending(pinned, h.config)
		if err != nil {
			return fmt.Errorf("database seed state: %w", err)
		}
		if pending {
			if err := migrations.RunFreshSeed(pinned, h.config, local); err != nil {
				return err
			}
		}
		cmd.Printf("executed %d migrations (%d official, %d local, %d adopted)\n", officialCount+localCount, officialCount, localCount, adopted)
		return nil
	})
	if err != nil {
		cmd.Println("database migrate error:", err)
		return
	}
	cmd.Println("database migrate success")
}

func (h *MigrateHandler) Rollback(cmd *cobra.Command, args []string) {
	//TODO:
}
