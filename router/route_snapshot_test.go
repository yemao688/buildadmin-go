package router

import (
	"flag"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// routeGoldenPath 是注册路由表黄金快照的位置。testdata 是 Go 约定的测试夹具目录，
// 该文件只被本包的快照测试读取，业务运行时代码从不使用它。
const routeGoldenPath = "testdata/registered_routes.golden"

// routeGoldenHeader 写在快照文件开头，说明用途与生成方式；读取时以 # 开头的行会被跳过。
const routeGoldenHeader = `# 注册路由表黄金快照 —— 仅供 router 包快照测试比对，业务运行时代码从不读取本文件。
# Golden snapshot of the registered route table; only the router snapshot test reads it.
# 由测试生成，禁止手改。路由有意变更后重新生成：
#   go test ./router/ -run TestRouteSnapshotMatchesGolden -update
`

var updateRoutesGolden = flag.Bool("update", false, "regenerate "+routeGoldenPath)

func TestRouteSnapshotMatchesGolden(t *testing.T) {
	engine := newCompleteRouter()
	actual, duplicates := sortedUniqueRouteKeys(engine.Routes())
	if len(duplicates) > 0 {
		t.Fatalf("duplicate routes registered:\n%s", strings.Join(duplicates, "\n"))
	}

	if *updateRoutesGolden {
		writeRouteGolden(t, actual)
	}
	expected := readRouteGolden(t)
	if slices.Equal(expected, actual) {
		return
	}

	missing, extra := routeSnapshotDiff(expected, actual)
	t.Fatalf(
		"route snapshot mismatch\nmissing routes:\n%s\nextra routes:\n%s",
		formatRouteDiff(missing),
		formatRouteDiff(extra),
	)
}

func sortedUniqueRouteKeys(routes []gin.RouteInfo) ([]string, []string) {
	all := make([]string, 0, len(routes))
	for _, route := range routes {
		all = append(all, routeKey(route.Method, route.Path))
	}
	sort.Strings(all)

	unique := make([]string, 0, len(all))
	duplicates := make([]string, 0)
	for _, route := range all {
		if len(unique) > 0 && unique[len(unique)-1] == route {
			if len(duplicates) == 0 || duplicates[len(duplicates)-1] != route {
				duplicates = append(duplicates, route)
			}
			continue
		}
		unique = append(unique, route)
	}
	return unique, duplicates
}

func readRouteGolden(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(routeGoldenPath)
	if err != nil {
		t.Fatalf("read route golden: %v", err)
	}
	var routes []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		routes = append(routes, line)
	}
	return routes
}

func writeRouteGolden(t *testing.T, routes []string) {
	t.Helper()
	data := routeGoldenHeader + strings.Join(routes, "\n") + "\n"
	if err := os.WriteFile(routeGoldenPath, []byte(data), 0o644); err != nil {
		t.Fatalf("write route golden: %v", err)
	}
}

func routeSnapshotDiff(expected, actual []string) (missing, extra []string) {
	actualSet := make(map[string]struct{}, len(actual))
	for _, route := range actual {
		actualSet[route] = struct{}{}
	}
	for _, route := range expected {
		if _, ok := actualSet[route]; !ok {
			missing = append(missing, route)
		}
	}

	expectedSet := make(map[string]struct{}, len(expected))
	for _, route := range expected {
		expectedSet[route] = struct{}{}
	}
	for _, route := range actual {
		if _, ok := expectedSet[route]; !ok {
			extra = append(extra, route)
		}
	}
	return missing, extra
}

func formatRouteDiff(routes []string) string {
	if len(routes) == 0 {
		return "(none)"
	}
	return strings.Join(routes, "\n")
}
