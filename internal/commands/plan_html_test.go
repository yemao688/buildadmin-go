package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePlanHTMLRendersDiffAndCommand(t *testing.T) {
	t.Parallel()
	payload := planToJSON([]helper.ApplyTableResult{{
		Table:  "ops_order",
		Action: helper.ApplyAltered,
		Diffs: []helper.ApplyChange{
			{Field: "status", Class: helper.DiffRequiresApproval, Category: helper.ApprovalAttributes, DDL: "ALTER TABLE ops_order MODIFY status varchar(32)", Reason: "column attribute change requires approval"},
			{Field: "legacy", Class: helper.DiffRejected, Reason: `column removal <script>alert(1)</script> is not automatic`},
		},
		Blocked: []helper.ApplyChange{{Field: "legacy", Class: helper.DiffRejected}},
	}}, "crud_specs/", "")
	path := filepath.Join(t.TempDir(), "plan.html")
	if err := writePlanHTML(path, payload, "crud_specs/", "demo"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path) //nolint:gosec // G304: path 为测试内 t.TempDir 夹具路径
	if err != nil {
		t.Fatal(err)
	}
	page := string(content)
	for _, want := range []string{"ops_order", "requires-approval", "rejected", "crud:apply crud_specs/ --approve=attributes", "需先写业务迁移"} {
		if !strings.Contains(page, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Fatal("reasons must be HTML-escaped")
	}
}

func TestPlanHTMLMetaRiskLevels(t *testing.T) {
	t.Parallel()
	ok := planHTMLMetaFrom(planJSON{Tables: []planTableJSON{{Table: "a"}}}, "", "")
	if ok.Risk != "ok" || ok.Blocked != 0 || ok.Rejected != 0 {
		t.Fatalf("clean plan = %+v", ok)
	}
	warn := planHTMLMetaFrom(planJSON{Tables: []planTableJSON{{Table: "a", Changes: []planChangeJSON{{Class: "requires-approval", Category: "attributes"}}}}}, "", "")
	if warn.Risk != "warn" || warn.Blocked != 1 {
		t.Fatalf("approval plan = %+v", warn)
	}
	bad := planHTMLMetaFrom(planJSON{Tables: []planTableJSON{{Table: "a", Changes: []planChangeJSON{{Class: "rejected"}}}}}, "", "")
	if bad.Risk != "bad" || bad.Rejected != 1 {
		t.Fatalf("rejected plan = %+v", bad)
	}
}

// 头部「待批准」只统计一次：table.Blocked 与逐变更统计不能重复累加。
func TestPlanHTMLMetaCountsBlockedOnce(t *testing.T) {
	t.Parallel()
	payload := planJSON{Tables: []planTableJSON{{
		Table:   "ops_order",
		Blocked: 2,
		Changes: []planChangeJSON{
			{Field: "a", Class: "requires-approval", Category: "defaults"},
			{Field: "b", Class: "requires-approval", Category: "attributes", Approved: true},
			{Field: "c", Class: "rejected"},
		},
	}}}
	meta := planHTMLMetaFrom(payload, "crud_specs/ops_order.yaml", "demo")
	if meta.Blocked != 1 || meta.Rejected != 1 {
		t.Fatalf("blocked=%d rejected=%d, want 1/1", meta.Blocked, meta.Rejected)
	}
}
