package crud_helper

import (
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/utils"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRegistrarTemplateRendersAndFormats(t *testing.T) {
	path := filepath.Join(utils.RootPath(), "internal", "admin", "handler", "demo_route.go")
	content, err := render(path, registrarTemp, RegistrarData{
		Namespace: "handler",
		ClassName: "Demo",
		RouteName: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `CRUDRoutes(g, demoRoute, r.handler)`) {
		t.Fatalf("registrar route call missing:\n%s", content)
	}
	if !strings.Contains(content, `CRUDCapabilities(demoRoute)`) {
		t.Fatalf("registrar capability call missing:\n%s", content)
	}
}

func TestRegistrarTemplateMatchesCountryLanguageShape(t *testing.T) {
	path := filepath.Join(utils.RootPath(), "internal", "admin", "router", "country_language.go")
	content, err := render(path, registrarTemp, RegistrarData{
		Namespace:            "router",
		ClassName:            "Language",
		RouteName:            "language",
		RoutePath:            "country.Language",
		BaseHandlerQualifier: "handler.",
		BaseHandlerAlias:     "handler",
		BaseHandlerImport:    "buildadmin-go/internal/admin/handler",
	})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if content != string(expected) {
		t.Fatalf("registrar template differs from country sample:\n--- got ---\n%s\n--- want ---\n%s", content, expected)
	}
}

func TestRegistrarProviderEntryRoundTrip(t *testing.T) {
	path := filepath.Join(utils.RootPath(), "internal", "router", "registrar_set.go")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.WriteFile(path, original, 0644) })

	if err := writeRegistrarProviderEntry("Test", "internal/admin/handler"); err != nil {
		t.Fatal(err)
	}
	added, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(added), "testRegistrar *admin.TestRegistrar,") || !strings.Contains(string(added), "testRegistrar,") {
		t.Fatalf("registrar provider entry missing:\n%s", added)
	}
	if err := writeRegistrarProviderEntry("Test", "internal/admin/handler"); err != nil {
		t.Fatal(err)
	}
	addedAgain, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(addedAgain) != string(added) {
		t.Fatal("registrar provider insertion is not idempotent")
	}

	if err := RemoveRegistrarProvider("Test", "internal/admin/handler"); err != nil {
		t.Fatal(err)
	}
	removed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(removed) != string(original) {
		t.Fatalf("registrar provider round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", removed, original)
	}
	if err := RemoveRegistrarProvider("Test", "internal/admin/handler"); err != nil {
		t.Fatal(err)
	}
	removedAgain, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(removedAgain) != string(original) {
		t.Fatal("removing an absent registrar provider entry is not idempotent")
	}
}

func TestProviderEntryRoundTrip(t *testing.T) {
	original := "package model\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewFooModel,\n\n\tNewBarModel,\n)\n"
	lastIndex := strings.LastIndex(original, ")")
	added := original[:lastIndex] + "\tNewTestModel,\n)"
	if strings.Contains(added, ",\n\n\n\tNewTestModel") {
		t.Fatalf("insert produced a double blank line:\n%s", added)
	}
	removed, err := removeProviderEntry(added, "TestModel")
	if err != nil {
		t.Fatal(err)
	}
	if removed != original {
		t.Fatalf("provider round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", removed, original)
	}
}

func TestRemoveProviderEntryShapesRemainParseable(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "unique", body: "\tNewTargetModel,\n"},
		{name: "first", body: "\tNewTargetModel,\n\tNewKeepModel,\n"},
		{name: "middle", body: "\tNewKeepModel,\n\tNewTargetModel,\n\tNewOtherModel,\n"},
		{name: "last", body: "\tNewKeepModel,\n\tNewTargetModel,\n"},
		{name: "single line unique", body: "NewTargetModel"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "package provider\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n" + tc.body + ")\n"
			removed, err := removeProviderEntry(content, "TargetModel")
			if err != nil {
				t.Fatal(err)
			}
			assertParseableGo(t, "provider.go", removed)
			if strings.Contains(removed, "NewTargetModel") {
				t.Fatalf("target provider remains:\n%s", removed)
			}
		})
	}
}

