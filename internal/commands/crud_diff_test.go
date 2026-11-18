package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"strings"
	"testing"
)

// crud:diff 的数据库路径需要 MySQL，这里只覆盖纯辅助函数：分类计数、行收集与
// approve 类别拼装。数据库行为由 helper 的 apply/plan 测试覆盖。
func diffFixture() []helper.ApplyTableResult {
	return []helper.ApplyTableResult{
		{
			Table:  "ops_order",
			Action: helper.ApplyAltered,
			Diffs: []helper.ApplyChange{
				{Field: "status", Type: "attributes", Class: helper.DiffRequiresApproval, Category: helper.ApprovalAttributes, Reason: "column attribute change requires approval", DDL: "ALTER TABLE ops_order MODIFY status varchar(32)"},
				{Field: "remark", Type: "new-column", Class: helper.DiffSafeAuto, Reason: "new nullable column", DDL: "ALTER TABLE ops_order ADD COLUMN remark varchar(64)"},
			},
		},
		{
			Table:  "ops_banner",
			Action: helper.ApplyBlocked,
			Diffs: []helper.ApplyChange{
				{Field: "legacy", Class: helper.DiffRejected, Reason: "column removal is not automatic", DDL: "ALTER TABLE ops_banner DROP COLUMN legacy"},
				{Field: "weigh", Type: "default", Class: helper.DiffRequiresApproval, Category: helper.ApprovalDefaults, Reason: "default value change requires approval"},
			},
		},
	}
}

func TestCrudDiffSummarizeApplyPlan(t *testing.T) {
	t.Parallel()
	summary := summarizeApplyPlan(diffFixture())
	want := crudDiffSummary{Tables: 2, SafeAuto: 1, RequiresApproval: 2, Rejected: 1}
	if summary != want {
		t.Fatalf("summary = %+v, want %+v", summary, want)
	}
	if empty := summarizeApplyPlan(nil); empty != (crudDiffSummary{}) {
		t.Fatalf("empty summary = %+v", empty)
	}
}

func TestCrudDiffCollectRowsOrdersByTableThenField(t *testing.T) {
	t.Parallel()
	approvals := collectDiffRows(diffFixture(), helper.DiffRequiresApproval)
	if len(approvals) != 2 {
		t.Fatalf("approval rows = %+v", approvals)
	}
	if approvals[0].Table != "ops_banner" || approvals[0].Field != "weigh" {
		t.Fatalf("rows must be ordered by table then field: %+v", approvals)
	}
	if approvals[1].Table != "ops_order" || approvals[1].DDL == "" {
		t.Fatalf("missing DDL row: %+v", approvals[1])
	}
	rejected := collectDiffRows(diffFixture(), helper.DiffRejected)
	if len(rejected) != 1 || rejected[0].Reason == "" {
		t.Fatalf("rejected rows = %+v", rejected)
	}
	if got := collectDiffRows(diffFixture(), helper.DiffSafeAuto); len(got) != 1 || got[0].Field != "remark" {
		t.Fatalf("safe-auto rows = %+v", got)
	}
}

func TestCrudDiffApprovalCategories(t *testing.T) {
	t.Parallel()
	rows := collectDiffRows(diffFixture(), helper.DiffRequiresApproval)
	categories := approvalCategories(rows)
	if len(categories) != 2 || categories[0] != string(helper.ApprovalAttributes) || categories[1] != string(helper.ApprovalDefaults) {
		t.Fatalf("categories must be deduplicated and sorted: %v", categories)
	}
	duplicated := approvalCategories([]crudDiffRow{{Category: "defaults"}, {Category: "attributes"}, {Category: "attributes"}, {Category: ""}})
	if len(duplicated) != 2 || duplicated[0] != "attributes" || duplicated[1] != "defaults" {
		t.Fatalf("dedup categories = %v", duplicated)
	}
	if len(approvalCategories(nil)) != 0 {
		t.Fatal("empty rows must yield no categories")
	}
}

func TestCrudDiffCommandRequiresOneSpec(t *testing.T) {
	t.Parallel()
	diffCmd := newCrudDiffCommand(nil)
	if err := diffCmd.ValidateArgs(nil); err == nil {
		t.Fatal("crud:diff must require exactly one spec path")
	}
	if err := diffCmd.ValidateArgs([]string{"a.yaml", "b.yaml"}); err == nil {
		t.Fatal("crud:diff must reject multiple spec paths")
	}
	if err := diffCmd.ValidateArgs([]string{"a.yaml"}); err != nil {
		t.Fatalf("single spec must be accepted: %v", err)
	}
}

func TestRenderRejectedMigrationBody(t *testing.T) {
	t.Parallel()
	body := renderRejectedMigrationBody([]crudDiffRow{{
		Table:  "ops_order",
		Field:  "legacy",
		Reason: "column removal is not automatic",
		DDL:    "ALTER TABLE `ba_ops_order` DROP COLUMN `legacy`",
	}})
	for _, want := range []string{
		"ops_order.legacy: column removal is not automatic",
		"ALTER TABLE `ba_ops_order` DROP COLUMN `legacy`",
		"TODO",
		"return nil",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "\n\t\treturn nil") {
		t.Fatalf("body must be indented for the Up function:\n%s", body)
	}
}
