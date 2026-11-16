package commands

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

// planHTMLMeta 是审批页头部的事实：谁、在哪、什么时候、风险多大。
type planHTMLMeta struct {
	SpecRef   string
	Database  string
	Generated string
	Tables    int
	Blocked   int
	Rejected  int
	Risk      string
}

type planHTMLPage struct {
	Meta    planHTMLMeta
	Plan    planJSON
	Approve string
}

// planHTMLTemplate 是自包含的单文件页面：不引用外部资源，样式内联，页面
// **只读**——批准动作产出的是可复制的命令文本，执行仍留在终端与 git 里。
var planHTMLTemplate = template.Must(template.New("plan").Funcs(template.FuncMap{
	"badgeTone": func(class string) string {
		switch class {
		case "safe-auto":
			return "ok"
		case "requires-approval":
			return "warn"
		case "rejected":
			return "bad"
		default:
			return "muted"
		}
	},
}).Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CRUD 审批 · {{.Meta.Tables}} 张表</title>
<style>
  :root { color-scheme: light dark; --fg:#1c1c1e; --muted:#6b7280; --line:#e5e7eb; --bg:#ffffff; --panel:#f9fafb; --ok:#047857; --okbg:#ecfdf5; --warn:#b45309; --warnbg:#fffbeb; --bad:#b91c1c; --badbg:#fef2f2; }
  @media (prefers-color-scheme: dark) { :root { --fg:#e5e7eb; --muted:#9ca3af; --line:#374151; --bg:#0f1115; --panel:#161922; --ok:#34d399; --okbg:#0b2a20; --warn:#fbbf24; --warnbg:#2a2008; --bad:#f87171; --badbg:#2a0f0f; } }
  * { box-sizing: border-box; }
  body { margin:0; padding:24px; background:var(--bg); color:var(--fg); font:14px/1.55 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif; }
  h1 { font-size:20px; margin:0 0 4px; }
  .sub { color:var(--muted); margin-bottom:20px; }
  .meta { display:flex; flex-wrap:wrap; gap:12px 28px; padding:14px 16px; background:var(--panel); border:1px solid var(--line); border-radius:10px; margin-bottom:20px; }
  .meta div span { display:block; color:var(--muted); font-size:12px; }
  .risk { font-weight:600; }
  .risk.ok { color:var(--ok); } .risk.warn { color:var(--warn); } .risk.bad { color:var(--bad); }
  .card { border:1px solid var(--line); border-radius:10px; margin-bottom:18px; overflow:hidden; }
  .card h2 { font-size:15px; margin:0; padding:12px 16px; background:var(--panel); border-bottom:1px solid var(--line); display:flex; gap:10px; align-items:center; flex-wrap:wrap; }
  table { width:100%; border-collapse:collapse; }
  th, td { text-align:left; padding:9px 12px; border-bottom:1px solid var(--line); vertical-align:top; }
  th { font-size:12px; color:var(--muted); font-weight:500; }
  tr:last-child td { border-bottom:none; }
  code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; font-size:12px; background:var(--panel); padding:1px 5px; border-radius:4px; word-break:break-all; }
  .badge { display:inline-block; padding:1px 8px; border-radius:999px; font-size:12px; font-weight:500; white-space:nowrap; }
  .badge.ok { color:var(--ok); background:var(--okbg); } .badge.warn { color:var(--warn); background:var(--warnbg); } .badge.bad { color:var(--bad); background:var(--badbg); } .badge.muted { color:var(--muted); background:var(--panel); }
  .reason { color:var(--muted); font-size:13px; }
  .approve { border:1px solid var(--line); border-radius:10px; padding:14px 16px; background:var(--panel); }
  .approve pre { margin:8px 0 0; white-space:pre-wrap; word-break:break-all; }
  .foot { color:var(--muted); font-size:12px; margin-top:18px; }
</style>
</head>
<body>
<h1>CRUD 变更审批</h1>
<div class="sub">只读页面：批准动作不会在这里执行，复制命令回终端确认。</div>
<div class="meta">
  <div><span>spec</span>{{if .Meta.SpecRef}}{{.Meta.SpecRef}}{{else}}-{{end}}</div>
  <div><span>目标库</span>{{if .Meta.Database}}{{.Meta.Database}}{{else}}-{{end}}</div>
  <div><span>生成时间</span>{{.Meta.Generated}}</div>
  <div><span>表</span>{{.Meta.Tables}}</div>
  <div><span>待批准</span>{{.Meta.Blocked}}</div>
  <div><span>rejected</span>{{.Meta.Rejected}}</div>
  <div><span>结论</span><span class="risk {{.Meta.Risk}}">{{if eq .Meta.Risk "bad"}}需先写业务迁移{{else if eq .Meta.Risk "warn"}}需要人工批准{{else}}可自动执行{{end}}</span></div>
</div>

{{range .Plan.Tables}}
<div class="card">
  <h2>{{.Table}}
    <span class="badge muted">{{.Action}}</span>
    {{if .Destructive}}<span class="badge bad">删除重建</span>{{end}}
    {{if .Blocked}}<span class="badge warn">{{.Blocked}} 项阻断</span>{{end}}
  </h2>
  {{if .Changes}}
  <table>
    <thead><tr><th>字段</th><th>分类</th><th>变更</th><th>说明</th></tr></thead>
    <tbody>
    {{range .Changes}}
      <tr>
        <td><code>{{.Field}}</code></td>
        <td><span class="badge {{badgeTone .Class}}">{{.Class}}{{if .Category}} · {{.Category}}{{end}}{{if .Approved}} · 已批准{{end}}</span></td>
        <td>{{if .DDL}}<code>{{.DDL}}</code>{{else}}-{{end}}</td>
        <td class="reason">{{if .Reason}}{{.Reason}}{{else}}-{{end}}{{if .Unmanaged}}<br>unmanaged: {{range $index, $item := .Unmanaged}}{{if $index}}, {{end}}{{$item}}{{end}}{{end}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>
  {{else}}<div style="padding:12px 16px" class="reason">无差量</div>{{end}}
</div>
{{end}}

<div class="approve">
  <strong>批准命令</strong>
  {{if .Approve}}<pre><code>{{.Approve}}</code></pre>{{else}}<span class="reason">没有待批准的 requires-approval 变更。rejected 变更只能通过业务迁移处理。</span>{{end}}
</div>
<div class="foot">本页面由 crud:apply --plan --html 生成，内容来自只读计划；页面不含任何写操作。</div>
</body>
</html>
`))

// planHTMLMetaFrom 汇总头部事实。rejected 永远阻断，requires-approval 需批准。
func planHTMLMetaFrom(payload planJSON, specRef, database string) planHTMLMeta {
	meta := planHTMLMeta{
		SpecRef:   specRef,
		Database:  database,
		Generated: time.Now().Format("2006-01-02 15:04:05"),
		Tables:    len(payload.Tables),
		Risk:      "ok",
	}
	for _, table := range payload.Tables {
		meta.Blocked += table.Blocked
		for _, change := range table.Changes {
			switch change.Class {
			case "rejected":
				meta.Rejected++
			case "requires-approval":
				if !change.Approved {
					meta.Blocked++
				}
			}
		}
	}
	if meta.Rejected > 0 {
		meta.Risk = "bad"
	} else if meta.Blocked > 0 {
		meta.Risk = "warn"
	}
	return meta
}

// writePlanHTML 渲染审批页；目录不存在时自动创建。
func writePlanHTML(path string, payload planJSON, specRef, database string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create plan directory: %w", err)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create plan page: %w", err)
	}
	defer func() { _ = file.Close() }()
	page := planHTMLPage{
		Meta:    planHTMLMetaFrom(payload, specRef, database),
		Plan:    payload,
		Approve: payload.ApproveCommand,
	}
	if err := planHTMLTemplate.Execute(file, page); err != nil {
		return fmt.Errorf("render plan page: %w", err)
	}
	return nil
}
