package crud_helper

import (
	crudmodel "buildadmin-go/internal/model"
	model "buildadmin-go/internal/admin/repository"
	"strings"
	"testing"
)

func TestParseApprovalCategories(t *testing.T) {
	approved, err := ParseApprovalCategories("defaults, type-widening,defaults")
	if err != nil {
		t.Fatal(err)
	}
	if len(approved) != 2 || !approved[ApprovalDefaults] || !approved[ApprovalTypeWidening] {
		t.Fatalf("approved categories = %#v", approved)
	}

	all, err := ParseApprovalCategories("all")
	if err != nil || len(all) != 4 {
		t.Fatalf("all categories = %#v, err = %v", all, err)
	}
	for _, category := range approvalCategories {
		if !all[category] {
			t.Fatalf("all did not include %q", category)
		}
	}

	empty, err := ParseApprovalCategories("")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty approval = %#v, err = %v", empty, err)
	}

	if _, err := ParseApprovalCategories("defaults,unknown"); err == nil || !strings.Contains(err.Error(), "defaults, auto-increment, type-widening, attributes, all") {
		t.Fatalf("invalid category error = %v", err)
	}
}

func TestApprovalCategoriesAreSetAtAllRequiresApprovalDiffPoints(t *testing.T) {
	cases := []struct {
		name     string
		field    crudmodel.Field
		column   model.Column
		category ApprovalCategory
	}{
		{
			name:     "defaults",
			field:    crudmodel.Field{Name: "amount", Type: "decimal", Length: 10, Precision: 2, DefaultType: "INPUT", Default: "2"},
			column:   alterTestColumn("amount", "decimal(10,2)", "NO", "1", "", ""),
			category: ApprovalDefaults,
		},
		{
			name:     "auto-increment",
			field:    crudmodel.Field{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true},
			column:   alterTestColumn("id", "bigint", "NO", nil, "", ""),
			category: ApprovalAutoIncrement,
		},
		{
			name:     "type-widening",
			field:    crudmodel.Field{Name: "name", Type: "varchar", Length: 40},
			column:   alterTestColumn("name", "varchar(20)", "NO", nil, "", ""),
			category: ApprovalTypeWidening,
		},
		{
			name:     "attributes fallback",
			field:    crudmodel.Field{Name: "name", Type: "varchar", Length: 20},
			column:   alterTestColumn("name", "varchar(20)", "NO", nil, "", ""),
			category: ApprovalAttributes,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, category, _ := classifyFieldDiff(tc.field, tc.column, false)
			if category != tc.category {
				t.Fatalf("category = %q, want %q", category, tc.category)
			}
		})
	}
}

func TestApprovalCategoryPropagatesToApplyChange(t *testing.T) {
	diff := AlterDiff{
		Field:    crudmodel.Field{Name: "name", Type: "varchar", Length: 40},
		Change:   crudmodel.ChangeField{Type: "change-field-attr"},
		Class:    DiffRequiresApproval,
		Category: ApprovalTypeWidening,
		Reason:   "type widening is covered by the explicit safety matrix",
	}
	changes := applyChangesFromDiffs([]AlterDiff{diff}, map[ApprovalCategory]bool{ApprovalTypeWidening: true})
	if len(changes) != 1 || changes[0].Category != ApprovalTypeWidening || !changes[0].Approved {
		t.Fatalf("apply changes = %+v", changes)
	}
}

func TestApprovalBlockingMatrix(t *testing.T) {
	approval := AlterDiff{Class: DiffRequiresApproval, Category: ApprovalDefaults}
	rejected := AlterDiff{Class: DiffRejected, Category: ApprovalDefaults}
	approved := map[ApprovalCategory]bool{ApprovalDefaults: true}

	if firstBlockingDiff([]AlterDiff{approval}, nil) == nil {
		t.Fatal("unapproved requires-approval diff was not blocked")
	}
	if firstBlockingDiff([]AlterDiff{approval}, approved) != nil {
		t.Fatal("approved requires-approval diff remained blocked")
	}
	if firstBlockingDiff([]AlterDiff{rejected}, approved) == nil {
		t.Fatal("rejected diff was approved")
	}

	approvedPlan := []ApplyTableResult{{Table: "orders", Diffs: []ApplyChange{{Field: "amount", Class: DiffRequiresApproval, Category: ApprovalDefaults, Reason: "default value change requires approval"}}}}
	if err := planBlockingError(approvedPlan, false, approved); err != nil {
		t.Fatalf("approved plan was blocked: %v", err)
	}
	if err := planBlockingError(approvedPlan, false, nil); err == nil || !strings.Contains(err.Error(), "--approve=defaults") {
		t.Fatalf("unapproved plan error = %v", err)
	}

	mixed := []ApplyTableResult{{Table: "orders", Diffs: []ApplyChange{
		{Field: "amount", Class: DiffRequiresApproval, Category: ApprovalDefaults, Reason: "default value change requires approval"},
		{Field: "status", Class: DiffRejected, Reason: "unsigned attribute changes are rejected"},
	}}}
	err := planBlockingError(mixed, false, approved)
	if err == nil || strings.Contains(err.Error(), "--approve=defaults") || !strings.Contains(err.Error(), "unsigned attribute changes are rejected") {
		t.Fatalf("mixed plan error = %v", err)
	}
}

func TestApprovalMissingPreservesDefaultBlockingBehavior(t *testing.T) {
	change := ApplyChange{Class: DiffRequiresApproval, Category: ApprovalTypeWidening}
	if len(blockedApplyChanges([]ApplyChange{change}, nil)) != 1 {
		t.Fatal("requires-approval diff without --approve was not blocked")
	}
	if len(blockedApplyChanges([]ApplyChange{{Class: DiffRejected, Category: ApprovalTypeWidening}}, map[ApprovalCategory]bool{ApprovalTypeWidening: true})) != 1 {
		t.Fatal("rejected diff was affected by --approve")
	}
}
