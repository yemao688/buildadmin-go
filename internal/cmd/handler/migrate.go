package handler

import (
	"fmt"
	"strconv"

	helper "buildadmin-go/internal/pkg/crud_helper"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/database/migrations"
	"buildadmin-go/internal/infra/db"

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
	cmd.Printf("executed %d migrations (%d official, %d framework, %d business)", report.Official+report.Framework+report.Business, report.Official, report.Framework, report.Business)
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

func (h *MigrateHandler) Rollback(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: migrate rollback [business]")
	}
	if len(args) == 1 && args[0] != "business" {
		return fmt.Errorf("%s migration rollback is unsupported; official and local migrations are forward-only", args[0])
	}
	steps, err := cmd.Flags().GetUint64("steps")
	if err != nil {
		return err
	}
	toBreakpoint, err := cmd.Flags().GetBool("to-breakpoint")
	if err != nil {
		return err
	}
	report, err := migrations.Rollback(h.db, h.config, migrations.RollbackOptions{Steps: steps, ToBreakpoint: toBreakpoint})
	for _, entry := range report.Entries {
		status := "not-rolled-back"
		if entry.DownExecuted && entry.LedgerRemoved {
			status = "rolled-back"
		} else if entry.DownExecuted {
			status = "down-applied-ledger-retained"
		}
		cmd.Printf("business migration %-28s sequence=%d batch=%d %s\n", entry.ID, entry.Sequence, entry.Batch, status)
	}
	if err != nil {
		cmd.Printf("database rollback error: %v (rolled back %d, not rolled back %d)\n", err, report.RolledBack(), report.NotRolledBack())
		return err
	}
	if len(report.Entries) == 0 {
		cmd.Println("no business migrations to rollback")
		return nil
	}
	cmd.Printf("rolled back %d business migration(s)\n", report.RolledBack())
	return nil
}

func (h *MigrateHandler) SetBreakpoint(cmd *cobra.Command, args []string) error {
	sequence, err := parseBreakpointSequence(args)
	if err != nil {
		return err
	}
	if err := migrations.SetBreakpoint(h.db, h.config, sequence); err != nil {
		return fmt.Errorf("set business migration breakpoint: %w", err)
	}
	cmd.Printf("business migration breakpoint set: %d\n", sequence)
	return nil
}

func (h *MigrateHandler) ClearBreakpoint(cmd *cobra.Command, args []string) error {
	if err := migrations.ClearBreakpoint(h.db, h.config); err != nil {
		return fmt.Errorf("clear business migration breakpoint: %w", err)
	}
	cmd.Println("business migration breakpoint cleared")
	return nil
}

func (h *MigrateHandler) ListBreakpoints(cmd *cobra.Command, args []string) error {
	breakpoint, err := migrations.GetBreakpoint(h.db, h.config)
	if err != nil {
		return fmt.Errorf("list business migration breakpoint: %w", err)
	}
	if breakpoint == nil {
		cmd.Println("business migration breakpoint: none")
		return nil
	}
	cmd.Printf("business migration breakpoint: %d (set at %s)\n", breakpoint.Sequence, breakpoint.SetTime.Format("2006-01-02 15:04:05.000000"))
	return nil
}

func parseBreakpointSequence(args []string) (uint64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("usage: migrate breakpoint set <version>")
	}
	sequence, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid breakpoint version %q: %w", args[0], err)
	}
	return sequence, nil
}
