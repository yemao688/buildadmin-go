package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// crudPreviewSpec 覆盖四类字段：主键、注释字典的 switch、远程下拉与时间列；
// 故意省略 formFields/columnFields，验证预览展示的是解析后的默认归属。
const crudPreviewSpec = `name: ops_preview_demo
comment: 预览示例 <b>转义</b>
dataScope:
  mode: auto
menu:
  title: 预览示例菜单
fields:
  id: {type: bigint, unsigned: true, primaryKey: true, autoIncrement: true, designType: pk, comment: ID}
  status: {type: tinyint, length: 1, null: false, default: "1", designType: switch, comment: "状态:0=禁用,1=启用"}
  user_id: {type: int, null: false, default: "0", designType: remoteSelect, comment: 会员, form: {remoteTable: user, remoteField: username}}
  create_time: {type: bigint, unsigned: true, null: false, default: "0", comment: 创建时间}
`

func writeCrudPreviewSpec(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ops_preview_demo.yaml")
	if err := os.WriteFile(path, []byte(crudPreviewSpec), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runCrudPreview(t *testing.T, specPath, outPath string) *bytes.Buffer {
	t.Helper()
	previewCmd := newCrudPreviewCommand()
	buffer := &bytes.Buffer{}
	previewCmd.SetOut(buffer)
	previewCmd.SetErr(io.Discard)
	if outPath != "" {
		if err := previewCmd.Flags().Set("out", outPath); err != nil {
			t.Fatal(err)
		}
	}
	if err := previewCmd.RunE(previewCmd, []string{specPath}); err != nil {
		t.Fatalf("crud:preview failed: %v", err)
	}
	return buffer
}

func TestCrudPreviewWritesHTML(t *testing.T) {
	t.Parallel()
	outPath := filepath.Join(t.TempDir(), "preview", "ops_preview_demo.html")
	runCrudPreview(t, writeCrudPreviewSpec(t), outPath)
	content, err := os.ReadFile(outPath) //nolint:gosec // G304: outPath 为测试内 t.TempDir 下的预览产物
	if err != nil {
		t.Fatalf("preview not written: %v", err)
	}
	page := string(content)
	for _, want := range []string{
		"<!doctype html>", "ops_preview_demo", "预览示例菜单",
		"user_id", "remoteSelect", "switch", "create_time",
		"关联 user.username", "注释字典 0=禁用,1=启用",
		"表单字段顺序（formFields）", "列表列（columnFields）",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("preview page missing %q", want)
		}
	}
	// html/template 必须转义 spec 里的原始标签，页面只做展示。
	if !strings.Contains(page, "&lt;b&gt;转义&lt;/b&gt;") {
		t.Fatal("table comment must be HTML-escaped")
	}
	if strings.Contains(page, "<b>转义</b>") {
		t.Fatal("raw HTML from spec must not reach the page")
	}
}

// TestCrudPreviewJSONOutput 不能并行：jsonMode 读全局 pflag.CommandLine，
// 测试必须临时注册并复位该旗标。
func TestCrudPreviewJSONOutput(t *testing.T) { //nolint:paralleltest // --json 依赖全局 pflag.CommandLine，无法并行
	if pflag.Lookup("json") == nil {
		pflag.Bool("json", false, "test-only global json flag")
	}
	if err := pflag.Set("json", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pflag.Set("json", "false"); err != nil {
			t.Errorf("reset --json: %v", err)
		}
	})

	output := runCrudPreview(t, writeCrudPreviewSpec(t), "")
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Table      string   `json:"table"`
			FormFields []string `json:"formFields"`
			Columns    []string `json:"columns"`
			Fields     []struct {
				Name           string `json:"name"`
				DesignType     string `json:"designType"`
				FormExcluded   bool   `json:"formExcluded"`
				ColumnExcluded bool   `json:"columnExcluded"`
			} `json:"fields"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON output: %v: %s", err, output.String())
	}
	if !envelope.OK {
		t.Fatalf("ok = false: %s", output.String())
	}
	if envelope.Data.Table != "ops_preview_demo" {
		t.Fatalf("table = %q", envelope.Data.Table)
	}
	if len(envelope.Data.Fields) != 4 {
		t.Fatalf("fields = %d, want 4", len(envelope.Data.Fields))
	}
	if len(envelope.Data.FormFields) != 2 {
		t.Fatalf("formFields = %v, want 2 entries (pk and create_time excluded)", envelope.Data.FormFields)
	}
	if len(envelope.Data.Columns) != 4 {
		t.Fatalf("columns = %v, want 4 entries", envelope.Data.Columns)
	}
	if envelope.Data.Fields[2].DesignType != "remoteSelect" {
		t.Fatalf("third field designType = %q, want remoteSelect", envelope.Data.Fields[2].DesignType)
	}
	if !envelope.Data.Fields[3].FormExcluded || envelope.Data.Fields[3].ColumnExcluded {
		t.Fatalf("create_time must be form-excluded but kept as a column: %+v", envelope.Data.Fields[3])
	}
}
