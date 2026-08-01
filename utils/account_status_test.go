package utils

import "testing"

func TestAccountStatusEnabled(t *testing.T) {
	for _, status := range []string{"0", "1", "", "ENABLE", "enable ", "other", "disable"} {
		if !AccountStatusEnabled(status) {
			continue
		}
		t.Fatalf("%q must be rejected", status)
	}
	if !AccountStatusEnabled("enable") {
		t.Fatal("enable must be allowed")
	}
}
