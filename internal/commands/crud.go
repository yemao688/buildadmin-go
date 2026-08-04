package commands

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/infra/db"
	"buildadmin-go/internal/middleware"
	helper "buildadmin-go/internal/pkg/crud_helper"
	"buildadmin-go/internal/pkg/data_scope"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newCrudValidateCommand 纯 spec 校验，不连接数据库或生成文件，也不依赖运行配置。
func newCrudValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "crud:validate <spec.yaml...>",
		Short:         "纯校验 CRUD spec，不连接数据库或生成文件",
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			warnings, err := helper.ValidateSpecs(args)
			for _, warning := range warnings {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %s\n", warning.SpecPath, warning.Message)
			}
			return err
		},
	}
}

// newCrudGenerateCommand 根据 YAML spec 生成 CRUD 文件与菜单。
func newCrudGenerateCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	generateCmd := &cobra.Command{
		Use:           "crud:generate <spec.yaml>",
		Short:         "根据 YAML spec 生成 CRUD",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.crudH.Generate(cmd, args)
		},
	}
	generateCmd.Flags().Bool("skip-menu", false, "skip menu creation")
	generateCmd.Flags().Int32("admin-id", 1, "administrator ID recorded as the generator owner")
	return generateCmd
}

// newCrudDeleteCommand 删除已生成的 CRUD 文件与菜单。
func newCrudDeleteCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	return &cobra.Command{
		Use:           "crud:delete <tableName>",
		Short:         "删除 CRUD 文件",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.crudH.Delete(cmd, args)
		},
	}
}

// newCrudApplyCommand 将 spec 声明的表结构与菜单幂等应用到目标库（部署语义，alter 安全子集）。
func newCrudApplyCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	applyCmd := &cobra.Command{
		Use:           "crud:apply [spec.yaml...]",
		Short:         "将 spec 声明的表结构与菜单幂等应用到目标库（部署语义，alter 安全子集）",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.crudH.Apply(cmd, args)
		},
	}
	applyCmd.Flags().Bool("allow-rebuild", false, "allow destructive drop-and-recreate for primary-key drift (data loss, disposable environments only)")
	applyCmd.Flags().Bool("plan", false, "print the classified CRUD plan without executing changes")
	applyCmd.Flags().String("approve", "", "approve requires-approval categories: defaults,auto-increment,type-widening,attributes, or all")
	applyCmd.Flags().Bool("skip-menu", false, "skip menu sync")
	applyCmd.Flags().Int32("admin-id", 1, "administrator ID recorded for adopted CRUD logs")
	return applyCmd
}

type CrudHandler struct {
	logger *zap.Logger
	config *conf.Configuration
	db     *gorm.DB
}

func NewCrudHandler(logger *zap.Logger, config *conf.Configuration) *CrudHandler {
	return &CrudHandler{
		logger: logger, config: config, db: db.NewDB(config, logger),
	}
}

func (h *CrudHandler) Generate(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: crud:generate <spec.yaml>")
	}
	opts, err := helper.LoadSpec(args[0])
	if err != nil {
		return fmt.Errorf("load CRUD spec error: %w", err)
	}
	opts.SkipMenu, _ = cmd.Flags().GetBool("skip-menu")
	opts.AdminID, _ = cmd.Flags().GetInt32("admin-id")
	opts.RegisterAtomicRoute = func(method, path string) {
		action := path[strings.LastIndex(path, "/")+1:]
		middleware.RegisterAtomicRoute(middleware.AtomicRoute{Route: path[:strings.LastIndex(path, "/")], Action: action, Method: method})
	}
	opts.UnregisterAtomicRoute = func(method, path string) {
		action := path[strings.LastIndex(path, "/")+1:]
		middleware.UnregisterAtomicRoute(middleware.AtomicRoute{Route: path[:strings.LastIndex(path, "/")], Action: action, Method: method})
	}
	result, err := helper.GenerateFromSpec(h.db, h.config, *opts)
	if err != nil {
		return fmt.Errorf("CRUD generation error: %w", err)
	}
	data_scope.InvalidateBusinessIdentifierCache()
	cmd.Printf("CRUD generation success (log id: %d)\n", result.LogID)
	for _, file := range result.Files {
		cmd.Printf("%s\n", file)
	}
	return nil
}

