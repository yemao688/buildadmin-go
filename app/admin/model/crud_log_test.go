package model

import (
	"encoding/json"
	"testing"
)

func TestBoolOrStringUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  BoolOrString
	}{
		{"bool true", `{"select-multi": true}`, "1"},
		{"bool false", `{"select-multi": false}`, ""},
		{"string 1", `{"select-multi": "1"}`, "1"},
		{"empty string", `{"select-multi": ""}`, ""},
		{"absent", `{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f FormAttr
			if err := json.Unmarshal([]byte(tc.input), &f); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.input, err)
			}
			if f.SelectMulti != tc.want {
				t.Fatalf("got %q, want %q", f.SelectMulti, tc.want)
			}
		})
	}
}

func TestBoolOrStringUnmarshalInvalid(t *testing.T) {
	var f FormAttr
	if err := json.Unmarshal([]byte(`{"select-multi": 123}`), &f); err == nil {
		t.Fatal("expected error for numeric select-multi")
	}
}
