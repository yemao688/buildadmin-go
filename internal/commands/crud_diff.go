package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/migrations"
	"buildadmin-go/internal/pkg/util"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// crudDiffSummary 是 crud:diff 的分类计数，退出码只看 Rejected。
type crudDiffSummary struct {
	Tables           int
	SafeAuto         int
	RequiresApproval int
	Rejected         int
}

// crudDiffRow 是一条需要人工处理的差量（requires-approval 或 rejected）。
type crudDiffRow struct {
	Table    string
	Field    string
	Category string
	Reason   string
	DDL      string
}

// newCrudDiffCommand 只读地预览"库现状 vs spec"的差量：不写库、不写文件。
func newCrudDiffCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	var writeMigration bool
	diffCmd := &cobra.Command{
		Use:           "crud:diff <spec.yaml>",
		Short:         "只读预览库现状与 spec 的差量（不写库）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.crudH.Diff(cmd, args, writeMigration)
		}),
	}
	diffCmd.Flags().BoolVar(&writeMigration, "write-migration", false, "为 rejected 变更创建 business 迁移骨架（含逐条 TODO）")
	return diffCmd
}

// Diff 输出库现状与 spec 的只读差量；requires-approval 不算失败，rejected 以
// 非零退出，便于 CI/agent 判断必须补业务迁移。
func (h *CrudHandler) Diff(cmd *cobra.Command, args []string, writeMigration bool) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: crud:diff <spec.yaml>")
	}
	specPath := args[0]
	results, planErr := helper.PlanSpecs(h.db, h.config, helper.NewTableStore(h.db, h.config), []string{specPath}, helper.ApplyOptions{AdminID: 1, Plan: true})
	// PlanSpecs 把"存在待处理差量"也报成 ApplyBlockedError；那不是读取失败，
	// 逐条明细就在 results 里，退出码由 rejected 计数决定。
	var blocked *helper.ApplyBlockedError
	if planErr != nil && !errors.As(planErr, &blocked) {
		return fmt.Errorf("CRUD diff error: %w", planErr)
	}
	summary := summarizeApplyPlan(results)
	rejectedErr := fmt.Errorf("crud:diff: %s 有 %d 条 rejected 变更，crud:apply 不会执行它们，必须写业务迁移", specPath, summary.Rejected)
	migrationPath := ""
	if writeMigration {
		rejected := collectDiffRows(results, helper.DiffRejected)
		var migrationErr error
		migrationPath, migrationErr = writeRejectedMigration(results, rejected)
		if migrationErr != nil {
			return migrationErr
		}
	}
	if jsonMode() && migrationPath != "" {
		payload := planToJSON(results, specPath, confRef())
		payload.MigrationPath = relativeToRoot(migrationPath)
		if summary.Rejected > 0 {
			if printErr := printJSONError(cmd, payload, rejectedErr); printErr != nil {
				return printErr
			}
			return rejectedErr
		}
		return printJSON(cmd, payload)
	}
	if jsonMode() {
		if summary.Rejected > 0 {
			if printErr := printJSONError(cmd, planToJSON(results, specPath, confRef()), rejectedErr); printErr != nil {
				return printErr
			}
			return rejectedErr
		}
		return printJSON(cmd, planToJSON(results, specPath, confRef()))
	}
	if migrationPath != "" {
		cmd.Printf("migration skeleton written: %s\n", relativeToRoot(migrationPath))
	}
	printCrudDiff(cmd, results, summary, specPath)
	if summary.Rejected > 0 {
		return rejectedErr
	}
	return nil
}

// summarizeApplyPlan 统计每张表的 action 与三类差量数量（纯函数）。
func summarizeApplyPlan(results []helper.ApplyTableResult) crudDiffSummary {
	summary := crudDiffSummary{Tables: len(results)}
	for _, result := range results {
		for _, change := range result.Diffs {
			switch change.Class {
			case helper.DiffSafeAuto:
				summary.SafeAuto++
			case helper.DiffRequiresApproval:
				summary.RequiresApproval++
			case helper.DiffRejected:
				summary.Rejected++
			}
		}
	}
	return summary
}

// collectDiffRows 收集指定分类的差量并按表名、字段排序。
func collectDiffRows(results []helper.ApplyTableResult, class helper.DiffClass) []crudDiffRow {
	rows := make([]crudDiffRow, 0)
	for _, result := range results {
		for _, change := range result.Diffs {
			if change.Class != class {
				continue
			}
			rows = append(rows, crudDiffRow{
				Table: result.Table, Field: change.Field, Category: string(change.Category),
				Reason: change.Reason, DDL: change.DDL,
			})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Table != rows[j].Table {
			return rows[i].Table < rows[j].Table
		}
		return rows[i].Field < rows[j].Field
	})
	return rows
}

