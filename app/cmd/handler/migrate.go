package handler

import (
	"go-build-admin/conf"
	"go-build-admin/service/db"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MigrateHandler struct {
	logger *zap.Logger
	db     *gorm.DB
}

func NewMigrateHandler(logger *zap.Logger, config *conf.Configuration) *MigrateHandler {
	return &MigrateHandler{
		logger: logger,
		db:     db.NewDB(config, logger),
	}
}

func (h *MigrateHandler) Migrate(cmd *cobra.Command, args []string) {
	err := h.db.AutoMigrate(
	// &model.Admin{},
	// &model.AdminLog{},
	)

	if err != nil {
		cmd.Println("database migrate error:", err)
	}
	cmd.Println("database migrate success")
}
