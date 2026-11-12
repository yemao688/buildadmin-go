package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"buildadmin-go/internal/conf"
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/infra/db"
	"buildadmin-go/internal/migrations"

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
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.migrateH.Run(cmd, args)
		}),
	}
	addMigrateRunFlags(migrateCmd)
	runMigrateCmd := &cobra.Command{
		Use:           "run",
		Short:         "执行数据库迁移",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.migrateH.Run(cmd, args)
		}),
	}
	addMigrateRunFlags(runMigrateCmd)
	rollbackMigrateCmd := &cobra.Command{
		Use:           "rollback [business]",
		Short:         "回滚 business 数据库迁移",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.migrateH.Rollback(cmd, args)
		}),
	}
	rollbackMigrateCmd.Flags().Uint64("steps", 0, "rollback N business migrations; default is the latest applied business migration")
	rollbackMigrateCmd.Flags().Bool("to-breakpoint", false, "rollback business migrations newer than the saved breakpoint")
	breakpointCmd := &cobra.Command{
		Use:           "breakpoint",
		Short:         "管理 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.migrateH.ListBreakpoints(cmd, args)
		}),
	}
	breakpointSetCmd := &cobra.Command{
		Use:           "set <version>",
		Short:         "设置 business 迁移回滚目标点",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.migrateH.SetBreakpoint(cmd, args)
		}),
	}
	breakpointClearCmd := &cobra.Command{
		Use:           "clear",
		Short:         "清除 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.migrateH.ClearBreakpoint(cmd, args)
		}),
	}
	breakpointListCmd := &cobra.Command{
		Use:           "list",
		Short:         "查看 business 迁移回滚目标点",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.migrateH.ListBreakpoints(cmd, args)
		}),
	}
	breakpointCmd.AddCommand(breakpointSetCmd, breakpointClearCmd, breakpointListCmd)
	migrateCmd.AddCommand(runMigrateCmd, rollbackMigrateCmd, breakpointCmd, newMigrateNewCommand())
	return migrateCmd
}

// addMigrateRunFlags 给 migrate 与 migrate run 挂相同的执行开关。
func addMigrateRunFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("plan", false, "只读检查待执行的迁移与 spec 差量，不写数据库")
	cmd.Flags().Bool("strict", false, "rejected 或未批准的差量视为失败并以非零退出")
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
	plan, err := cmd.Flags().GetBool("plan")
	if err != nil {
		return err
	}
	if plan {
		return h.Plan(cmd, args)
	}
	strict, err := cmd.Flags().GetBool("strict")
	if err != nil {
		return err
	}

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
		results, applyErr := helper.ApplySpecsFromDir(h.db, h.config, helper.NewTableStore(h.db, h.config), dir, helper.ApplyOptions{AdminID: 1, ApproveAll: true, AllowRejected: !strict})
		if applyErr != nil {
			printMigrateDiagnostics(cmd, results)
			cmd.Printf("CRUD apply error: %v\n", applyErr)
			var blocked *helper.ApplyBlockedError
			if errors.As(applyErr, &blocked) {
				cmd.Printf("hint: %s\n", blocked.Hint())
			}
			return applyErr
		}
		printMigrateResults(cmd, results)
	}
	return nil
}

// Plan 只读输出安装状态、迁移进度与 spec 差量；--strict 下 blocked 差量以失败退出。
func (h *MigrateHandler) Plan(cmd *cobra.Command, args []string) error {
	strict, err := cmd.Flags().GetBool("strict")
	if err != nil {
		return err
	}
	report, err := migrations.Status(h.db, h.config)
	if err != nil {
		return err
	}
	if report.State != migrations.InstallStrictUpgrade {
		printDatabaseInstallGuidance(report.State)
		return fmt.Errorf("database is not installed (state: %s); nothing to plan", report.State)
	}
	printMigrationStatus(report)
	if strict {
		// 迁移未跑完时 spec 差量基于旧 schema，strict 计划必须先把库升到最新。
		if pending, incomplete := migrationTotals(report); pending > 0 || incomplete > 0 {
			return fmt.Errorf("database migrations are not up to date (pending %d, incomplete %d); run migrate before planning", pending, incomplete)
		}
	}

	dir := helper.DefaultSpecDir()
	if dir == "" {
		cmd.Println("[database] 未找到 crud_specs 目录，跳过 spec 差量检查")
		return nil
	}
	// --plan 是发版前检查 rejected 的唯一事实源：rejected 必须让它失败，
	// 不能因为没带 --strict 就悄悄退出 0。
	results, planErr := helper.PlanSpecsFromDir(h.db, h.config, helper.NewTableStore(h.db, h.config), dir, helper.ApplyOptions{AdminID: 1, ApproveAll: true, AllowRejected: false, Plan: true})
	printPlanResults(cmd, results)
	if planErr != nil {
		return fmt.Errorf("CRUD plan error: %w", planErr)
	}
	return nil
}

// printMigrateDiagnostics 输出未被 spec 管理的差异（note）与 blocked 差量（WARNING）；两者都不阻断执行。
func printMigrateDiagnostics(cmd *cobra.Command, results []helper.ApplyTableResult) {
	for _, result := range results {
		for _, change := range result.Blocked {
			cmd.Printf("CRUD apply WARNING %s.%s: %s；%s\n", result.Table, change.Field, change.Reason, blockedChangeHint)
		}
	}
	printUnmanagedNotes(cmd.Printf, results, unmanagedNoteOptions{Prefix: "CRUD apply "})
}

// printMigrateResults 输出 diagnostics 与每张表的 apply 动作。
func printMigrateResults(cmd *cobra.Command, results []helper.ApplyTableResult) {
	printMigrateDiagnostics(cmd, results)
	for _, result := range results {
		if result.Action == helper.ApplyUnchanged {
			continue
		}
		cmd.Printf("CRUD apply %-9s %s\n", string(result.Action), result.Table)
	}
}

// printPlanResults 输出每张表的计划动作、逐差量分类与告警；--plan 是审计视图，unmanaged 保留完整明细。
func printPlanResults(cmd *cobra.Command, results []helper.ApplyTableResult) {
	for _, result := range results {
		cmd.Printf("[database] plan %-9s %s\n", string(result.Action), result.Table)
		for _, diff := range result.Diffs {
			line := fmt.Sprintf("[database] plan %s.%s: %s", result.Table, diff.Field, string(diff.Class))
			if diff.Category != "" {
				line += " (" + string(diff.Category) + ")"
			}
			if diff.Reason != "" {
				line += ": " + diff.Reason
			}
			cmd.Println(line)
		}
		for _, change := range result.Unmanaged {
			line := fmt.Sprintf("[database] note %s.%s: %s", result.Table, change.Field, change.Reason)
			if details := strings.Join(change.Unmanaged, ", "); details != "" {
				line += " (" + details + ")"
			}
			cmd.Println(line)
		}
		for _, change := range result.Blocked {
			cmd.Printf("[database] WARNING %s.%s: %s；%s\n", result.Table, change.Field, change.Reason, blockedChangeHint)
		}
	}
}

func (h *MigrateHandler) Rollback(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: migrate rollback [business]")
	}
	if len(args) == 1 && args[0] != "business" {
		return fmt.Errorf("%s migration rollback is unsupported; official and framework migrations are forward-only", args[0])
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
		cmd.Printf("business migration %-28s sequence=%d %s\n", entry.ID, entry.Sequence, status)
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