// approvalCategories 返回去重排序后的 approve 类别，用于拼出可直接执行的命令。
func approvalCategories(rows []crudDiffRow) []string {
	seen := map[string]struct{}{}
	categories := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Category == "" {
			continue
		}
		if _, ok := seen[row.Category]; ok {
			continue
		}
		seen[row.Category] = struct{}{}
		categories = append(categories, row.Category)
	}
	sort.Strings(categories)
	return categories
}

func printCrudDiff(cmd *cobra.Command, results []helper.ApplyTableResult, summary crudDiffSummary, specPath string) {
	if len(results) == 0 {
		cmd.Println("crud:diff: no table planned for this spec")
		return
	}
	for _, result := range results {
		label := string(result.Action)
		if result.Destructive {
			label = "REBUILD (DESTRUCTIVE)"
		}
		cmd.Printf("crud:diff  %-19s %s\n", label, result.Table)
	}
	cmd.Printf("changes: safe-auto %d, requires-approval %d, rejected %d\n",
		summary.SafeAuto, summary.RequiresApproval, summary.Rejected)

	safeAuto := collectDiffRows(results, helper.DiffSafeAuto)
	if len(safeAuto) > 0 {
		cmd.Println("safe-auto (crud:apply 直接执行):")
		for _, row := range safeAuto {
			cmd.Printf("  %s.%s\n", row.Table, row.Field)
			printDiffDDL(cmd, row.DDL)
		}
	}

	approvals := collectDiffRows(results, helper.DiffRequiresApproval)
	if len(approvals) > 0 {
		cmd.Println("requires-approval:")
		for _, row := range approvals {
			cmd.Printf("  %s.%s [%s] %s\n", row.Table, row.Field, row.Category, row.Reason)
			printDiffDDL(cmd, row.DDL)
		}
		if categories := approvalCategories(approvals); len(categories) > 0 {
			cmd.Printf("next: crud:apply %s --approve=%s\n", specPath, strings.Join(categories, ","))
		}
	}

	rejected := collectDiffRows(results, helper.DiffRejected)
	if len(rejected) > 0 {
		cmd.Println("rejected (必须写业务迁移，crud:apply 不会执行):")
		for _, row := range rejected {
			cmd.Printf("  %s.%s: %s\n", row.Table, row.Field, row.Reason)
			printDiffDDL(cmd, row.DDL)
		}
	}

	if len(safeAuto) == 0 && len(approvals) == 0 && len(rejected) == 0 {
		cmd.Println("no changes: database matches the spec")
	}
}

func printDiffDDL(cmd *cobra.Command, ddl string) {
	if ddl == "" {
		return
	}
	for _, line := range strings.Split(ddl, "\n") {
		cmd.Printf("    %s\n", line)
	}
}

// writeRejectedMigration 为 rejected 变更创建 business 迁移骨架：每条变更写成
// TODO 注释（含原因与建议 DDL），Up 返回 nil 以便骨架先编译通过。
func writeRejectedMigration(results []helper.ApplyTableResult, rejected []crudDiffRow) (string, error) {
	if len(rejected) == 0 {
		return "", nil
	}
	table := results[0].Table
	dir := filepath.Join(util.RootPath(), "internal", "migrations", "business")
	list, err := migrations.BusinessMigrations()
	if err != nil {
		return "", fmt.Errorf("读取 business 迁移注册表: %w", err)
	}
	versions := make([]uint64, 0, len(list))
	for _, migration := range list {
		versions = append(versions, migration.Version)
	}
	return createBusinessMigrationWithBody(dir, table+"_rejected", versions, renderRejectedMigrationBody(rejected))
}

// renderRejectedMigrationBody 生成 Up 函数体（已按文件缩进）。
func renderRejectedMigrationBody(rejected []crudDiffRow) string {
	var builder strings.Builder
	builder.WriteString("\t\t\t// crud:diff 报告以下 rejected 变更：crud:apply 永不执行它们。\n")
	builder.WriteString("\t\t\t// 每条都要先处理存量数据，再把 spec 对齐；完成后重跑 crud:diff 应为 0 rejected。\n")
	for _, row := range rejected {
		fmt.Fprintf(&builder, "\t\t\t//\n\t\t\t// - %s.%s: %s\n", row.Table, row.Field, row.Reason)
		for _, line := range strings.Split(row.DDL, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(&builder, "\t\t\t//   %s\n", line)
		}
	}
	builder.WriteString("\t\t\t// TODO: 回填/归档数据并执行上面的 DDL，然后同步 spec\n\t\t\treturn nil")
	return builder.String()
}
