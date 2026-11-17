package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestPlanToJSONExposesApproveCommand(t *testing.T) {
	t.Parallel()
	results := []helper.ApplyTableResult{{
		Table:  "ops_order",
		Action: helper.ApplyAltered,
		Diffs: []helper.ApplyChange{
			{Field: "status", Type: "attributes", Class: helper.DiffRequiresApproval, Category: helper.ApprovalAttributes, DDL: "ALTER TABLE ops_order MODIFY status varchar(32)"},
			{Field: "legacy", Class: helper.DiffRejected, Reason: "column removal is not automatic"},
		},
		Blocked: []helper.ApplyChange{{Field: "legacy", Class: helper.DiffRejected}},
	}}
	payload := planToJSON(results, "crud_specs/ops_order.yaml", "")
	if len(payload.BlockedCategories) != 1 || payload.BlockedCategories[0] != string(helper.ApprovalAttributes) {
		t.Fatalf("blocked categories = %v", payload.BlockedCategories)
	}
	if payload.ApproveCommand != "crud:apply crud_specs/ops_order.yaml --approve=attributes" {
		t.Fatalf("approve command = %q", payload.ApproveCommand)
	}
	if payload.Tables[0].Blocked != 1 || len(payload.Tables[0].Changes) != 2 {
		t.Fatalf("table payload = %+v", payload.Tables[0])
	}
}

func TestPrintValidateJSONEnvelope(t *testing.T) {
	t.Parallel()
	path := writeTempSpec(t, `name: ops_json_demo
fields:
  id: {type: bigint, primaryKey: true}
  name: {type: varchar, length: 20}
`)
	cmd := &cobra.Command{}
	buffer := &bytes.Buffer{}
	cmd.SetOut(buffer)
	if err := printValidateJSON(cmd, []string{path}); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Validated int `json:"validated"`
			Warnings  []struct {
				Spec    string `json:"spec"`
				Message string `json:"message"`
			} `json:"warnings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &envelope); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, buffer.String())
	}
	if !envelope.OK || envelope.Data.Validated != 1 || len(envelope.Data.Warnings) != 0 {
		t.Fatalf("envelope = %+v", envelope)
	}
}

func writeTempSpec(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/spec.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// 命令先算出结构化结果、再因 rejected/失败退出时，信封必须是 ok:false 且带上
// data，executeRoot 不能再补第二条 JSON。
func TestPrintJSONErrorKeepsData(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{}
	buffer := &bytes.Buffer{}
	cmd.SetOut(buffer)
	if err := printJSONError(cmd, map[string]string{"table": "ops_order"}, errors.New("rejected changes")); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		OK    bool              `json:"ok"`
		Error string            `json:"error"`
		Data  map[string]string `json:"data"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &envelope); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if envelope.OK || envelope.Error != "rejected changes" || envelope.Data["table"] != "ops_order" {
		t.Fatalf("envelope = %+v", envelope)
	}
}
