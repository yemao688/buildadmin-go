package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/util"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newCrudPreviewCommand 把 CRUD spec 渲染成只读 HTML 预览：字段表格、表单
// 字段顺序和列表列都来自 spec 本身，用来看清每个字段选了哪种组件。只读文件、
// 不连数据库、不生成任何产物。
func newCrudPreviewCommand() *cobra.Command {
	var outPath string
	previewCmd := &cobra.Command{
		Use:           "crud:preview <spec.yaml>",
		Short:         "把 CRUD spec 渲染成只读 HTML 预览（不连数据库）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			options, err := helper.LoadSpec(args[0])
			if err != nil {
				return fmt.Errorf("load CRUD spec error: %w", err)
			}
			preview := buildCrudPreview(options)
			if jsonMode() {
				// --json 只给结构化字段清单，不写 HTML：机器读清单，人看页面。
				return printJSON(cmd, preview)
			}
			path := outPath
			if path == "" {
				path = filepath.Join(util.RootPath(), "runtime", "preview", preview.Table+".html")
			}
			if err := writeCrudPreviewHTML(path, preview); err != nil {
				return err
			}
			cmd.Printf("preview created: %s\n", path)
			cmd.Println("open it and check the component choices (designType / formFields / columnFields) before crud:generate")
			return nil
		},
	}
	previewCmd.Flags().StringVar(&outPath, "out", "", "HTML 输出路径，默认 runtime/preview/<table>.html")
	return previewCmd
}

// crudPreviewData 是 HTML 与 --json 共用的预览结果：字段顺序、表单/列表归属
// 和字典/关联来源都在构建阶段定型，模板只做展示。
type crudPreviewData struct {
	Table      string             `json:"table"`
	Comment    string             `json:"comment,omitempty"`
	MenuTitle  string             `json:"menuTitle,omitempty"`
	DataScope  string             `json:"dataScope"`
	FormFields []string           `json:"formFields"`
	Columns    []string           `json:"columns"`
	Fields     []crudPreviewField `json:"fields"`
}

// crudPreviewField 是一个字段的预览行；Excluded 表示该字段被排除在表单或
// 列表之外（默认规则由 spec 解析阶段给出）。
type crudPreviewField struct {
	Name           string `json:"name"`
	Title          string `json:"title"`
	SQLType        string `json:"type"`
	DesignType     string `json:"designType"`
	Nullable       bool   `json:"nullable"`
	Default        string `json:"default,omitempty"`
	Comment        string `json:"comment,omitempty"`
	PrimaryKey     bool   `json:"primaryKey,omitempty"`
	FormExcluded   bool   `json:"formExcluded"`
	ColumnExcluded bool   `json:"columnExcluded"`
	Source         string `json:"source,omitempty"`
}

