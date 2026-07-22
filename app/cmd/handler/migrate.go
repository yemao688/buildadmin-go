package handler

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations"
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
	report, err := migrations.Run(h.db, h.config)
	if err != nil {
		cmd.Printf("database migrate error: %v\n", err)
		return
	}
	cmd.Printf("executed %d migrations (%d official, %d local, %d adopted)", report.Official+report.Local, report.Official, report.Local, report.Adopted)
	if report.Seeded {
		cmd.Print(" (seeded)")
	}
	cmd.Println()
}

func (h *MigrateHandler) Rollback(cmd *cobra.Command, args []string) {
	//TODO:
}
