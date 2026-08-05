package admin_test

// admin 渠道命名机械执法（与 internal/api/naming_test.go 同一风格）。
// 生成器只兜底五类产物（entity/repository/dto/handler/registrar），
// admin 侧 service 与部分历史 handler/dto 是手写代码，同样需要执法。
//
// 规则：文件名 = 模块名（小写），以 Service/Handler/Param 结尾的类型名必须
// = <模块 PascalCase><后缀>。允许"方法拆分文件"（无类型声明的文件）。
//
// legacyAdminNames 豁免 PHP 上游继承的历史短名（v3.0.0 迁移前的既有命名，
// 改名会连锁破坏 wire/provider 引用）。新代码一律红牌，不得模仿 legacy。

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type adminNamingRule struct {
	dir      string
	suffixes []string // 该包类型允许的后缀
}

var adminNamingRules = []adminNamingRule{
	{dir: "service", suffixes: []string{"Service"}},
	{dir: "handler", suffixes: []string{"Handler"}},
	{dir: "dto", suffixes: []string{"Param"}},
}

// legacyAdminNames 键 = "<dir>/<file>:<TypeName>"，值 = 历史原因。
var legacyAdminNames = map[string]string{
	"service/routine_config.go:ConfigService":          "PHP 上游控制器名 Config（表 routine_config）",
	"service/security_sensitive_data.go:SensitiveDataService": "PHP 上游控制器名 SensitiveData",
	"handler/crud_log.go:LogHandler":                   "PHP 上游控制器名 Log（表 crud_log）",
	"handler/routine_admin_info.go:AdminInfoHandler":   "PHP 上游控制器名 AdminInfo",
	"handler/routine_attachment.go:AttachmentHandler":  "PHP 上游控制器名 Attachment",
	"handler/routine_config.go:ConfigHandler":          "PHP 上游控制器名 Config",
	"handler/security_data_recycle.go:DataRecycleHandler":         "PHP 上游控制器名 DataRecycle",
	"handler/security_data_recycle_log.go:DataRecycleLogHandler":  "PHP 上游控制器名 DataRecycleLog",
	"handler/security_sensitive_data.go:SensitiveDataHandler":     "PHP 上游控制器名 SensitiveData",
	"handler/security_sensitive_data_log.go:SensitiveDataLogHandler": "PHP 上游控制器名 SensitiveDataLog",
	"handler/user_money_log.go:MoneyLogHandler":        "PHP 上游控制器名 MoneyLog",
	"dto/routine_mail.go:MailParam":                    "PHP 上游控制器名 Mail",
}

type adminNamingViolation struct {
	File     string
	Line     int
	TypeName string
	Message  string
}

func collectAdminNamingViolations(root string) ([]adminNamingViolation, error) {
	var violations []adminNamingViolation
	for _, rule := range adminNamingRules {
		dir := filepath.Join(root, rule.dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "provider.go" {
				continue
			}
			file := filepath.Join(dir, name)
			v, err := inspectAdminNamingFile(file, name, rule)
			if err != nil {
				return nil, err
			}
			violations = append(violations, v...)
		}
	}
	return violations, nil
}

func inspectAdminNamingFile(file, name string, rule adminNamingRule) ([]adminNamingViolation, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	fileNameBase := strings.TrimSuffix(name, ".go")
	var violations []adminNamingViolation
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !typeSpec.Name.IsExported() {
				continue
			}
			typeName := typeSpec.Name.Name
			suffix := ""
			for _, s := range rule.suffixes {
				if strings.HasSuffix(typeName, s) {
					suffix = s
					break
				}
			}
			if suffix == "" {
				continue // 辅助类型（实体/参数载体）不检查
			}
			module := strings.TrimSuffix(typeName, suffix)
			if module == "" {
				continue
			}
			wantFile := toSnakeModule(module)
			if fileNameBase == wantFile {
				continue
			}
			key := rule.dir + "/" + name + ":" + typeName
			if _, legacy := legacyAdminNames[key]; legacy {
				continue
			}
			line := fset.Position(typeSpec.Pos()).Line
			violations = append(violations, adminNamingViolation{
				File: file, Line: line, TypeName: typeName,
				Message: fmt.Sprintf("type %s must live in %s.go (filename = module; e.g. user.go holds UserHandler); legacy PHP names require explicit allowlist entry", typeName, wantFile),
			})
		}
	}
	return violations, nil
}

// toSnakeModule 把 PascalCase 模块名转成 snake_case：User → user、UserOrder → user_order。
func toSnakeModule(name string) string {
	var b strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestAdminNamingRules(t *testing.T) {
	violations, err := collectAdminNamingViolations(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		return
	}
	for _, v := range violations {
		t.Errorf("%s:%d: %s: %s", v.File, v.Line, v.TypeName, v.Message)
	}
}

func TestAdminNamingDetectsViolations(t *testing.T) {
	root := t.TempDir()
	samples := []struct {
		file   string
		source string
	}{
		{"service/user.go", "package service\n\ntype UserService struct{}\n"},
		{"service/member.go", "package service\n\ntype UserService struct{}\n"},
		{"handler/crud_log.go", "package handler\n\ntype LogHandler struct{}\n"},
		{"handler/admin.go", "package handler\n\ntype AdminHandler struct{}\n"},
		{"dto/order.go", "package dto\n\ntype OrderParam struct{}\n"},
		{"dto/order.go", "package dto\n\ntype MailParam struct{}\n"}, // legacy 豁免（dto/routine_mail.go 才有豁免键，这里不应豁免）
	}
	for _, sample := range samples {
		dir := filepath.Join(root, filepath.Dir(sample.file))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, sample.file), []byte(sample.source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	violations, err := collectAdminNamingViolations(root)
	if err != nil {
		t.Fatal(err)
	}
	// 期望违规：service/member.go 的 UserService（文件名不符）、
	// handler/crud_log.go 的 LogHandler（该键在 legacy 里是
	// handler/crud_log.go:LogHandler，文件名相同会命中 legacy，豁免！
	// 所以此处应为 0 违规——见下方说明）。
	// 实际：samples 里 handler/crud_log.go:LogHandler 命中 legacy 豁免；
	// dto/order.go:MailParam 不在 legacy（键是 dto/routine_mail.go:MailParam），
	// 文件名 order ≠ mail → 违规。
	want := 2 // service/member.go UserService + dto/order.go MailParam
	if len(violations) != want {
		t.Fatalf("violations = %d, want %d: %+v", len(violations), want, violations)
	}
}
