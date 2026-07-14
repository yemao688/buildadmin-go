package utils

import "testing"

func TestAccountStatusEnabled(t *testing.T) {
	if !AccountStatusEnabled("enable") {
		t.Fatal("enable must be allowed")
	}
	for _, status := range []string{"disable", "0", "1", "", "ENABLE", "enable ", "other"} {
		if AccountStatusEnabled(status) {
			t.Fatalf("%q must be rejected", status)
		}
	}
}
