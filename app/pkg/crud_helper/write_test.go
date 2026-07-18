package crud_helper

import (
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