func (h *CrudHandler) Apply(cmd *cobra.Command, args []string) error {
	opts := helper.ApplyOptions{AdminID: 1}
	opts.AllowRebuild, _ = cmd.Flags().GetBool("allow-rebuild")
	opts.SkipMenu, _ = cmd.Flags().GetBool("skip-menu")
	opts.AdminID, _ = cmd.Flags().GetInt32("admin-id")
	approve, _ := cmd.Flags().GetString("approve")
	var err error
	opts.ApprovedCategories, err = helper.ParseApprovalCategories(approve)
	if err != nil {
		return fmt.Errorf("invalid --approve: %w", err)
	}
	plan, _ := cmd.Flags().GetBool("plan")
	opts.Plan = plan
	var results []helper.ApplyTableResult

	if plan {
		if len(args) == 0 {
			dir := helper.DefaultSpecDir()
			if dir == "" {
				return fmt.Errorf("no spec directory crud_specs found; pass spec files explicitly: crud:apply --plan <spec.yaml...>")
			}
			results, err = helper.PlanSpecsFromDir(h.db, h.config, dir, opts)
		} else {
			results, err = helper.PlanSpecs(h.db, h.config, args, opts)
		}
		printApplyPlan(cmd, results)
		if err != nil {
			return fmt.Errorf("CRUD plan error: %w", err)
		}
		return nil
	}

	if len(args) == 0 {
		dir := helper.DefaultSpecDir()
		if dir == "" {
			return fmt.Errorf("no spec directory crud_specs found; pass spec files explicitly: crud:apply <spec.yaml...>")
		}
		results, err = helper.ApplySpecsFromDir(h.db, h.config, dir, opts)
	} else {
		results, err = helper.ApplySpecs(h.db, h.config, args, opts)
	}
	if err != nil {
		printApplyResults(cmd, results)
		return fmt.Errorf("CRUD apply error: %w", err)
	}
	if len(results) == 0 {
		cmd.Println("CRUD apply: no specs found, nothing to do")
		return nil
	}
	printApplyResults(cmd, results)
	return nil
}

func printApplyResults(cmd *cobra.Command, results []helper.ApplyTableResult) {
	for _, result := range results {
		line := fmt.Sprintf("CRUD apply %-9s %s (log id: %d)", string(result.Action), result.Table, result.LogID)
		if len(result.Changes) > 0 {
			line += ": " + strings.Join(result.Changes, ", ")
		}
		if result.Destructive {
			line = "CRUD apply REBUILD (DESTRUCTIVE) " + result.Table + fmt.Sprintf(" (log id: %d)", result.LogID)
		}
		cmd.Println(line)
		if result.Action != helper.ApplyBlocked {
			for _, change := range result.Diffs {
				if !change.Approved {
					continue
				}
				cmd.Printf("CRUD apply approved table=%s field=%s type=%s reason=%s category=%s\n", result.Table, change.Field, change.Type, change.Reason, change.Category)
			}
		}
		if result.Action == helper.ApplyBlocked {
			for _, change := range result.Diffs {
				cmd.Printf("  [%s] %s: %s\n", change.Class, change.Field, change.Reason)
			}
		}
		for _, change := range result.Unmanaged {
			cmd.Printf("CRUD apply WARNING table=%s field=%s: %s\n", result.Table, change.Field, change.Reason)
		}
		for _, menu := range result.MenuResults {
			cmd.Printf("CRUD menu  %-9s %s\n", string(menu.Action), menu.Name)
		}
	}
}

func printApplyPlan(cmd *cobra.Command, results []helper.ApplyTableResult) {
	for _, result := range results {
		label := string(result.Action)
		if result.Destructive {
			label = "REBUILD (DESTRUCTIVE)"
		}
		cmd.Printf("CRUD plan  %-19s %s\n", label, result.Table)
		for _, change := range result.Diffs {
			cmd.Printf("  [%s] %s: %s\n", change.Class, change.Field, change.DDL)
			if change.Reason != "" {
				cmd.Printf("    reason: %s\n", change.Reason)
			}
			if change.Class == helper.DiffRequiresApproval && change.Category != "" {
				if change.Approved {
					cmd.Printf("    approval: approved via --approve=%s\n", change.Category)
				} else {
					cmd.Printf("    approval: 可被 --approve=%s 放行\n", change.Category)
				}
			} else if change.Class == helper.DiffRejected {
				cmd.Println("    rejected: 必须业务迁移")
			}
			if len(change.Unmanaged) > 0 {
				cmd.Printf("    unmanaged: %s\n", strings.Join(change.Unmanaged, ", "))
			}
		}
		for _, change := range result.Unmanaged {
			cmd.Printf("  [%s] %s: %s\n", change.Class, change.Field, change.Reason)
			cmd.Printf("    unmanaged: %s\n", strings.Join(change.Unmanaged, ", "))
		}
	}
}

func (h *CrudHandler) Delete(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: crud:delete <tableName>")
	}
	if err := helper.DeleteFromSpecWithHooks(h.db, h.config, args[0], func(method, path string) {
		action := path[strings.LastIndex(path, "/")+1:]
		middleware.UnregisterAtomicRoute(middleware.AtomicRoute{Route: path[:strings.LastIndex(path, "/")], Action: action, Method: method})
	}); err != nil {
		return fmt.Errorf("CRUD deletion error: %w", err)
	}
	data_scope.InvalidateBusinessIdentifierCache()
	cmd.Printf("CRUD deletion success: %s\n", args[0])
	return nil
}
