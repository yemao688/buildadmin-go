package handler

import (
	helper "go-build-admin/app/pkg/crud_helper"
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

func (h *MigrateHandler) Run(cmd *cobra.Command, args []string) error {
	report, err := migrations.Run(h.db, h.config)
	if err != nil {
		cmd.Printf("database migrate error: %v\n", err)
		return err
	}
	cmd.Printf("executed %d migrations (%d official, %d local, %d business)", report.Official+report.Local+report.Business, report.Official, report.Local, report.Business)
	if report.Seeded {
		cmd.Print(" (seeded)")
	}
	cmd.Println()
	if h.config == nil || !h.config.Crud.ApplyOnMigrate {
		cmd.Println("CRUD apply skipped (set crud.apply_on_migrate: true to enable)")
		return nil
	}
	// 部署闭环第四阶段：crud_specs 存在时幂等应用业务表结构与菜单（alter 安全子集）
	if dir := helper.DefaultSpecDir(); dir != "" {
		results, applyErr := helper.ApplySpecsFromDir(h.db, h.config, dir, helper.ApplyOptions{AdminID: 1})
		if applyErr != nil {
			cmd.Printf("CRUD apply error: %v\n", applyErr)
			return applyErr
		}
		for _, result := range results {
			if result.Action == helper.ApplyUnchanged {
				continue
			}
			cmd.Printf("CRUD apply %-9s %s\n", string(result.Action), result.Table)
		}
	}
	return nil
}

func (h *MigrateHandler) Rollback(cmd *cobra.Command, args []string) {
	//TODO:
}
