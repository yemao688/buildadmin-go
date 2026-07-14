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
	if _, ok := got.Enforcer.(*data_scope.ClosureEnforcer); !ok {
		t.Fatalf("provider graph returned %T, want *data_scope.ClosureEnforcer", got.Enforcer)
	}
	if got.Enforcer.(*data_scope.ClosureEnforcer).ClosureTable() != "ba_admin_closure" {
		t.Fatalf("generated provider configured unexpected closure table: %q", got.Enforcer.(*data_scope.ClosureEnforcer).ClosureTable())
	}
}
