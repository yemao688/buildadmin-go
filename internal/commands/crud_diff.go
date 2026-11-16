package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"errors"
	"fmt"
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
	return &cobra.Command{
		Use:           "crud:diff <spec.yaml>",
		Short:         "只读预览库现状与 spec 的差量（不写库）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.crudH.Diff(cmd, args)
		}),
	}
}

// Diff 输出库现状与 spec 的只读差量；requires-approval 不算失败，rejected 以
// 非零退出，便于 CI/agent 判断必须补业务迁移。
func (h *CrudHandler) Diff(cmd *cobra.Command, args []string) error {
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
	if jsonMode() {
		if printErr := printJSON(cmd, planToJSON(results, specPath)); printErr != nil {
			return printErr
		}
	} else {
		printCrudDiff(cmd, results, summary, specPath)
	}
	if summary.Rejected > 0 {
		return fmt.Errorf("crud:diff: %s 有 %d 条 rejected 变更，crud:apply 不会执行它们，必须写业务迁移", specPath, summary.Rejected)
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

	if len(approvals) == 0 && len(rejected) == 0 {
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
