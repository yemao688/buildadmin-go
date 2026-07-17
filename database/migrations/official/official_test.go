package official

import (
	"testing"

	"go-build-admin/conf"
)

func TestVersion202DashboardRuleCount(t *testing.T) {
	if err := validateDashboardRuleCount(0); err != nil {
		t.Fatalf("empty rule table must be safe for fresh seed: %v", err)
	}
	if err := validateDashboardRuleCount(1); err != nil {
		t.Fatalf("single dashboard rule must be valid: %v", err)
	}
	if err := validateDashboardRuleCount(2); err == nil {
		t.Fatal("duplicate dashboard rules must be rejected")
	}
}

func TestMenuRuleBackupName(t *testing.T) {
	config := &conf.Configuration{}
	config.Database.Prefix = "ba_"
	if got := menuRuleBackupName(config); got != "ba_menu_rule_version200_backup" {
		t.Fatalf("backup table = %q", got)
	}
}
