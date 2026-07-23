package crud_helper

import "testing"

func TestIsProtectedTable(t *testing.T) {
	for _, name := range []string{"admin", "admin_closure", "security_rule", "table", "user"} {
		if !IsProtectedTable(name) {
			t.Errorf("IsProtectedTable(%q) = false, want true", name)
		}
	}
	// Exact logical matching only: prefixed or category-suffixed names are not
	// protected without prefix context.
	for _, name := range []string{"orders", "ba_orders", "administrator", "user_profile", "seller_user", "ba_admin", "ba_user"} {
		if IsProtectedTable(name) {
			t.Errorf("IsProtectedTable(%q) = true, want false", name)
		}
	}
}

func TestIsProtectedTableWithPrefix(t *testing.T) {
	const prefix = "ba_"
	for _, name := range []string{"user", "ba_user", "admin", "ba_admin", "ba_security_rule", "ba_table"} {
		if !IsProtectedTableWithPrefix(prefix, name) {
			t.Errorf("IsProtectedTableWithPrefix(%q, %q) = false, want true", prefix, name)
		}
	}
	for _, name := range []string{"seller_user", "ba_seller_user", "ops_config", "ba_ops_config", "user_profile", "ba_user_profile"} {
		if IsProtectedTableWithPrefix(prefix, name) {
			t.Errorf("IsProtectedTableWithPrefix(%q, %q) = true, want false", prefix, name)
		}
	}
	// An empty prefix performs no stripping, so physical names are inert.
	if IsProtectedTableWithPrefix("", "ba_user") {
		t.Errorf("IsProtectedTableWithPrefix(%q, %q) = true, want false", "", "ba_user")
	}
}

func TestProtectedTableNamesAreStable(t *testing.T) {
	want := map[string]bool{
		"admin": true, "admin_closure": true, "admin_log": true, "admin_rule": true,
		"user": true, "user_money_log": true, "user_score_log": true, "user_rule": true,
		"user_group": true, "attachment": true, "crud_log": true,
		"data_recycle_log": true, "sensitive_data_log": true, "security_rule": true,
		"table": true, "security_data_recycle": true, "security_sensitive_data": true,
		"admin_group": true, "admin_group_access": true, "config": true,
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
