package validate

import (
	"encoding/json"
	"testing"
)

func TestFlexNumbers(t *testing.T) {
	tests := []struct {
		name string
		data string
		want any
	}{
		{"int32 number", `1`, FlexInt32(1)},
		{"int32 string", `"2"`, FlexInt32(2)},
		{"int32 empty", `""`, FlexInt32(0)},
		{"int32 null", `null`, FlexInt32(0)},
		{"int64 number", `3`, FlexInt64(3)},
		{"int64 string", `"4"`, FlexInt64(4)},
		{"int64 empty", `""`, FlexInt64(0)},
		{"int64 null", `null`, FlexInt64(0)},
		{"float number", `5.5`, FlexFloat64(5.5)},
		{"float string", `"6.5"`, FlexFloat64(6.5)},
		{"float empty", `""`, FlexFloat64(0)},
		{"float null", `null`, FlexFloat64(0)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got any
			switch test.want.(type) {
			case FlexInt32:
				got = new(FlexInt32)
			case FlexInt64:
				got = new(FlexInt64)
			case FlexFloat64:
				got = new(FlexFloat64)
			}
			if err := json.Unmarshal([]byte(test.data), got); err != nil {
				t.Fatal(err)
			}
			switch want := test.want.(type) {
			case FlexInt32:
				if *got.(*FlexInt32) != want {
					t.Fatalf("got %v, want %v", *got.(*FlexInt32), want)
				}
			case FlexInt64:
				if *got.(*FlexInt64) != want {
					t.Fatalf("got %v, want %v", *got.(*FlexInt64), want)
				}
			case FlexFloat64:
				if *got.(*FlexFloat64) != want {
					t.Fatalf("got %v, want %v", *got.(*FlexFloat64), want)
				}
			}
		})
	}
}

func TestFlexNumbersRejectInvalidStrings(t *testing.T) {
	for _, target := range []any{new(FlexInt32), new(FlexInt64), new(FlexFloat64)} {
		if err := json.Unmarshal([]byte(`"not-a-number"`), target); err == nil {
			t.Fatal("expected invalid numeric string to fail")
		}
	}
}
