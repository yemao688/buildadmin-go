package commands

import (
	"buildadmin-go/internal/pkg/data_scope"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

// runCrudList 直接调用 RunE 而不是 Execute：cobra 的 OnInitialize 会触发
// initConfig 读取进程 os.Args，在测试进程里会误判为未豁免并退出。
func runCrudList(t *testing.T, dir string) (string, error) {
	t.Helper()
	listCmd := newCrudListCommand(nil)
	buffer := &bytes.Buffer{}
	listCmd.SetOut(buffer)
	listCmd.SetErr(buffer)
	if err := listCmd.Flags().Set("dir", dir); err != nil {
		t.Fatal(err)
	}
	err := listCmd.RunE(listCmd, nil)
	return buffer.String(), err
}

func writeSpecSkeleton(t *testing.T, dir, table, comment string) {
	t.Helper()
	path := filepath.Join(dir, table+".yaml")
	if err := os.WriteFile(path, []byte(renderSpecSkeleton(table, comment)), 0o644); err != nil { //nolint:gosec // G306: 测试内临时 spec
		t.Fatal(err)
	}
}

// writeMinimalSpec 写一个没有 menu/comment 的 spec，用于覆盖 "-" 展示分支。
func writeMinimalSpec(t *testing.T, dir, table string) {
	t.Helper()
	content := `name: ` + table + `
fields:
  id: {type: bigint, unsigned: true, primaryKey: true, autoIncrement: true, designType: pk}
  name: {type: varchar, length: 30, null: true}
  create_time: {type: bigint, unsigned: true, null: false, default: "0"}
  update_time: {type: bigint, unsigned: true, null: false, default: "0"}
`
	path := filepath.Join(dir, table+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // G306: 测试内临时 spec
		t.Fatal(err)
	}
}

func TestCrudListSortsAndCountsSpecs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpecSkeleton(t, dir, "ops_banner", "运营轮播")
	writeMinimalSpec(t, dir, "admin_note")
	if err := os.WriteFile(filepath.Join(dir, "schema.json"), []byte("{}"), 0o644); err != nil { //nolint:gosec // G306: 测试内临时文件
		t.Fatal(err)
	}
	output, err := runCrudList(t, dir)
	if err != nil {
		t.Fatalf("crud:list failed: %v\n%s", err, output)
	}
	lines := make([]string, 0, 4)
	for _, line := range bytes.Split([]byte(output), []byte("\n")) {
		if len(line) > 0 {
			lines = append(lines, string(line))
		}
	}
	if len(lines) != 4 {
		t.Fatalf("expected header + 2 rows + footer, got %d lines:\n%s", len(lines), output)
	}
	// 按表名排序：admin_note 在 ops_banner 前。
	if !bytes.HasPrefix([]byte(lines[1]), []byte("admin_note")) {
		t.Fatalf("first row must be admin_note, got %q", lines[1])
	}
	if !bytes.HasPrefix([]byte(lines[2]), []byte("ops_banner")) {
		t.Fatalf("second row must be ops_banner, got %q", lines[2])
	}
	// 没有 menu 的 spec 菜单列显示 "-"。
	if fields := bytes.Fields([]byte(lines[1])); len(fields) < 5 || string(fields[2]) != "-" {
		t.Fatalf("missing menu title must render as -, got %q", lines[1])
	}
	if !bytes.HasPrefix([]byte(lines[3]), []byte("2 spec(s) in ")) {
		t.Fatalf("footer = %q", lines[3])
	}
}

func TestCrudListJSONEnvelope(t *testing.T) { //nolint:paralleltest // 依赖全局 pflag.CommandLine 注入 --json，必须串行
	// 不并行：jsonMode() 读全局 --json，测试内临时打开它。
	enableJSONMode(t)
	dir := t.TempDir()
	writeSpecSkeleton(t, dir, "ops_banner", "运营轮播")
	writeMinimalSpec(t, dir, "admin_note")
	output, err := runCrudList(t, dir)
	if err != nil {
		t.Fatalf("crud:list --json failed: %v\n%s", err, output)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Count int `json:"count"`
			Specs []struct {
				Table       string `json:"table"`
				Comment     string `json:"comment"`
				Menu        string `json:"menu"`
				DataScope   string `json:"dataScope"`
				Fields      int    `json:"fields"`
				InheritFrom string `json:"inheritFrom"`
			} `json:"specs"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, output)
	}
	if !envelope.OK || envelope.Data.Count != 2 || len(envelope.Data.Specs) != 2 {
		t.Fatalf("envelope = %+v", envelope)
	}
	if envelope.Data.Specs[0].Table != "admin_note" || envelope.Data.Specs[1].Table != "ops_banner" {
		t.Fatalf("specs must be sorted by table: %+v", envelope.Data.Specs)
	}
	note := envelope.Data.Specs[0]
	if note.Menu != "-" || note.Comment != "-" || note.Fields != 4 || note.InheritFrom != "-" {
		t.Fatalf("minimal spec summary = %+v", note)
	}
	banner := envelope.Data.Specs[1]
	if banner.Menu != "运营轮播" || banner.DataScope != "auto" || banner.Fields != 5 {
		t.Fatalf("skeleton summary = %+v", banner)
	}
}

func TestCrudListReportsUnreadableSpec(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpecSkeleton(t, dir, "ops_banner", "运营轮播")
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte("name: broken\nfields: []\n"), 0o644); err != nil { //nolint:gosec // G306: 测试内临时 spec
		t.Fatal(err)
	}
	output, err := runCrudList(t, dir)
	if err == nil {
		t.Fatalf("broken spec must fail at the end:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("broken.yaml")) {
		t.Fatalf("failure summary must name the broken spec:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("ops_banner")) {
		t.Fatalf("readable specs must still be listed:\n%s", output)
	}
}

func TestCrudListScopeLabels(t *testing.T) {
	t.Parallel()
	if got := dataScopeLabel(nil); got != "auto" {
		t.Fatalf("nil scope mode = %q", got)
	}
	if got := inheritFromLabel(nil); got != "-" {
		t.Fatalf("nil scope inherit = %q", got)
	}
	parent := &data_scope.Config{Mode: data_scope.ModeRequired, InheritFrom: &data_scope.InheritRef{Table: "user", ByColumn: "admin_id"}}
	if got := inheritFromLabel(parent); got != "user(admin_id)" {
		t.Fatalf("inherit label = %q", got)
	}
	if got := dataScopeLabel(parent); got != "required" {
		t.Fatalf("scope label = %q", got)
	}
	noColumn := &data_scope.Config{InheritFrom: &data_scope.InheritRef{Table: "user"}}
	if got := inheritFromLabel(noColumn); got != "user" {
		t.Fatalf("inherit label without column = %q", got)
	}
}

// enableJSONMode 用临时 pflag.CommandLine 提供 --json：直接调用 RunE 时 Execute
// 未注册该旗标。结束后换回原 FlagSet，避免污染同包其它测试（重复注册会 panic）。
func enableJSONMode(t *testing.T) {
	t.Helper()
	original := pflag.CommandLine
	fresh := pflag.NewFlagSet("commands-test", pflag.ContinueOnError)
	fresh.Bool("json", true, "machine-readable JSON output")
	pflag.CommandLine = fresh
	t.Cleanup(func() { pflag.CommandLine = original })
}
