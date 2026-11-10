package crud_helper

import (
	"testing"

	crudmodel "buildadmin-go/internal/pkg/crudmodel"
)

func TestApproveAllTreatsRequiresApprovalAsApproved(t *testing.T) {
	approved := allApprovalCategories()
	if len(approved) != len(approvalCategories) {
		t.Fatalf("allApprovalCategories size = %d, want %d", len(approved), len(approvalCategories))
	}
	for _, category := range approvalCategories {
		if !approved[category] {
			t.Fatalf("category %q was not approved", category)
		}
		if !isApprovedDiff(DiffRequiresApproval, category, approved) {
			t.Fatalf("requires-approval diff for %q was not approved", category)
		}
		if isBlockingDiff(DiffRequiresApproval, category, approved) {
			t.Fatalf("requires-approval diff for %q remained blocking", category)
		}
	}
	// rejected 永远不因 ApproveAll 变为可批准
	if isApprovedDiff(DiffRejected, ApprovalDefaults, approved) {
		t.Fatal("rejected diff was approved by allApprovalCategories")
	}
	if !isBlockingDiff(DiffRejected, "", approved) {
		t.Fatal("rejected diff stopped blocking under allApprovalCategories")
	}
}

func TestBlockedApplyChangesWithAllApprovedOnlyRejectsRejected(t *testing.T) {
	changes := []ApplyChange{
		{Field: "note", Class: DiffSafeAuto, Reason: "new nullable column"},
	}
	for _, category := range approvalCategories {
		changes = append(changes, ApplyChange{Field: "col-" + string(category), Class: DiffRequiresApproval, Category: category, Reason: "requires approval"})
	}
	changes = append(changes, ApplyChange{Field: "amount", Class: DiffRejected, Reason: "type narrowing may truncate existing data"})

	blocked := blockedApplyChanges(changes, allApprovalCategories())
	if len(blocked) != 1 {
		t.Fatalf("blocked changes = %+v, want only the rejected diff", blocked)
	}
	if blocked[0].Class != DiffRejected || blocked[0].Field != "amount" {
		t.Fatalf("blocked change = %+v, want rejected amount diff", blocked[0])
	}
}

func TestPlanBlockingErrorLenientSkipsRejected(t *testing.T) {
	rejected := []ApplyTableResult{{Table: "orders", Diffs: []ApplyChange{
		{Field: "amount", Class: DiffRejected, Reason: "type narrowing may truncate existing data"},
	}}}
	if err := planBlockingError(rejected, false, false, nil); err == nil {
		t.Fatal("严格模式下 rejected 计划未返回错误")
	}
	if err := planBlockingError(rejected, false, true, nil); err != nil {
		t.Fatalf("宽松模式下 rejected 计划仍返回错误: %v", err)
	}

	approval := []ApplyTableResult{{Table: "orders", Diffs: []ApplyChange{
		{Field: "status", Class: DiffRequiresApproval, Category: ApprovalDefaults, Reason: "default value change requires approval"},
	}}}
	// 宽松模式只放过 rejected；未批准的 requires-approval 仍然阻止计划。
	if err := planBlockingError(approval, false, true, nil); err == nil {
		t.Fatal("宽松模式不应放过未批准的 requires-approval 计划")
	}
	if err := planBlockingError(approval, false, true, allApprovalCategories()); err != nil {
		t.Fatalf("全部类别已批准时 requires-approval 计划仍返回错误: %v", err)
	}
}

// 计划阶段必须过滤引用"应用后仍不存在"的列的索引，避免 --plan 高报实际会跳过的 ADD INDEX。
func TestPlanIndexAdditionsSkipsMissingColumns(t *testing.T) {
	added := []ApplyChange{
		{Field: "idx_ok", Type: "add-index", Class: DiffSafeAuto},
		{Field: "idx_missing", Type: "add-index", Class: DiffSafeAuto},
		{Field: "idx_prefixed", Type: "add-index", Class: DiffSafeAuto},
	}
	indexes := []crudmodel.IndexSpec{
		{Name: "idx_ok", Columns: []string{"status"}},
		{Name: "idx_missing", Columns: []string{"settled"}},
		{Name: "idx_prefixed", Columns: []string{"content(64)"}},
	}
	willExist := map[string]bool{"status": true, "content": true}

	kept, skipped := planIndexAdditions(added, indexes, willExist)
	if len(kept) != 2 || kept[0].Field != "idx_ok" || kept[1].Field != "idx_prefixed" {
		t.Fatalf("kept = %+v, want idx_ok and idx_prefixed", kept)
	}
	if len(skipped) != 1 || skipped[0].Field != "idx_missing" {
		t.Fatalf("skipped = %+v, want idx_missing", skipped)
	}
	if skipped[0].Class != DiffRejected {
		t.Fatalf("skipped class = %q, want rejected", skipped[0].Class)
	}
}
