package util

import (
	"errors"
	"strconv"
	"testing"
)

func TestNormalizeIDs(t *testing.T) {
	invalid := func(id int32) error { return errors.New("bad id " + strconv.Itoa(int(id))) }

	cases := []struct {
		name        string
		ids         []int32
		rejectEmpty bool
		want        []int32
		wantErr     string
	}{
		{name: "empty accepted", ids: nil, rejectEmpty: false, want: []int32{}},
		{name: "empty rejected", ids: []int32{}, rejectEmpty: true, wantErr: "invalid widget ids"},
		{name: "single id", ids: []int32{7}, want: []int32{7}},
		{name: "duplicates keep first occurrence order", ids: []int32{4, 4, 2, 4, 9, 2}, want: []int32{4, 2, 9}},
		{name: "zero id", ids: []int32{3, 0, 5}, wantErr: "bad id 0"},
		{name: "negative id", ids: []int32{-2}, wantErr: "bad id -2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeIDs(tc.ids, "widget", tc.rejectEmpty, invalid)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				if got != nil {
					t.Fatalf("ids = %v, want nil on error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("ids = nil, want non-nil slice")
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ids = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("ids = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