func TestRemoveRegistrarProviderEntryShapesRemainParseable(t *testing.T) {
	cases := []struct {
		name   string
		params string
		items  string
	}{
		{name: "unique", params: "targetRegistrar *admin.TargetRegistrar", items: "targetRegistrar"},
		{name: "first", params: "targetRegistrar *admin.TargetRegistrar,\n\tkeepRegistrar *admin.KeepRegistrar", items: "targetRegistrar,\n\tkeepRegistrar"},
		{name: "middle", params: "keepRegistrar *admin.KeepRegistrar,\n\ttargetRegistrar *admin.TargetRegistrar,\n\totherRegistrar *admin.OtherRegistrar", items: "keepRegistrar,\n\ttargetRegistrar,\n\totherRegistrar"},
		{name: "last", params: "keepRegistrar *admin.KeepRegistrar,\n\ttargetRegistrar *admin.TargetRegistrar", items: "keepRegistrar,\n\ttargetRegistrar"},
		{name: "legacy bare var", params: "keepRegistrar *admin.KeepRegistrar,\n\ttarget *admin.TargetRegistrar", items: "keepRegistrar,\n\ttarget"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "package router\n\nimport admin \"buildadmin-go/internal/admin/handler\"\n\ntype RouteRegistrar interface{}\n\nfunc ProvideRegistrars(" + tc.params + ") []RouteRegistrar {\n\treturn []RouteRegistrar{" + tc.items + "}\n}\n"
			removed, err := removeRegistrarProviderEntry(content, "Target", "internal/admin/handler")
			if err != nil {
				t.Fatal(err)
			}
			assertParseableGo(t, "registrar_set.go", removed)
			if strings.Contains(removed, "TargetRegistrar") {
				t.Fatalf("target registrar remains:\n%s", removed)
			}
			if strings.Contains(removed, "targetRegistrar") || strings.Contains(removed, "\ttarget,") || strings.Contains(removed, "target *") {
				t.Fatalf("target registrar var remains:\n%s", removed)
			}
		})
	}
}

func TestRemoveRegistrarProviderEntrySubpackageKeepsRootSameName(t *testing.T) {
	content := "package router\n\nimport (\n\tadmin \"buildadmin-go/internal/admin/handler\"\n\torder \"buildadmin-go/internal/admin/handler/order\"\n)\n\ntype RouteRegistrar interface{}\n\nfunc ProvideRegistrars(\n\tuser *admin.UserRegistrar,\n\torderUserRegistrar *order.UserRegistrar,\n) []RouteRegistrar {\n\treturn []RouteRegistrar{\n\t\tuser,\n\t\torderUserRegistrar,\n\t}\n}\n"

	// 删除子包 User：根包 user 参数与条目必须保留，子包条目与 import 一并移除
	removed, err := removeRegistrarProviderEntry(content, "User", "internal/admin/handler/order")
	if err != nil {
		t.Fatal(err)
	}
	assertParseableGo(t, "registrar_set.go", removed)
	if !strings.Contains(removed, "user *admin.UserRegistrar,") || !strings.Contains(removed, "\t\tuser,\n") {
		t.Fatalf("root user registrar must be kept when deleting subpackage User:\n%s", removed)
	}
	if strings.Contains(removed, "orderUserRegistrar") || strings.Contains(removed, "handler/order") {
		t.Fatalf("subpackage registrar/import not fully removed:\n%s", removed)
	}

	// 删除根包 User：子包条目必须保留
	removedRoot, err := removeRegistrarProviderEntry(content, "User", "internal/admin/handler")
	if err != nil {
		t.Fatal(err)
	}
	assertParseableGo(t, "registrar_set.go", removedRoot)
	if !strings.Contains(removedRoot, "orderUserRegistrar *order.UserRegistrar,") || !strings.Contains(removedRoot, "\t\torderUserRegistrar,\n") {
		t.Fatalf("subpackage registrar must be kept when deleting root User:\n%s", removedRoot)
	}
	if strings.Contains(removedRoot, "user *admin.UserRegistrar") {
		t.Fatalf("root user registrar not removed:\n%s", removedRoot)
	}
}

