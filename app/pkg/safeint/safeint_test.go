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

func TestMulInt32(t *testing.T) {
	cases := []struct {
		a, b int32
		want int32
		err  error
	}{
		{2, 3, 6, nil},
		{100, 100, 10000, nil},
		{math.MaxInt32, 2, 0, ErrOverflow},
		{math.MinInt32, 2, 0, ErrUnderflow},
	}
	for _, c := range cases {
		got, err := MulInt32(c.a, c.b)
		if err != c.err {
			t.Fatalf("MulInt32(%d,%d) err = %v, want %v", c.a, c.b, err, c.err)
		}
		if err == nil && got != c.want {
			t.Fatalf("MulInt32(%d,%d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseDecimalCents(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int32
	}{
		{`1.25`, 125},
		{`"1.25"`, 125},
		{`1`, 100},
		{`-0.5`, -50},
	} {
		got, err := ParseDecimalCents([]byte(tc.raw))
		if err != nil || got != tc.want {
			t.Fatalf("ParseDecimalCents(%s) = %d, %v; want %d", tc.raw, got, err, tc.want)
		}
	}
	for _, raw := range []string{`1.001`, `"1.001"`, `1e-2`} {
		if _, err := ParseDecimalCents([]byte(raw)); err == nil {
			t.Fatalf("ParseDecimalCents(%s) unexpectedly accepted", raw)
		}
	}
}
