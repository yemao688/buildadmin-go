package api_test

// api 渠道命名机械执法（与 internal/boundary_test.go 的 R1-R7 同一风格）：
// api 侧代码全部手写（无 CRUD 生成器兜底），文件名与类型名必须遵守
// "文件名 = 模块名（小写），类型名 = <模块 PascalCase><后缀>" 规范：
//
//   - repository/user.go  → UserRepository
//   - service/member.go   → MemberService
//   - handler/user.go     → UserHandler
//   - router/user.go      → UserRegistrar
//
// 允许"方法拆分文件"：ajax.go / alioss.go 这类只包含某个 Handler 方法、
// 不含任何类型声明的文件不要求文件名对齐。
//
// 禁止非规范后缀变体：Repo / Dao / DAO / Svc / Mgr / Impl / Controller 等。
// 违规会被本测试红牌，并给出建议命名。

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type apiNamingRule struct {
	dir      string   // internal/api 下相对目录
	suffix   string   // 该包类型必须使用的精确后缀（suffix 模式）
	prefix   bool     // dto 投影模式：类型名必须以文件模块名 PascalCase 开头（如 user.go 的 OutUser）
	banned   []string // 禁止出现在类型名中的非规范词根
	skipFile []string // 基础设施文件（无模块类型，豁免文件名对齐）
}

var apiNamingRules = []apiNamingRule{
	{dir: "repository", suffix: "Repository", banned: []string{"Repo", "Dao", "DAO", "RepositoryImpl", "Svc"}},
	{dir: "service", suffix: "Service", banned: []string{"Svc", "ServiceImpl", "Mgr", "Manager", "Repo"}},
	{dir: "handler", suffix: "Handler", banned: []string{"Controller", "HandlerImpl", "Api"}},
	{dir: "router", suffix: "Registrar", banned: []string{"Route", "Router"}, skipFile: []string{"registrar.go", "router.go", "api_routes.go"}},
	// dto 是投影层：类型名以文件模块名 PascalCase 开头（user.go → User，如 OutUser），
	// 允许 Out/Resp 等前缀修饰；禁止 Repo/Dao 等仓库词根混入投影。
	{dir: "dto", prefix: true, banned: []string{"Repo", "Dao", "DAO", "Svc", "Mgr", "Impl"}},
}

type apiNamingViolation struct {
	File     string
	Line     int
	TypeName string
	Message  string
}

// collectAPINamingViolations 解析 root 下各规则目录，收集命名违规。
// provider.go 与 *_test.go 豁免；无类型声明的文件（方法拆分文件）豁免。
func collectAPINamingViolations(root string) ([]apiNamingViolation, error) {
	var violations []apiNamingViolation
	for _, rule := range apiNamingRules {
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
			if slices.Contains(rule.skipFile, name) {
				continue
			}
			base := strings.TrimSuffix(name, ".go")
			file := filepath.Join(dir, name)
			v, err := inspectAPINamingFile(file, base, rule)
			if err != nil {
				return nil, err
			}
			violations = append(violations, v...)
		}
	}
	return violations, nil
}

func inspectAPINamingFile(file, base string, rule apiNamingRule) ([]apiNamingViolation, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	var violations []apiNamingViolation
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
			line := fset.Position(typeSpec.Pos()).Line
			if v, ok := checkAPITypeName(typeSpec.Name.Name, base, rule, line, file); ok {
				violations = append(violations, v)
			}
		}
	}
	return violations, nil
}

