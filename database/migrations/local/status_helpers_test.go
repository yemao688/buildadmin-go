package local

import "testing"

func TestMapAccountStatuses(t *testing.T) {
	got, err := mapAccountStatuses([]string{"0", "1", "enable", "disable"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"disable", "enable", "enable", "disable"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("status %d = %q, want %q", i, got[i], want[i])
		}
	}
	if _, err := mapAccountStatuses([]string{"enable", ""}); err == nil {
		t.Fatal("empty status must be rejected")
	}
	for _, value := range []string{"ENABLE", "Disable", "enable ", "other"} {
		if _, err := mapAccountStatuses([]string{"enable", value}); err == nil {
			t.Fatalf("%q must be rejected", value)
		}
	}
}