func TestRegistrarProviderEntrySubpackageRoundTrip(t *testing.T) {
	path := filepath.Join(utils.RootPath(), "internal", "router", "registrar_set.go")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.WriteFile(path, original, 0644) })

	handlerRoot := "internal/admin/handler/registrar_subpkg_test"
	if err := writeRegistrarProviderEntry("Order", handlerRoot); err != nil {
		t.Fatal(err)
	}
	added, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertParseableGo(t, "registrar_set.go", string(added))
	for _, want := range []string{
		"registrar_subpkg_test \"buildadmin-go/internal/admin/handler/registrar_subpkg_test\"",
		"registrar_subpkg_testOrderRegistrar *registrar_subpkg_test.OrderRegistrar,",
		"\t\tregistrar_subpkg_testOrderRegistrar,",
	} {
		if !strings.Contains(string(added), want) {
			t.Fatalf("subpackage registrar entry missing %q:\n%s", want, added)
		}
	}
	if strings.Contains(string(added), "*admin.OrderRegistrar") {
		t.Fatalf("subpackage registrar must not use root admin qualifier:\n%s", added)
	}

	if err := RemoveRegistrarProvider("Order", handlerRoot); err != nil {
		t.Fatal(err)
	}
	removed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(removed) != string(original) {
		t.Fatalf("subpackage registrar round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", removed, original)
	}
}

const wireFixture = `package main

import (
	adminHandler "buildadmin-go/internal/admin/handler"
	adminModel "buildadmin-go/internal/admin/repository"

	"github.com/google/wire"
)

func wireApp() {
	panic(wire.Build(
		adminHandler.ProviderSet,
		adminModel.ProviderSet,
	))
}
`

func TestWireProviderSetRefSkipsWiredRoots(t *testing.T) {
	for _, root := range []string{"internal/admin/handler", "internal/admin/repository", "internal/admin/model", "internal/common/model", "internal/api/handler"} {
		if _, _, _, needed, err := wireProviderSetRef(root); err != nil || needed {
			t.Fatalf("wired root %q should not need aggregation: needed=%v err=%v", root, needed, err)
		}
	}
	importPath, alias, anchor, needed, err := wireProviderSetRef("internal/admin/handler/order")
	if err != nil || !needed {
		t.Fatalf("subpackage ref failed: %v", err)
	}
	if importPath != "buildadmin-go/internal/admin/handler/order" || alias != "orderHandler" || anchor != "\t\tadminHandler.ProviderSet,\n" {
		t.Fatalf("unexpected handler subpackage ref: %q %q %q", importPath, alias, anchor)
	}
	if _, alias, _, _, err := wireProviderSetRef("internal/admin/repository/order"); err != nil || alias != "orderRepo" {
		t.Fatalf("unexpected repository subpackage alias %q: %v", alias, err)
	}
}

func TestWireProviderSetEntryAddRemoveRoundTrip(t *testing.T) {
	_, alias, anchor, _, err := wireProviderSetRef("internal/admin/handler/order")
	if err != nil {
		t.Fatal(err)
	}
	added, err := addWireProviderSetEntry(wireFixture, "buildadmin-go/internal/admin/handler/order", alias, anchor)
	if err != nil {
		t.Fatal(err)
	}
	assertParseableGo(t, "wire.go", added)
	if !strings.Contains(added, "orderHandler \"buildadmin-go/internal/admin/handler/order\"") {
		t.Fatalf("wire.go import missing:\n%s", added)
	}
	if !strings.Contains(added, "\t\tadminHandler.ProviderSet,\n\t\torderHandler.ProviderSet,\n") {
		t.Fatalf("wire.go provider set not anchored after adminHandler:\n%s", added)
	}
	again, err := addWireProviderSetEntry(added, "buildadmin-go/internal/admin/handler/order", alias, anchor)
	if err != nil {
		t.Fatal(err)
	}
	if again != added {
		t.Fatal("wire provider set insertion is not idempotent")
	}
	removed := removeWireProviderSetEntry(added, "buildadmin-go/internal/admin/handler/order", alias)
	if removed != wireFixture {
		t.Fatalf("wire provider set round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", removed, wireFixture)
	}
}

