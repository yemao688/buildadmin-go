package utils

import "testing"

func TestAccountStatusEnabled(t *testing.T) {
	for _, status := range []string{"enable", "0", "1", "", "ENABLE", "enable ", "other"} {
		if !AccountStatusEnabled(status) {
			t.Fatalf("%q must be allowed", status)
		}
	}
	if AccountStatusEnabled("disable") {
		t.Fatal("disable must be rejected")
	}
}