// checkAPITypeName 对单个导出类型做命名检查，返回 (违规, 是否违规)。
func checkAPITypeName(typeName, fileNameBase string, rule apiNamingRule, line int, file string) (apiNamingViolation, bool) {
	if rule.prefix {
		// dto 投影模式：类型名必须以文件模块名 PascalCase 开头或结尾
		// （user.go → User；OutUser 以 User 结尾、UserResp 以 User 开头均合法）。
		module := toPascalModule(fileNameBase)
		if module == "" {
			return apiNamingViolation{}, false
		}
		if !strings.HasPrefix(typeName, module) && !strings.HasSuffix(typeName, module) {
			return apiNamingViolation{File: file, Line: line, TypeName: typeName,
				Message: fmt.Sprintf("type %s must start or end with module %s (filename = module; e.g. user.go holds OutUser projection)", typeName, module)}, true
		}
		for _, banned := range rule.banned {
			if strings.Contains(typeName, banned) {
				return apiNamingViolation{File: file, Line: line, TypeName: typeName,
					Message: fmt.Sprintf("non-canonical word %q in dto projection; want <module>Out... (e.g. user.go holds OutUser)", banned)}, true
			}
		}
		return apiNamingViolation{}, false
	}
	if strings.HasSuffix(typeName, rule.suffix) {
		module := strings.TrimSuffix(typeName, rule.suffix)
		if module == "" {
			return apiNamingViolation{File: file, Line: line, TypeName: typeName,
				Message: "type name is only the suffix"}, true
		}
		// 文件名必须 = 模块名 snake_case（不带 .go 比较）。
		wantFile := toSnakeModule(module)
		if fileNameBase != wantFile {
			return apiNamingViolation{File: file, Line: line, TypeName: typeName,
				Message: fmt.Sprintf("type %s must live in %s.go (filename = module; e.g. user.go holds UserRepository)", typeName, wantFile)}, true
		}
		return apiNamingViolation{}, false
	}
	for _, banned := range rule.banned {
		if strings.Contains(typeName, banned) {
			return apiNamingViolation{File: file, Line: line, TypeName: typeName,
				Message: fmt.Sprintf("non-canonical suffix %q; want <module>%s (e.g. user.go holds UserRepository)", banned, rule.suffix)}, true
		}
	}
	// 无后缀/无关后缀的辅助类型（如 handler 请求 DTO）不检查。
	return apiNamingViolation{}, false
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

// toPascalModule 把 snake_case 文件名转成 PascalCase 模块名：user → User、
// user_login → UserLogin（dto 投影前缀）。
func toPascalModule(name string) string {
	var b strings.Builder
	upper := true
	for _, r := range name {
		if r == '_' {
			upper = true
			continue
		}
		if upper {
			if r >= 'a' && r <= 'z' {
				r -= 'a' - 'A'
			}
			upper = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestAPINamingRules(t *testing.T) {
	violations, err := collectAPINamingViolations(".")
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

func TestAPINamingDetectsViolations(t *testing.T) {
	root := t.TempDir()
	for _, rule := range apiNamingRules {
		if err := os.MkdirAll(filepath.Join(root, rule.dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	samples := []struct {
		file     string
		source   string
		wantLine int
		wantMsg  string
	}{
		{
			file:   "repository/user.go",
			source: "package repository\n\ntype UserRepo struct{}\n",
			wantMsg: "non-canonical suffix",
		},
		{
			file:   "repository/member.go",
			source: "package repository\n\ntype UserRepository struct{}\n",
			wantMsg: "must live in user.go",
		},
		{
			file:   "service/member.go",
			source: "package service\n\ntype MemberSvc struct{}\n",
			wantMsg: "non-canonical suffix",
		},
		{
			file:   "handler/auth.go",
			source: "package handler\n\ntype UserController struct{}\n",
			wantMsg: "non-canonical suffix",
		},
		{
			file:   "router/user.go",
			source: "package router\n\ntype UserRoute struct{}\n",
			wantMsg: "non-canonical suffix",
		},
	}
	for _, sample := range samples {
		path := filepath.Join(root, sample.file)
		if err := os.WriteFile(path, []byte(sample.source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	violations, err := collectAPINamingViolations(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != len(samples) {
		t.Fatalf("violations = %d, want %d: %+v", len(violations), len(samples), violations)
	}
	for _, v := range violations {
		if !strings.Contains(v.Message, "non-canonical") && !strings.Contains(v.Message, "must live in") {
			t.Errorf("unexpected message: %s", v.Message)
		}
	}
}

func TestToSnakeModule(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"User", "user"},
		{"Member", "member"},
		{"UserOrder", "user_order"},
		{"Index", "index"},
	} {
		if got := toSnakeModule(tc.in); got != tc.want {
			t.Errorf("toSnakeModule(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
