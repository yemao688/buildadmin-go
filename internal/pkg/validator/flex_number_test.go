package validator

import (
	"database/sql/driver"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestFlexBoolJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want FlexBool
	}{
		{"true", `true`, true},
		{"false", `false`, false},
		{"one", `1`, true},
		{"zero", `0`, false},
		{"string one", `"1"`, true},
		{"string zero", `"0"`, false},
		{"string true", `"TrUe"`, true},
		{"string false", `"FALSE"`, false},
		{"empty", `""`, false},
		{"null", `null`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got FlexBool
			if err := json.Unmarshal([]byte(test.data), &got); err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
	for _, test := range []struct {
		value FlexBool
		want  string
	}{{false, "0"}, {true, "1"}} {
		got, err := json.Marshal(test.value)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != test.want {
			t.Fatalf("marshal(%v) = %s, want %s", test.value, got, test.want)
		}
	}
}

func TestFlexBoolRejectsNonCanonicalValues(t *testing.T) {
	for _, data := range []string{`2`, `-1`, `1.0`, `"yes"`, `{}`, `[]`} {
		var value FlexBool
		if err := json.Unmarshal([]byte(data), &value); err == nil {
			t.Errorf("expected %s to be rejected", data)
		}
	}
}

func TestFlexBoolScanAndValue(t *testing.T) {
	for _, test := range []struct {
		input any
		want  FlexBool
	}{{nil, false}, {false, false}, {true, true}, {int64(0), false}, {int64(1), true}, {"0", false}, {"1", true}, {[]byte("false"), false}, {[]byte("true"), true}} {
		var got FlexBool
		if err := got.Scan(test.input); err != nil {
			t.Fatalf("scan(%v): %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("scan(%v) = %v, want %v", test.input, got, test.want)
		}
	}
	for _, input := range []any{int8(2), int16(2), int32(2), int64(2), uint8(2), uint16(2), uint32(2), uint64(2), "yes", []byte("2")} {
		var got FlexBool
		if err := got.Scan(input); err == nil {
			t.Errorf("expected scan(%v) to fail", input)
		}
	}
	for _, test := range []struct {
		value FlexBool
		want  driver.Value
	}{{false, int64(0)}, {true, int64(1)}} {
		got, err := test.value.Value()
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("value(%v) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestFlexYearJSON(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
		want FlexYear
	}{
		{"string", `"2026"`, FlexYear("2026")},
		{"number", `2026`, FlexYear("2026")},
		{"minimum", `"1901"`, FlexYear("1901")},
		{"maximum", `2155`, FlexYear("2155")},
		{"explicit numeric zero", `0`, FlexYear("0")},
		{"explicit string zero", `"0"`, FlexYear("0")},
		{"zero", `"0000"`, FlexYear("0")},
		{"empty", `""`, FlexYear("")},
		{"null", `null`, FlexYear("")},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got FlexYear
			if err := json.Unmarshal([]byte(test.data), &got); err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
	for _, test := range []struct {
		value FlexYear
		want  string
	}{{FlexYear("2026"), `"2026"`}, {FlexYear("0"), `"0"`}, {FlexYear(""), `null`}} {
		got, err := json.Marshal(test.value)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != test.want {
			t.Fatalf("marshal(%q) = %s, want %s", test.value, got, test.want)
		}
	}
}

func TestFlexYearRejectsInvalidValues(t *testing.T) {
	for _, data := range []string{`1899`, `1900`, `2156`, `"26"`, `"202A"`, `true`, `{}`, `[]`} {
		var value FlexYear
		if err := json.Unmarshal([]byte(data), &value); err == nil {
			t.Errorf("expected %s to be rejected", data)
		}
	}
}

func TestFlexYearScanAndValue(t *testing.T) {
	for _, test := range []struct {
		input any
		want  FlexYear
	}{
		{int64(2026), FlexYear("2026")},
		{"2026", FlexYear("2026")},
		{int64(0), FlexYear("0")},
		{[]byte("0000"), FlexYear("0")},
		{"0", FlexYear("0")},
		{"", FlexYear("")},
		{nil, FlexYear("")},
	} {
		var got FlexYear
		if err := got.Scan(test.input); err != nil {
			t.Fatalf("scan(%v): %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("scan(%v) = %q, want %q", test.input, got, test.want)
		}
	}
	for _, input := range []any{int64(2156), "1900", []byte("202A")} {
		var got FlexYear
		if err := got.Scan(input); err == nil {
			t.Errorf("expected scan(%v) to fail", input)
		}
	}
	for _, test := range []struct {
		value FlexYear
		want  driver.Value
	}{{FlexYear("2026"), "2026"}, {FlexYear("0"), "0"}, {FlexYear(""), nil}} {
		got, err := test.value.Value()
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("value(%q) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestFlexNumbers(t *testing.T) {
	tests := []struct {
		name string
		data string
		want any
	}{
		{"int32 number", `1`, FlexInt32(1)},
		{"int32 string", `"2"`, FlexInt32(2)},
		{"int32 true", `true`, FlexInt32(1)},
		{"int32 false", `false`, FlexInt32(0)},
		{"int32 empty", `""`, FlexInt32(0)},
		{"int32 null", `null`, FlexInt32(0)},
		{"int64 number", `3`, FlexInt64(3)},
		{"int64 string", `"4"`, FlexInt64(4)},
		{"int64 true", `true`, FlexInt64(1)},
		{"int64 false", `false`, FlexInt64(0)},
		{"int64 empty", `""`, FlexInt64(0)},
		{"int64 null", `null`, FlexInt64(0)},
		{"float number", `5.5`, FlexFloat64(5.5)},
		{"float string", `"6.5"`, FlexFloat64(6.5)},
		{"float true", `true`, FlexFloat64(1)},
		{"float false", `false`, FlexFloat64(0)},
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

func TestFlexInt32SliceJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []int32
	}{
		{"numbers", `[1,2,3]`, []int32{1, 2, 3}},
		{"strings", `["1","2","3"]`, []int32{1, 2, 3}},
		{"mixed", `[1,"2",3]`, []int32{1, 2, 3}},
		{"bools", `[true,false]`, []int32{1, 0}},
		{"null elements", `[1,null,""]`, []int32{1, 0, 0}},
		{"empty array", `[]`, []int32{}},
		{"null", `null`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got FlexInt32Slice
			if err := json.Unmarshal([]byte(test.data), &got); err != nil {
				t.Fatal(err)
			}
			if !slicesEqual([]int32(got), test.want) {
				t.Fatalf("got %v, want %v", []int32(got), test.want)
			}
		})
	}
}

func TestFlexInt32SliceRejectsInvalidElements(t *testing.T) {
	for _, test := range []struct {
		data string
		want string
	}{
		{`["abc"]`, "index 0"},
		{`[1,"abc",2]`, "index 1"},
		{`[1.5]`, "index 0"},
		{`["1.5"]`, "index 0"},
	} {
		var got FlexInt32Slice
		err := json.Unmarshal([]byte(test.data), &got)
		if err == nil {
			t.Fatalf("expected %s to be rejected", test.data)
		}
		if !strings.Contains(err.Error(), test.want) {
			t.Fatalf("error for %s = %v, want index info %q", test.data, err, test.want)
		}
	}
	// 非数组 JSON 拒绝
	var got FlexInt32Slice
	if err := json.Unmarshal([]byte(`{"a":1}`), &got); err == nil {
		t.Fatal("expected object to be rejected")
	}
}

func TestFlexInt32MapJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want map[int32]int
	}{
		{"string keys numeric values", `{"1":2,"3":4}`, map[int32]int{1: 2, 3: 4}},
		{"string values", `{"1":"2","3":"4"}`, map[int32]int{1: 2, 3: 4}},
		{"mixed values", `{"1":2,"3":"4"}`, map[int32]int{1: 2, 3: 4}},
		{"null values", `{"1":null,"3":""}`, map[int32]int{1: 0, 3: 0}},
		{"empty object", `{}`, map[int32]int{}},
		{"null", `null`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got FlexInt32Map
			if err := json.Unmarshal([]byte(test.data), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("got %v, want %v", map[int32]int(got), test.want)
			}
			for k, v := range test.want {
				if got[k] != v {
					t.Fatalf("got %v, want %v", map[int32]int(got), test.want)
				}
			}
		})
	}
}

func TestFlexInt32MapRejectsInvalidEntries(t *testing.T) {
	for _, test := range []struct {
		data string
		want string
	}{
		{`{"abc":1}`, `key "abc"`},
		{`{"1.5":2}`, `key "1.5"`},
		{`{"1":"abc"}`, `key "1"`},
		{`{"1":2.5}`, `key "1"`},
	} {
		var got FlexInt32Map
		err := json.Unmarshal([]byte(test.data), &got)
		if err == nil {
			t.Fatalf("expected %s to be rejected", test.data)
		}
		if !strings.Contains(err.Error(), test.want) {
			t.Fatalf("error for %s = %v, want key info %q", test.data, err, test.want)
		}
	}
	// 非对象 JSON 拒绝
	var got FlexInt32Map
	if err := json.Unmarshal([]byte(`[1,2]`), &got); err == nil {
		t.Fatal("expected array to be rejected")
	}
}

func TestFlexInt64SliceJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []int64
	}{
		{"numbers", `[1,2,3]`, []int64{1, 2, 3}},
		{"strings", `["1","2","3"]`, []int64{1, 2, 3}},
		{"mixed", `[1,"2",3]`, []int64{1, 2, 3}},
		{"bools", `[true,false]`, []int64{1, 0}},
		{"null elements", `[1,null,""]`, []int64{1, 0, 0}},
		{"large int64", `[9223372036854775807,"9223372036854775806"]`, []int64{9223372036854775807, 9223372036854775806}},
		{"empty array", `[]`, []int64{}},
		{"null", `null`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got FlexInt64Slice
			if err := json.Unmarshal([]byte(test.data), &got); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal([]int64(got), test.want) {
				t.Fatalf("got %v, want %v", []int64(got), test.want)
			}
		})
	}
}

func TestFlexInt64SliceRejectsInvalidElements(t *testing.T) {
	for _, test := range []struct {
		data string
		want string
	}{
		{`["abc"]`, "index 0"},
		{`[1,"abc",2]`, "index 1"},
		{`[1.5]`, "index 0"},
		{`["1.5"]`, "index 0"},
		{`[9223372036854775808]`, "index 0"},
	} {
		var got FlexInt64Slice
		err := json.Unmarshal([]byte(test.data), &got)
		if err == nil {
			t.Fatalf("expected %s to be rejected", test.data)
		}
		if !strings.Contains(err.Error(), test.want) {
			t.Fatalf("error for %s = %v, want index info %q", test.data, err, test.want)
		}
	}
	// 非数组 JSON 拒绝
	var got FlexInt64Slice
	if err := json.Unmarshal([]byte(`{"a":1}`), &got); err == nil {
		t.Fatal("expected object to be rejected")
	}
}

func slicesEqual(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
