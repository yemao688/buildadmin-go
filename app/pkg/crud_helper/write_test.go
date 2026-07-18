package crud_helper

import (
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
