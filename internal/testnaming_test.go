package app_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 保证所有直接调用 MySQL 门禁
// （OpenMySQL/OpenFixtureDatabase）的测试文件都以 _mysql_test.go 结尾：后缀是
// "需要 mysql_test 配置"的唯一可见标记，缺失时 MySQL 用例会混在纯逻辑测试里，
// 无法按文件名审阅与筛选。
func TestMySQLTestFileSuffix(t *testing.T) {
	t.Parallel()
	violations, err := collectMySQLTestFileViolations(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		return
	}
	sort.Strings(violations)
	var b strings.Builder
	b.WriteString("以下测试文件调用了 OpenMySQL/OpenFixtureDatabase，但未以 _mysql_test.go 结尾：\n")
	for _, file := range violations {
		b.WriteString("  " + file + "\n")
	}
	t.Fatal(b.String())
}

func collectMySQLTestFileViolations(root string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_mysql_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		// testutil 自身实现门禁，必然直接调用这两个构造函数。
		if strings.HasPrefix(rel, "pkg/testutil/") {
			return nil
		}
		uses, err := callsMySQLTestUtil(path)
		if err != nil {
			return err
		}
		if uses {
			violations = append(violations, "internal/"+rel)
		}
		return nil
	})
	return violations, err
}

// callsMySQLTestUtil 用 AST 匹配 testutil 包内的 OpenMySQL/OpenFixtureDatabase
// 调用，避免把仅有 import 或注释提及该包名的文件误判为 MySQL 依赖。
func callsMySQLTestUtil(path string) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return false, err
	}
	uses := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != "testutil" {
			return true
		}
		if selector.Sel.Name == "OpenMySQL" || selector.Sel.Name == "OpenFixtureDatabase" {
			uses = true
		}
		return true
	})
	return uses, nil
}
