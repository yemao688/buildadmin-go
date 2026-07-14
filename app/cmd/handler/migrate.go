package handler

import (
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
	// Legacy table/column normalization must precede AutoMigrate: otherwise
	// GORM may create the replacement table and leave old data behind.
	if err := migrations.PrepareLegacySchema(h.db, h.config); err != nil {
		cmd.Println("legacy schema migration error:", err)
		return
	}
	fresh, err := migrations.IsFreshDatabase(h.db, h.config)
	if err != nil {
		cmd.Println("database state check error:", err)
		return
	}

	err = h.db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
		&model.AdminGroupAccess{},
		&model.AdminGroup{},
		&model.AdminLog{},
		&model.AdminRule{},
		&model.Admin{},
		&model.Area{},
		&model.Attachment{},
		&model.Captcha{},
		&model.Config{},
		&model.CrudLog{},
		&model.Migrations{},
		&model.SecurityDataRecycleLog{},
		&model.SecurityDataRecycle{},
		&model.SecuritySensitiveDataLog{},
		&model.SecuritySensitiveData{},
		&model.TestBuild{},
		&model.Token{},
		&model.UserGroup{},
		&model.UserMoneyLog{},
		&model.UserRule{},
		&model.UserScoreLog{},
		&model.User{},
	)
	if err != nil {
		cmd.Println("database migrate error:", err)
		return
	}
	if fresh {
		if err := migrations.MarkSeedPending(h.db, h.config); err != nil {
			cmd.Println("database seed marker error:", err)
			return
		}
	}
	if err := migrations.ValidateCurrentSchema(h.db, h.config); err != nil {
		cmd.Println("database schema validation error:", err)
		return
	}
	// 版本化迁移（处理 AutoMigrate 无法完成的列类型变更）
	upgradeCount, err := migrations.RunVersionMigrations(h.db, h.config)
	if err != nil {
		cmd.Println("version migration error:", err)
		return
	}
	if err := migrations.ReconcileLegacyData(h.db, h.config); err != nil {
		cmd.Println("legacy data reconciliation error:", err)
		return
	}
	if upgradeCount > 0 {
		cmd.Printf("executed %d version migrations\n", upgradeCount)
	}

	pending, err := migrations.SeedPending(h.db, h.config)
	if err != nil {
		cmd.Println("database seed state error:", err)
		return
	}
	if pending {
		install := migrations.NewInstall(h.db)
		if err := install.InsertData(); err != nil {
			cmd.Println("database seed error:", err)
			return
		}
		if err := migrations.MarkSeedCompleted(h.db, h.config); err != nil {
			cmd.Println("database seed marker error:", err)
			return
		}
		cmd.Println("data seed completed")
	}

	cmd.Println("database migrate success")
}

func (h *MigrateHandler) Rollback(cmd *cobra.Command, args []string) {
	//TODO:
}
