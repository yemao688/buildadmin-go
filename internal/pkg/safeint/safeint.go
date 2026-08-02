package safeint

import (
	"errors"
	"math"
)

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
