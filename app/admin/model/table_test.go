package model

import "testing"

func TestStripTablePrefix(t *testing.T) {
	cases := []struct {
		table  string
		prefix string
		want   string
	}{
		{"ba_admin", "ba_", "admin"},
		{"ba_test", "ba_", "test"},
		{"BA_admin", "ba_", "admin"}, // 大小写不敏感，对齐上游 /i
		{"other_table", "ba_", "other_table"},
		{"admin", "ba_", "admin"},
		{"ba_admin", "", "ba_admin"},
		{"ba_", "ba_", ""},
	}
	for _, c := range cases {
		if got := StripTablePrefix(c.table, c.prefix); got != c.want {
			t.Errorf("StripTablePrefix(%q, %q) = %q, want %q", c.table, c.prefix, got, c.want)
		}
	}
}
