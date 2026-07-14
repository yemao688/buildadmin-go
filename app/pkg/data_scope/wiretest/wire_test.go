package wiretest

import (
	"testing"

	"go-build-admin/app/pkg/data_scope"
)

func TestProviderGraphBindsEnforcer(t *testing.T) {
	got := Initialize()
	if got == nil || got.Enforcer == nil {
		t.Fatal("provider graph returned nil Enforcer")
	}
	if _, ok := got.Enforcer.(*data_scope.DenyAllEnforcer); !ok {
		t.Fatalf("provider graph returned %T, want *data_scope.DenyAllEnforcer", got.Enforcer)
	}
}
