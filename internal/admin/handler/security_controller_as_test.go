package handler

import "testing"

func TestNormalizeControllerAs(t *testing.T) {
	for _, test := range []struct {
		controller string
		want       string
	}{
		{controller: "security.DataRecycle", want: "security/datarecycle"},
		{controller: "auth.AdminLog", want: "auth/adminlog"},
	} {
		t.Run(test.controller, func(t *testing.T) {
			if got := normalizeControllerAs(test.controller); got != test.want {
				t.Fatalf("normalizeControllerAs(%q) = %q, want %q", test.controller, got, test.want)
			}
		})
	}
}
