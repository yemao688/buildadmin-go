package crud_helper

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicCapabilitiesAreRemovedWithRouter(t *testing.T) {
	const marker = "\t} {\n\t\tmiddleware.RegisterAtomicRoute(capability)"
	original := "prefix\n" + marker + "\nsuffix\n"
	name := "aiGateDemo"
	injected := injectAtomicCapabilities(original, name, marker)
	if !strings.Contains(injected, `Route: "aiGateDemo/add"`) {
		t.Fatal("atomic capabilities were not injected")
	}
	removed := removeAtomicCapabilities(injected, name)
	if removed != original {
		t.Fatalf("router content was not restored after removal:\n%s", removed)
	}
	if removeAtomicCapabilities(original, name) != original {
		t.Fatal("removing absent capabilities must be idempotent")
	}
}

func TestProviderEntryRoundTrip(t *testing.T) {
	original := "package model\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewFooModel,\n\n\tNewBarModel,\n)"
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

func TestRouterEntryRoundTrip(t *testing.T) {
	original := "package router\n\nfunc InitRouter(\n\tcountryCurrencyHandler *admin.CountryCurrencyHandler,\n) *gin.Engine {\n\trouter := gin.New()\n\tadmin.CollectRoutes(router)\n\n\tadminRouter.GET(\"countryCurrency/index\", countryCurrencyHandler.Index)\n}\n"
	added := insertRouterEntry(original, "Test")
	if !strings.Contains(added, "testHandler *admin.TestHandler,") {
		t.Fatalf("router entry was not injected:\n%s", added)
	}
	if !strings.Contains(added, `adminRouter.POST("test/sortable", testHandler.Sortable)`) {
		t.Fatalf("sortable route was not injected:\n%s", added)
	}
	removed, err := removeRouterEntry(added, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if removed != original {
		t.Fatalf("router round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", removed, original)
	}
}

func TestWriteProviderCreatesMissingScaffold(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "app", "admin", "model", "provider_scaffold_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := writeProvider("app/admin/model/provider_scaffold_test", "OwnerModel"); err != nil {
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

func TestRemoveAssociatedModelProviderEntries(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "app", "admin", "model", "assoc_provider_test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	provider := filepath.Join(dir, "provider.go")
	content := "package assoc_provider_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewChildModel,\n\tNewAssocModel,\n)\n"
	if err := os.WriteFile(provider, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	fields := []model.Field{{Form: model.FormAttr{RemoteTable: "assoc", RemoteModel: "app/admin/model/assoc_provider_test/Assoc.go", RelationFields: "name"}}}
	manifest := FileManifest{
		Generated: []string{filepath.Join(utils.RootPath(), "app", "admin", "model", "assoc_provider_test", "assoc.go")},
		Shared:    []string{provider},
	}
	if err := removeAssociatedModelProviders(fields, manifest); err != nil {
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
	provider := filepath.Join(utils.RootPath(), "app", "admin", "model", "provider.go")
	before, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	fields := []model.Field{{Form: model.FormAttr{RemoteTable: "ba_admin", RemoteModel: "app/admin/model/admin.go", RelationFields: "username"}}}
	manifest := FileManifest{
		Generated: []string{filepath.Join(utils.RootPath(), "app", "admin", "model", "test.go")},
		Shared:    []string{provider},
	}
	if err := removeAssociatedModelProviders(fields, manifest); err != nil {
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
	input := "type DemoParam struct {\n\tCount int32 `json:\"count\"`\n\tTotal int64 `json:\"total\"`\n\tRate float64 `json:\"rate\"`\n\tName string `json:\"name\"`\n}\n"
	want := "type DemoParam struct {\n\tCount validate.FlexInt32 `json:\"count\"`\n\tTotal validate.FlexInt64 `json:\"total\"`\n\tRate validate.FlexFloat64 `json:\"rate\"`\n\tName string `json:\"name\"`\n}\n"
	if got := rewriteFlexNumericParamFields(input); got != want {
		t.Fatalf("unexpected rewritten fields:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
