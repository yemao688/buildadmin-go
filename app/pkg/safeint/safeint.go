package safeint

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// ParseDecimalCents parses a JSON number or JSON string as yuan without using
// floating point. It accepts at most two fractional digits.
func ParseDecimalCents(raw []byte) (int32, error) {
	s := strings.TrimSpace(string(raw))
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" || strings.ContainsAny(s, "eE+") {
		return 0, errors.New("invalid money amount")
	}
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}
	if s == "" {
		return 0, errors.New("invalid money amount")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && len(parts[1]) > 2) {
		return 0, errors.New("money must have at most two decimals")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, errors.New("invalid money amount")
	}
	frac := int64(0)
	if len(parts) == 2 {
		parsedFrac, e := strconv.ParseInt(parts[1]+strings.Repeat("0", 2-len(parts[1])), 10, 64)
		if e != nil {
			return 0, errors.New("invalid money amount")
		}
		frac = parsedFrac
	}
	v := whole*100 + frac
	if neg {
		v = -v
	}
	if v > math.MaxInt32 || v < math.MinInt32 {
		return 0, ErrOverflow
	}
	return int32(v), nil
}

var (
	// ErrOverflow is returned when an operation would exceed MaxInt32.
	ErrOverflow = errors.New("safeint: integer overflow")
	// ErrUnderflow is returned when an operation would be less than MinInt32.
	ErrUnderflow = errors.New("safeint: integer underflow")
)

// AddInt32 adds two int32 values using an int64 intermediate and returns the
// result if it fits in an int32.
func AddInt32(a, b int32) (int32, error) {
	res := int64(a) + int64(b)
	if res > math.MaxInt32 {
		return 0, ErrOverflow
	}
	if res < math.MinInt32 {
		return 0, ErrUnderflow
	}
	return int32(res), nil
}

// MulInt32 multiplies two int32 values using an int64 intermediate and returns
// the result if it fits in an int32.
func MulInt32(a, b int32) (int32, error) {
	res := int64(a) * int64(b)
	if res > math.MaxInt32 {
		return 0, ErrOverflow
	}
	if res < math.MinInt32 {
		return 0, ErrUnderflow
	}
	return int32(res), nil
}
