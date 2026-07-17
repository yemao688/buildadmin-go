package crud_helper

import "testing"

func TestIsProtectedTable(t *testing.T) {
	for _, name := range []string{"admin", "ba_admin", "admin_closure", "ba_security_rule", "table"} {
		if !IsProtectedTable(name) {
			t.Errorf("IsProtectedTable(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"orders", "ba_orders", "administrator", "user_profile"} {
		if IsProtectedTable(name) {
			t.Errorf("IsProtectedTable(%q) = true, want false", name)
		}
	}
}

func TestProtectedTableNamesAreStable(t *testing.T) {
	want := map[string]bool{
		"admin": true, "admin_closure": true, "admin_log": true, "admin_rule": true,
		"user": true, "user_money_log": true, "user_score_log": true, "user_rule": true,
		"user_group": true, "attachment": true, "crud_log": true,
		"data_recycle_log": true, "sensitive_data_log": true, "security_rule": true,
		"table": true,
	}
	got := ProtectedTableNames()
	if len(got) != len(want) {
		t.Fatalf("protected table count = %d, want %d", len(got), len(want))
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected protected table %q", name)
		}
	}
}
