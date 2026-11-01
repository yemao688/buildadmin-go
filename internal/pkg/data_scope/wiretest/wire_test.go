package wiretest

import (
	"testing"

	"buildadmin-go/internal/pkg/data_scope"
)

func TestProviderGraphBindsEnforcer(t *testing.T) {
	got := Initialize()
	if got == nil || got.Enforcer == nil {
		t.Fatal("provider graph returned nil Enforcer")
	}
	enforcer, ok := got.Enforcer.(*data_scope.ClosureEnforcer)
	if !ok {
		t.Fatalf("provider graph returned %T, want *data_scope.ClosureEnforcer", got.Enforcer)
	}
	if enforcer.ClosureTable() != "ba_admin_closure" {
		t.Fatalf("generated provider configured unexpected closure table: %q", enforcer.ClosureTable())
	}
}
