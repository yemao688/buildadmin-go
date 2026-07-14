package handler

import (
	"testing"

	cErr "go-build-admin/app/pkg/error"
)

func TestValidateAccountStatusValue(t *testing.T) {
	for _, value := range []any{"enable", "disable"} {
		if err := validateAccountStatusValue(value); err != nil {
			t.Fatalf("%v should be accepted: %v", value, err)
		}
	}
	for _, value := range []any{"0", "1", "", "bad", 0, 1, nil} {
		err := validateAccountStatusValue(value)
		if err == nil {
			t.Fatalf("%v should be rejected", value)
		}
		if _, ok := err.(*cErr.Error); !ok {
			t.Fatalf("%v error type = %T", value, err)
		}
	}
}
