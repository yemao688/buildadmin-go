package handler

import (
	"bytes"
	helper "go-build-admin/internal/pkg/crud_helper"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPrintApprovalAuditAndPlanAnnotations(t *testing.T) {
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	printApplyResults(cmd, []helper.ApplyTableResult{{
		Table:  "orders",
		Action: helper.ApplyAltered,
		Diffs: []helper.ApplyChange{{
			Field:    "amount",
			Type:     "change-field-attr",
			Category: helper.ApprovalDefaults,
			Reason:   "default value change requires approval",
			Approved: true,
		}},
	}})
	printApplyPlan(cmd, []helper.ApplyTableResult{{
		Table:  "orders",
		Action: helper.ApplyBlocked,
		Diffs: []helper.ApplyChange{{
			Field:    "name",
			Type:     "change-field-attr",
			Class:    helper.DiffRequiresApproval,
			Category: helper.ApprovalTypeWidening,
			Reason:   "type widening is covered by the explicit safety matrix",
			DDL:      "ALTER TABLE `orders` MODIFY ...",
		}},
	}})

	got := output.String()
	for _, want := range []string{
		"CRUD apply approved table=orders field=amount type=change-field-attr reason=default value change requires approval category=defaults",
		"approval: 可被 --approve=type-widening 放行",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output = %q, want substring %q", got, want)
		}
	}
	t.Log("captured output:\n" + got)
}
