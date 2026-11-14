package util

import "testing"

func TestNormalizeControllerAs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		controller string
		want       string
	}{
		{controller: "security.DataRecycle", want: "security/datarecycle"},
		{controller: "auth.AdminLog", want: "auth/adminlog"},
	} {
		t.Run(test.controller, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeControllerAs(test.controller); got != test.want {
				t.Fatalf("NormalizeControllerAs(%q) = %q, want %q", test.controller, got, test.want)
			}
		})
	}
}
