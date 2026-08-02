package handler

import (
	"fmt"
	"go-build-admin/app/middleware"
	helper "go-build-admin/app/pkg/crud_helper"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"go-build-admin/service/db"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

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