func TestCountWireProviderSetEntries(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
		wantErr bool
	}{
		{
			name:    "empty",
			content: "package provider\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet()\n",
			want:    0,
		},
		{
			name:    "entries",
			content: "package provider\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewFoo,\n\tNewBar,\n)\n",
			want:    2,
		},
		{
			name:    "invalid",
			content: "package provider\nvar ProviderSet = wire.NewSet(\n",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := countWireProviderSetEntries(tc.content)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected parse error, got count %d", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("countWireProviderSetEntries() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRemoveWireProviderSetKeepsSharedProviderSetWhenEntriesRemain(t *testing.T) {
	rootDir := "internal/admin/handler/remove_wire_provider_set_test"
	dir := filepath.Join(utils.RootPath(), rootDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, "provider.go"), []byte("package provider\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewFoo,\n\tNewBar,\n)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	wirePath := filepath.Join(utils.RootPath(), "cmd", "server", "wire.go")
	original, err := os.ReadFile(wirePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.WriteFile(wirePath, original, 0644) })

	importPath, alias, anchor, needed, err := wireProviderSetRef(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	if !needed {
		t.Fatal("expected test root to require wire aggregation")
	}

	if err := AddWireProviderSet(rootDir); err != nil {
		t.Fatal(err)
	}
	added, err := os.ReadFile(wirePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{anchor, "\t\t" + alias + ".ProviderSet,\n", alias + " \"" + importPath + "\""} {
		if !strings.Contains(string(added), want) {
			t.Fatalf("wire.go missing %q after add:\n%s", want, added)
		}
	}

	if err := RemoveWireProviderSet(rootDir); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(wirePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(added) {
		t.Fatalf("wire.go changed unexpectedly while shared providers remain:\n--- got ---\n%s\n--- want ---\n%s", after, added)
	}
}

func assertParseableGo(t *testing.T, filename, content string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), filename, content, parser.AllErrors); err != nil {
		t.Fatalf("%s is not parseable: %v\n%s", filename, err, content)
	}
}

func TestWriteProviderCreatesMissingScaffold(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "internal", "admin", "model", "provider_scaffold_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := writeProvider("internal/admin/model/provider_scaffold_test", "OwnerModel"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "provider.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "ProviderSet") || !strings.Contains(string(content), "NewOwnerModel") {
		t.Fatalf("provider scaffold was not injected: %s", content)
	}
}

func TestWriteProviderSequentialEntriesOnFreshPackage(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "internal", "admin", "handler", "provider_fresh_seq_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	// 首个条目写入后 gofmt 会把单参数 NewSet 折叠成单行，第二个条目必须仍能合法追加
	if err := writeProvider("internal/admin/handler/provider_fresh_seq_test", "UserHandler"); err != nil {
		t.Fatal(err)
	}
	if err := writeProvider("internal/admin/handler/provider_fresh_seq_test", "UserRegistrar"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "provider.go"))
	if err != nil {
		t.Fatal(err)
	}
	assertParseableGo(t, "provider.go", string(content))
	if !strings.Contains(string(content), "\tNewUserHandler,\n") || !strings.Contains(string(content), "\tNewUserRegistrar,\n") {
		t.Fatalf("sequential provider entries missing: %s", content)
	}
	assertExactlyOneTrailingLF(t, string(content))
}

func TestProviderWriteRoundTripPreservesEOFConvention(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "internal", "admin", "model", "provider_eof_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	provider := filepath.Join(dir, "provider.go")
	base := "package provider_eof_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewExistingModel,\n)"
	canonical := base + "\n"
	for _, original := range []string{base, canonical, canonical + "\n\n"} {
		t.Run(strings.ReplaceAll(strconv.Quote(original), "\\n", "\\\n"), func(t *testing.T) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(provider, []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			if err := writeProvider("internal/admin/model/provider_eof_test", "OwnerModel"); err != nil {
				t.Fatal(err)
			}
			added, err := os.ReadFile(provider)
			if err != nil {
				t.Fatal(err)
			}
			assertExactlyOneTrailingLF(t, string(added))
			if !strings.Contains(string(added), "\tNewOwnerModel,\n") {
				t.Fatalf("provider entry missing after add: %q", added)
			}
			if err := RemoveProvider("internal/admin/model/provider_eof_test", "OwnerModel"); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(provider)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != canonical {
				t.Fatalf("first remove did not normalize provider: got %q, want %q", got, canonical)
			}
			for cycle := 0; cycle < 3; cycle++ {
				if err := writeProvider("internal/admin/model/provider_eof_test", "OwnerModel"); err != nil {
					t.Fatal(err)
				}
				if err := RemoveProvider("internal/admin/model/provider_eof_test", "OwnerModel"); err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(provider)
				if err != nil {
					t.Fatal(err)
				}
				assertExactlyOneTrailingLF(t, string(got))
				if string(got) != canonical {
					t.Fatalf("cycle %d changed provider EOF/content: got %q, want %q", cycle, got, canonical)
				}
			}
		})
	}
}

func TestRegistrarProviderWriteRoundTrip(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "internal", "admin", "handler", "registrar_provider_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	provider := filepath.Join(dir, "provider.go")
	original := "package registrar_provider_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewExistingHandler,\n)\n"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provider, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeProvider("internal/admin/handler/registrar_provider_test", "OwnerRegistrar"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveProvider("internal/admin/handler/registrar_provider_test", "OwnerRegistrar"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("registrar provider round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, original)
	}
}

func TestRemoveAssociatedModelProviderEntries(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "internal", "admin", "model", "assoc_provider_test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	provider := filepath.Join(dir, "provider.go")
	content := "package assoc_provider_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewChildModel,\n\tNewAssocModel,\n)\n"
	if err := os.WriteFile(provider, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	fields := []crudmodel.Field{{Form: crudmodel.FormAttr{RemoteTable: "assoc", RemoteModel: "internal/admin/model/assoc_provider_test/Assoc.go", RelationFields: "name"}}}
	manifest := FileManifest{
		Generated: []string{filepath.Join(utils.RootPath(), "internal", "admin", "model", "assoc_provider_test", "Assoc.go")},
		Shared:    []string{provider},
	}
	if err := removeAssociatedModelProviders(fields, manifest, true); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "NewAssocModel") || !strings.Contains(string(updated), "NewChildModel") {
		t.Fatalf("associated provider entry removal incorrect: %s", updated)
	}
}

// 关联既有核心模型(如 ba_admin)时,不得从共享 provider.go 中移除其注册,
// 否则 wire 将因缺少核心模型 provider 而失败。
func TestRemoveAssociatedModelProvidersKeepsCoreModel(t *testing.T) {
	provider := filepath.Join(utils.RootPath(), "internal", "admin", "repository", "provider.go")
	before, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	fields := []crudmodel.Field{{Form: crudmodel.FormAttr{RemoteTable: "ba_admin", RemoteModel: "internal/admin/model/admin.go", RelationFields: "username"}}}
	manifest := FileManifest{
		Generated: []string{filepath.Join(utils.RootPath(), "internal", "admin", "model", "test.go")},
		Shared:    []string{provider},
	}
	if err := removeAssociatedModelProviders(fields, manifest, true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("core model provider.go must remain untouched")
	}
}

func TestRewriteFlexNumericParamFields(t *testing.T) {
	input := "type DemoParam struct {\n\tEnabled bool `json:\"enabled\"`\n\tCount int32 `json:\"count\"`\n\tTotal int64 `json:\"total\"`\n\tRate float64 `json:\"rate\"`\n\tName string `json:\"name\"`\n}\n"
	want := "type DemoParam struct {\n\tEnabled validate.FlexBool `json:\"enabled\"`\n\tCount validate.FlexInt32 `json:\"count\"`\n\tTotal validate.FlexInt64 `json:\"total\"`\n\tRate validate.FlexFloat64 `json:\"rate\"`\n\tName string `json:\"name\"`\n}\n"
	if got := rewriteFlexNumericParamFields(input, nil); got != want {
		t.Fatalf("unexpected rewritten fields:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestRewriteHandlerParamFieldsUsesDesignTypeOverrides(t *testing.T) {
	input := "type DemoParam struct {\n" +
		"\tCount int32 `json:\"count\"`\n" +
		"\tCheckbox []string `json:\"feature_flags\"`\n" +
		"\tSelects []string `json:\"category_values\"`\n" +
		"\tRemoteSelects []string `json:\"reviewer_ids\"`\n" +
		"\tCity string `json:\"region_city\"`\n" +
		"\tImages []string `json:\"gallery_images\"`\n" +
		"\tFiles []string `json:\"attachments_files\"`\n" +
		"\tExtra []string `json:\"extra_data\"`\n" +
		"\tScheduled string `json:\"scheduled_at\"`\n" +
		"\tPublished string `json:\"published_on\"`\n" +
		"\tAt string `json:\"published_at\"`\n" +
		"\tCreated int64 `json:\"created_at\"`\n" +
		"\tTitle string `json:\"title_string\"`\n" +
		"\tPlain string `json:\"plain_text\"`\n" +
		"}\n"
	overrides := map[string]string{
		"count":             "validate.CustomInt",
		"feature_flags":     "validate.CommaJoined",
		"category_values":   "validate.CommaJoined",
		"reviewer_ids":      "validate.CommaJoined",
		"region_city":       "validate.CommaJoined",
		"gallery_images":    "validate.CommaJoined",
		"attachments_files": "validate.CommaJoined",
		"extra_data":        "validate.KeyValueArray",
		"scheduled_at":      "validate.FlexDateTime",
		"published_on":      "validate.FlexDate",
		"published_at":      "validate.FlexClock",
		"created_at":        "validate.FlexUnixTime",
		"title_string":      "validate.CustomString",
	}
	wantTypes := map[string]string{
		"Count":         "validate.CustomInt",
		"Checkbox":      "validate.CommaJoined",
		"Selects":       "validate.CommaJoined",
		"RemoteSelects": "validate.CommaJoined",
		"City":          "validate.CommaJoined",
		"Images":        "validate.CommaJoined",
		"Files":         "validate.CommaJoined",
		"Extra":         "validate.KeyValueArray",
		"Scheduled":     "validate.FlexDateTime",
		"Published":     "validate.FlexDate",
		"At":            "validate.FlexClock",
		"Created":       "validate.FlexUnixTime",
		"Title":         "validate.CustomString",
	}
	got := rewriteFlexNumericParamFields(input, overrides)
	for field, typeName := range wantTypes {
		if !strings.Contains(got, field+" "+typeName+" `json:") {
			t.Errorf("%s was not rewritten to %s:\n%s", field, typeName, got)
		}
	}
	if strings.Contains(got, "title_string string") {
		t.Fatal("override matching must use the JSON name, not the Go field type")
	}
	if !strings.Contains(got, "Plain string `json:\"plain_text\"`") {
		t.Fatalf("ordinary string field was changed:\n%s", got)
	}
}

func TestHandlerParamTypeOverridesFromAnalysedFields(t *testing.T) {
	cases := []struct {
		name       string
		designType string
		dataType   string
		typeName   string
		length     int
		defaultTyp string
		defaultVal string
		want       string
	}{
		{name: "switch tinyint length", designType: "switch", typeName: "tinyint", length: 1, want: "validate.FlexBool"},
		{name: "radio tinyint data type", designType: "radio", typeName: "tinyint", dataType: "tinyint(1)", want: "validate.FlexBool"},
		{name: "year design type", designType: "year", typeName: "year", dataType: "year", want: "validate.FlexYear"},
		{name: "tinyint input default remains numeric", designType: "number", typeName: "tinyint", defaultTyp: "INPUT", defaultVal: "1", want: ""},
		{name: "char one is not bool", designType: "radio", typeName: "char", dataType: "char(1)", length: 1, want: ""},
		{name: "checkbox", designType: "checkbox", dataType: "set", want: "validate.CommaJoined"},
		{name: "selects", designType: "selects", dataType: "set", want: "validate.CommaJoined"},
		{name: "remoteSelects", designType: "remoteSelects", dataType: "varchar", want: "validate.CommaJoined"},
		{name: "city", designType: "city", dataType: "varchar", want: "validate.CommaJoined"},
		{name: "images", designType: "images", dataType: "text", want: "validate.CommaJoined"},
		{name: "files", designType: "files", dataType: "text", want: "validate.CommaJoined"},
		{name: "array", designType: "array", dataType: "text", want: "validate.KeyValueArray"},
		{name: "datetime", designType: "datetime", dataType: "datetime", want: "validate.FlexDateTime"},
		{name: "date", designType: "date", dataType: "date", want: "validate.FlexDate"},
		{name: "time", designType: "time", dataType: "time", want: "validate.FlexClock"},
		{name: "create_time", designType: "timestamp", dataType: "bigint", want: ""},
		{name: "end_time", designType: "timestamp", dataType: "bigint", want: "validate.FlexFormattedUnixTime"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field := analyseField(crudmodel.Field{Name: tc.name, DesignType: tc.designType, Type: tc.typeName, DataType: tc.dataType, Length: tc.length, DefaultType: tc.defaultTyp, Default: tc.defaultVal})
			got := buildHandlerParamTypeOverrides([]crudmodel.Field{field})
			if got[tc.name] != tc.want {
				t.Fatalf("override = %q, want %q (analysed field: %+v)", got[tc.name], tc.want, field)
			}
		})
	}
}

func TestModelFieldTypeOverridesMatchStorageContracts(t *testing.T) {
	fields := []crudmodel.Field{
		{Name: "enabled", Type: "tinyint", Length: 1, DesignType: "switch"},
		{Name: "visible", Type: "tinyint", DataType: "tinyint(1)", DesignType: "radio"},
		{Name: "published_year", Type: "year", DataType: "year", DesignType: "year"},
		{Name: "char_flag", Type: "char", Length: 1, DesignType: "radio"},
		{Name: "flags", DataType: "set", DesignType: "checkbox"},
		{Name: "options", DataType: "text", DesignType: "array"},
		{Name: "created_at", DataType: "datetime", DesignType: "datetime"},
		{Name: "published_at", DataType: "timestamp", DesignType: "datetime"},
		{Name: "day", DataType: "date", DesignType: "date"},
		{Name: "clock", DataType: "time", DesignType: "time"},
		{Name: "clock_native", DataType: "TIME", DesignType: "string"},
		{Name: "create_time", DataType: "bigint", DesignType: "timestamp"},
		{Name: "unix_at", DataType: "bigint", DesignType: "timestamp"},
		{Name: "mismatch", DataType: "datetime", DesignType: "string"},
	}
	got := buildModelFieldTypeOverrides(fields)
	want := map[string]string{
		"enabled":        "validate.FlexBool",
		"visible":        "validate.FlexBool",
		"published_year": "validate.FlexYear",
		"flags":          "validate.CommaJoined",
		"options":        "validate.KeyValueArray",
		"created_at":     "validate.FlexDateTime",
		"published_at":   "validate.FlexDateTime",
		"day":            "validate.FlexDate",
		"clock":          "validate.FlexClock",
		"clock_native":   "string",

		"unix_at": "validate.FlexFormattedUnixTime",
	}
	if len(got) != len(want) {
		t.Fatalf("overrides = %#v, want %#v", got, want)
	}
	for name, typeName := range want {
		if got[name] != typeName {
			t.Errorf("%s override = %q, want %q", name, got[name], typeName)
		}
	}
	if _, ok := got["mismatch"]; ok {
		t.Fatal("unrelated mismatched design type was overridden")
	}
	if _, ok := got["char_flag"]; ok {
		t.Fatal("char(1) must not be treated as boolean storage")
	}
}

func TestRenderHandlerSharesParamTypeForAddAndEdit(t *testing.T) {
	structContent := "type Demo struct {\n\tFeatureFlags []string `json:\"feature_flags\"`\n}\n"
	handlerData := HandlerData{
		Namespace:       "admin",
		ClassName:       "Demo",
		ModelNamespace:  "model",
		ModelImportPath: "buildadmin-go/internal/model",
		ModelName:       "Demo",
		ModelVar:        "demo",
		PkGoType:        "int32",
		PkJSONName:      "id",
		DTOQualifier:    "dto.",
		ParamTypeOverrides: map[string]string{
			"feature_flags": "validate.CommaJoined",
		},
	}
	// 参数类型改写发生在 DTO 文件（buildParamStruct + renderDTO）
	paramStruct := buildParamStruct(structContent, handlerData)
	if !strings.Contains(paramStruct, "FeatureFlags validate.CommaJoined") {
		t.Fatalf("DTO parameter rewrite missing:\n%s", paramStruct)
	}
	dtoContent, err := renderDTO(paramStruct)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(dtoContent, "validate.CommaJoined") != 1 || strings.Count(dtoContent, "FeatureFlags validate.CommaJoined") != 1 {
		t.Fatalf("DTO should contain one rewritten parameter type:\n%s", dtoContent)
	}
	// handler 只引用共享的 DTO 参数结构
	content, err := renderHandler(handlerData)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(content, "dto.DemoParam") != 2 {
		t.Fatalf("expected Add and Edit to use the shared dto parameter struct:\n%s", content)
	}
}

func TestModelTemplateAddsWeighFromIntegerPrimaryKey(t *testing.T) {
	withWeigh := `type Demo struct {
	ID int32 ` + "`json:\"id\"`" + `
	Weigh int32 ` + "`json:\"weigh\"`" + `
}

func TestModelTemplateAddsRowFactory(t *testing.T) {
	data := ModelData{
		Namespace:  "model",
		ClassName:  "Demo",
		ModelVar:   "demo",
		Pk:         "id",
		StructTemp: "type Demo struct{}",
	}
	code, err := renderModel(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "func (s *DemoRepository) NewRow() any") || !strings.Contains(code, "return &model.Demo{}") {
		t.Fatalf("row factory missing from repository template:\n%s", code)
	}
}

`
	withoutWeigh := `type Demo struct {
	ID int32 ` + "`json:\"id\"`" + `
}

`
	data := ModelData{Namespace: "model", ClassName: "Demo", ModelVar: "demo", Pk: "id", PkGoField: "ID", StructTemp: withWeigh}
	withCode, err := renderModel(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withCode, "if demo.Weigh == 0") || !strings.Contains(withCode, `Update("weigh", demo.ID)`) || !strings.Contains(withCode, "demo.Weigh = int32(demo.ID)") {
		t.Fatalf("weigh post-insert hook missing:\n%s", withCode)
	}

	data.StructTemp = withoutWeigh
	withoutCode, err := renderModel(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(withoutCode, "demo.Weigh") || strings.Contains(withoutCode, `Update("weigh", demo.ID)`) {
		t.Fatalf("weigh post-insert hook emitted without a weigh field:\n%s", withoutCode)
	}
}

func TestWriteGoFileNormalizesFormattingAndEOF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.go")
	if err := writeGoFile(path, "package provider\n\n\nvar ProviderSet = 1\n\n\n"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "package provider\n\nvar ProviderSet = 1\n" {
		t.Fatalf("normalized Go output = %q", content)
	}
	assertExactlyOneTrailingLF(t, string(content))
}

func assertExactlyOneTrailingLF(t *testing.T, content string) {
	t.Helper()
	if !strings.HasSuffix(content, "\n") || strings.HasSuffix(content, "\n\n") {
		t.Fatalf("content does not end with exactly one trailing LF: %q", content)
	}
}
