package safeint

import (
	"math"
	"testing"
)

func TestAddInt32(t *testing.T) {
	cases := []struct {
		a, b int32
		want int32
		err  error
	}{
		{1, 2, 3, nil},
		{math.MaxInt32 - 1, 1, math.MaxInt32, nil},
		{math.MaxInt32, 1, 0, ErrOverflow},
		{math.MinInt32 + 1, -1, math.MinInt32, nil},
		{math.MinInt32, -1, 0, ErrUnderflow},
	}
	for _, c := range cases {
		got, err := AddInt32(c.a, c.b)
		if err != c.err {
			t.Fatalf("AddInt32(%d,%d) err = %v, want %v", c.a, c.b, err, c.err)
		}
		if err == nil && got != c.want {
			t.Fatalf("AddInt32(%d,%d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
