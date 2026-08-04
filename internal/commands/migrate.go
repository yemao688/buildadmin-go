package commands

import (
	"errors"
	"fmt"
	"strconv"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations"
	"buildadmin-go/internal/infra/db"
	helper "buildadmin-go/internal/pkg/crud_helper"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newMigrateCommand 构造 migrate 命令树：run/rollback/breakpoint 子命令。
func newMigrateCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:           "migrate",
		Short:         "数据库迁移",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.Run(cmd, args)
		},
	}
	runMigrateCmd := &cobra.Command{
		Use:           "run",
		Short:         "执行数据库迁移",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.Run(cmd, args)
		},
	}
	rollbackMigrateCmd := &cobra.Command{
		Use:           "rollback [business]",
		Short:         "回滚 business 数据库迁移",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.Rollback(cmd, args)
		},
	}
	rollbackMigrateCmd.Flags().Uint64("steps", 0, "rollback N business migrations; default is the latest applied batch")
	rollbackMigrateCmd.Flags().Bool("to-breakpoint", false, "rollback business migrations newer than the saved breakpoint")
	breakpointCmd := &cobra.Command{
		Use:           "breakpoint",
		Short:         "管理 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.ListBreakpoints(cmd, args)
		},
	}
	breakpointSetCmd := &cobra.Command{
		Use:           "set <version>",
		Short:         "设置 business 迁移回滚目标点",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.SetBreakpoint(cmd, args)
		},
	}
	breakpointClearCmd := &cobra.Command{
		Use:           "clear",
		Short:         "清除 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.ClearBreakpoint(cmd, args)
		},
	}
	breakpointListCmd := &cobra.Command{
		Use:           "list",
		Short:         "查看 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.migrateH.ListBreakpoints(cmd, args)
		},
	}
	breakpointCmd.AddCommand(breakpointSetCmd, breakpointClearCmd, breakpointListCmd)
	migrateCmd.AddCommand(runMigrateCmd, rollbackMigrateCmd, breakpointCmd)
	return migrateCmd
}

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
		cmd.Println("CRUD apply skipped (crud.apply_on_migrate is disabled)")
		return nil
	}
	// 部署闭环第四阶段：crud_specs 存在时幂等应用业务表结构与菜单（alter 安全子集）
	if dir := helper.DefaultSpecDir(); dir != "" {
		results, applyErr := helper.ApplySpecsFromDir(h.db, h.config, dir, helper.ApplyOptions{AdminID: 1})
		if applyErr != nil {
			cmd.Printf("CRUD apply error: %v\n", applyErr)
			var blocked *helper.ApplyBlockedError
			if errors.As(applyErr, &blocked) {
				cmd.Printf("hint: %s\n", blocked.Hint())
			}
			return applyErr
		}
		for _, result := range results {
			// 线外手改的列/属性不受 spec 管理，静默保留但必须告警，防止漂移无感知。
			for _, change := range result.Unmanaged {
				cmd.Printf("CRUD apply WARNING %s.%s: %s\n", result.Table, change.Field, change.Reason)
			}
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