// crudPreviewTemplate 是自包含的单文件页面：样式内联、不引用外部资源，
// 宽表格在窄屏上横向滚动而不撑破页面。
var crudPreviewTemplate = template.Must(template.New("crud-preview").Funcs(template.FuncMap{
	"yesno": func(value bool) string {
		if value {
			return "是"
		}
		return "否"
	},
	"dash": func(value string) string {
		if strings.TrimSpace(value) == "" {
			return "-"
		}
		return value
	},
}).Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Table}} · CRUD 预览</title>
<style>
  :root { color-scheme: light dark; --fg:#1c1c1e; --muted:#6b7280; --line:#e5e7eb; --bg:#ffffff; --panel:#f9fafb; --ok:#047857; --okbg:#ecfdf5; }
  @media (prefers-color-scheme: dark) { :root { --fg:#e5e7eb; --muted:#9ca3af; --line:#374151; --bg:#0f1115; --panel:#161922; --ok:#34d399; --okbg:#0b2a20; } }
  * { box-sizing: border-box; }
  body { margin:0; padding:24px; background:var(--bg); color:var(--fg); font:14px/1.55 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif; }
  h1 { font-size:20px; margin:0 0 4px; }
  .sub { color:var(--muted); margin-bottom:20px; }
  .meta { display:flex; flex-wrap:wrap; gap:12px 28px; padding:14px 16px; background:var(--panel); border:1px solid var(--line); border-radius:10px; margin-bottom:20px; }
  .meta div span { display:block; color:var(--muted); font-size:12px; }
  .card { border:1px solid var(--line); border-radius:10px; margin-bottom:18px; overflow:hidden; }
  .card h2 { font-size:15px; margin:0; padding:12px 16px; background:var(--panel); border-bottom:1px solid var(--line); }
  .scroll { overflow-x:auto; }
  table { width:100%; border-collapse:collapse; }
  th, td { text-align:left; padding:9px 12px; border-bottom:1px solid var(--line); vertical-align:top; white-space:nowrap; }
  td.wrap { white-space:normal; min-width:180px; }
  th { font-size:12px; color:var(--muted); font-weight:500; }
  tr:last-child td { border-bottom:none; }
  code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; font-size:12px; background:var(--panel); padding:1px 5px; border-radius:4px; word-break:break-all; }
  .badge { display:inline-block; padding:1px 8px; border-radius:999px; font-size:12px; font-weight:500; white-space:nowrap; }
  .badge.on { color:var(--ok); background:var(--okbg); }
  .badge.off { color:var(--muted); background:var(--panel); }
  .flow { margin:0; padding:12px 16px 12px 36px; }
  .flow li { margin:2px 0; }
  .empty { color:var(--muted); font-size:13px; padding:12px 16px; }
  .foot { color:var(--muted); font-size:12px; margin-top:18px; }
</style>
</head>
<body>
<h1>{{.Table}}</h1>
<div class="sub">{{if .Comment}}{{.Comment}}{{else}}（无表注释）{{end}}</div>
<div class="meta">
  <div><span>菜单标题</span>{{.MenuTitle}}</div>
  <div><span>数据权限 mode</span>{{.DataScope}}</div>
  <div><span>字段数</span>{{len .Fields}}</div>
</div>

<div class="card">
  <h2>字段</h2>
  <div class="scroll">
  <table>
    <thead><tr><th>字段名</th><th>标题</th><th>SQL 类型</th><th>designType</th><th>可空</th><th>默认值</th><th>表单</th><th>列表</th><th>字典 / 关联来源</th><th>注释</th></tr></thead>
    <tbody>
    {{range .Fields}}
      <tr>
        <td><code>{{.Name}}</code>{{if .PrimaryKey}} <span class="badge off">PK</span>{{end}}</td>
        <td>{{.Title}}</td>
        <td><code>{{.SQLType}}</code></td>
        <td>{{dash .DesignType}}</td>
        <td>{{yesno .Nullable}}</td>
        <td>{{dash .Default}}</td>
        <td>{{if .FormExcluded}}<span class="badge off">排除</span>{{else}}<span class="badge on">包含</span>{{end}}</td>
        <td>{{if .ColumnExcluded}}<span class="badge off">排除</span>{{else}}<span class="badge on">包含</span>{{end}}</td>
        <td class="wrap">{{dash .Source}}</td>
        <td class="wrap">{{dash .Comment}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>
  </div>
</div>

<div class="card">
  <h2>表单字段顺序（formFields）</h2>
  {{if .FormFields}}<ol class="flow">{{range .FormFields}}<li><code>{{.}}</code></li>{{end}}</ol>{{else}}<div class="empty">（空）</div>{{end}}
</div>

<div class="card">
  <h2>列表列（columnFields）</h2>
  {{if .Columns}}<ol class="flow">{{range .Columns}}<li><code>{{.}}</code></li>{{end}}</ol>{{else}}<div class="empty">（空）</div>{{end}}
</div>

<div class="foot">只读预览：由 crud:preview 生成，不连数据库、不生成代码、不含写操作。</div>
</body>
</html>
`))

// buildCrudPreview 把已校验的 spec 转成预览数据。表单/列表归属按解析后的
// 字段清单判定，因此 spec 省略 formFields/columnFields 时也能看出默认结果。
func buildCrudPreview(options *helper.GenerateOptions) crudPreviewData {
	table := options.Table
	menuTitle := strings.TrimSpace(table.Comment)
	if options.Menu != nil {
		if title := strings.TrimSpace(options.Menu.Title); title != "" {
			menuTitle = title
		}
	}
	if menuTitle == "" {
		menuTitle = table.Name
	}
	mode := data_scope.ModeAuto
	if table.DataScope != nil && table.DataScope.Mode != "" {
		mode = table.DataScope.Mode
	}
	formFields := make([]string, len(table.FormFields))
	copy(formFields, table.FormFields)
	columns := make([]string, len(table.ColumnFields))
	copy(columns, table.ColumnFields)
	formSet := previewStringSet(formFields)
	columnSet := previewStringSet(columns)
	fields := make([]crudPreviewField, 0, len(options.Fields))
	for _, field := range options.Fields {
		_, inForm := formSet[field.Name]
		_, inColumns := columnSet[field.Name]
		fields = append(fields, crudPreviewField{
			Name:           field.Name,
			Title:          previewFieldTitle(field),
			SQLType:        previewSQLType(field),
			DesignType:     field.DesignType,
			Nullable:       field.Null,
			Default:        field.Default,
			Comment:        field.Comment,
			PrimaryKey:     field.PrimaryKey,
			FormExcluded:   !inForm,
			ColumnExcluded: !inColumns,
			Source:         previewFieldSource(field),
		})
	}
	return crudPreviewData{
		Table:      table.Name,
		Comment:    table.Comment,
		MenuTitle:  menuTitle,
		DataScope:  string(mode),
		FormFields: formFields,
		Columns:    columns,
		Fields:     fields,
	}
}

// previewFieldTitle 取界面标题：显式 title 优先，其次注释里的标签部分
// （字典注释取冒号前），最后退回字段名。
func previewFieldTitle(field crudmodel.Field) string {
	if title := strings.TrimSpace(field.Title); title != "" {
		return title
	}
	if label, _, ok := previewDict(field.Comment); ok && label != "" {
		return label
	}
	if comment := strings.TrimSpace(field.Comment); comment != "" {
		return comment
	}
	return field.Name
}

// previewSQLType 拼出 SQL 类型：enum/set 用 dataType 原文，其余按长度/精度
// 附括号，unsigned 追加在末尾。
func previewSQLType(field crudmodel.Field) string {
	dataType := strings.TrimSpace(field.DataType)
	if field.Type == "enum" || field.Type == "set" {
		if dataType != "" {
			return dataType
		}
	}
	sqlType := strings.TrimSpace(field.Type)
	if sqlType == "" {
		if dataType != "" {
			return dataType
		}
		return "-"
	}
	rendered := sqlType
	switch {
	case field.Precision > 0:
		rendered = fmt.Sprintf("%s(%d,%d)", sqlType, field.Length, field.Precision)
	case field.Length > 0:
		rendered = fmt.Sprintf("%s(%d)", sqlType, field.Length)
	}
	if field.Unsigned {
		rendered += " unsigned"
	}
	return rendered
}

// previewFieldSource 汇总组件的取值来源：远程下拉的关联表字段、enum/set 的
// 枚举定义、以及注释里的字典项（与生成器 getDictData 的识别口径一致）。
func previewFieldSource(field crudmodel.Field) string {
	var parts []string
	if field.DesignType == "remoteSelect" || field.DesignType == "remoteSelects" {
		target := field.Form.RemoteTable
		if target != "" && field.Form.RemoteField != "" {
			target += "." + field.Form.RemoteField
		}
		if target == "" {
			target = field.Form.RemoteUrl
		}
		if target != "" {
			parts = append(parts, "关联 "+target)
		}
	}
	if field.Type == "enum" || field.Type == "set" {
		if dataType := strings.TrimSpace(field.DataType); dataType != "" {
			parts = append(parts, "枚举 "+dataType)
		}
	}
	if _, entries, ok := previewDict(field.Comment); ok {
		parts = append(parts, "注释字典 "+strings.Join(entries, ","))
	}
	return strings.Join(parts, "；")
}

// previewDict 识别注释字典 "标签:0=禁用,1=启用"。全角冒号/逗号先归一化，
// 只有同时出现 : , = 且至少解析出一项时才认定为字典。
func previewDict(comment string) (string, []string, bool) {
	if strings.TrimSpace(comment) == "" {
		return "", nil, false
	}
	normalized := strings.ReplaceAll(strings.ReplaceAll(comment, "，", ","), "：", ":")
	label, values, found := strings.Cut(normalized, ":")
	if !found || !strings.Contains(values, ",") || !strings.Contains(values, "=") {
		return "", nil, false
	}
	entries := make([]string, 0, 4)
	for _, item := range strings.Split(values, ",") {
		key, value, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		entries = append(entries, strings.TrimSpace(key)+"="+strings.TrimSpace(value))
	}
	if len(entries) == 0 {
		return "", nil, false
	}
	return strings.TrimSpace(label), entries, true
}

// previewStringSet 把字段名清单转成集合，用于判定字段是否被表单/列表排除。
func previewStringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

// writeCrudPreviewHTML 渲染预览页；输出目录不存在时自动创建。
func writeCrudPreviewHTML(path string, data crudPreviewData) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create preview directory: %w", err)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create preview page: %w", err)
	}
	defer func() { _ = file.Close() }()
	if err := crudPreviewTemplate.Execute(file, data); err != nil {
		return fmt.Errorf("render preview page: %w", err)
	}
	return nil
}
