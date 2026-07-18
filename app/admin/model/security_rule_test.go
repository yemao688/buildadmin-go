package model

import "testing"

func TestValidateRuleIdentityChange(t *testing.T) {
	if err := validateRuleIdentityChange(true, "id", "admin_id", "id", "operator_admin_id"); err == nil {
		t.Fatal("owner change with logs must be rejected")
	}
	if err := validateRuleIdentityChange(true, "id", "admin_id", "uuid", "admin_id"); err == nil {
		t.Fatal("primary key change with logs must be rejected")
	}
	if err := validateRuleIdentityChange(false, "id", "admin_id", "uuid", "operator_admin_id"); err != nil {
		t.Fatal(err)
	}
	if err := validateRuleIdentityChange(true, "", "", "id", "admin_id"); err != nil {
		t.Fatal("default id/admin_id compatibility failed: ", err)
	}
}
