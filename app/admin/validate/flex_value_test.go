package validate

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/jinzhu/copier"
)

func TestCommaJoined(t *testing.T) {
	tests := []struct {
		name string
		data string
		want CommaJoined
		fail bool
	}{
		{"scalar array preserves order", `["a",2,true]`, "a,2,true", false},
		{"string passthrough", `"a,b"`, "a,b", false},
		{"null", `null`, "", false},
		{"empty string", `""`, "", false},
		{"empty array", `[]`, "", false},
		{"null element rejected", `["a",null]`, "", true},
		{"object rejected", `{"key":"value"}`, "", true},
		{"nested array rejected", `[["a"]]`, "", true},
		{"invalid JSON rejected", `[`, "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got CommaJoined
			err := json.Unmarshal([]byte(test.data), &got)
			if test.fail {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != CommaJoined(test.want) {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestKeyValueArray(t *testing.T) {
	valid := `[ {"key":"k1","value":"v1"}, {"key":"k2","value":"v2"} ]`
	tests := []struct {
		name string
		data string
		want KeyValueArray
		fail bool
	}{
		{"object array normalizes JSON", valid, `[{"key":"k1","value":"v1"},{"key":"k2","value":"v2"}]`, false},
		{"string containing array normalizes JSON", strconv.Quote(valid), `[{"key":"k1","value":"v1"},{"key":"k2","value":"v2"}]`, false},
		{"empty string", `""`, "", false},
		{"empty array", `[]`, "[]", false},
		{"null", `null`, "", false},
		{"non object rejected", `["x"]`, "", true},
		{"missing key rejected", `[{"value":"v"}]`, "", true},
		{"non string value rejected", `[{"key":"k","value":1}]`, "", true},
		{"invalid string rejected", `"not an array"`, "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got KeyValueArray
			err := json.Unmarshal([]byte(test.data), &got)
			if test.fail {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != KeyValueArray(test.want) {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestFlexTimes(t *testing.T) {
	local := time.FixedZone("test-local", 8*60*60)
	original := time.Local
	time.Local = local
	t.Cleanup(func() { time.Local = original })

	tests := []struct {
		name string
		data string
		new  func() any
		want any
		fail bool
	}{
		{"datetime local", `"2026-07-25 12:34:56"`, func() any { return new(FlexDateTime) }, time.Date(2026, 7, 25, 12, 34, 56, 0, local), false},
		{"datetime RFC3339", `"2026-07-25T12:34:56+08:00"`, func() any { return new(FlexDateTime) }, time.Date(2026, 7, 25, 12, 34, 56, 0, local), false},
		{"date", `"2026-07-25"`, func() any { return new(FlexDate) }, time.Date(2026, 7, 25, 0, 0, 0, 0, local), false},
		{"clock", `"12:34:56"`, func() any { return new(FlexClock) }, FlexClock("12:34:56"), false},
		{"datetime null", `null`, func() any { return new(FlexDateTime) }, time.Time{}, false},
		{"date empty", `""`, func() any { return new(FlexDate) }, time.Time{}, false},
		{"clock null", `null`, func() any { return new(FlexClock) }, FlexClock(""), false},
		{"clock empty", `""`, func() any { return new(FlexClock) }, FlexClock(""), false},
		{"date rejects datetime", `"2026-07-25 12:34:56"`, func() any { return new(FlexDate) }, time.Time{}, true},
		{"clock rejects RFC3339", `"2026-07-25T12:34:56Z"`, func() any { return new(FlexClock) }, time.Time{}, true},
		{"clock rejects datetime", `"2026-07-25 12:34:56"`, func() any { return new(FlexClock) }, FlexClock(""), true},
		{"clock rejects non-string", `123456`, func() any { return new(FlexClock) }, FlexClock(""), true},
		{"clock rejects invalid hour", `"24:00:00"`, func() any { return new(FlexClock) }, FlexClock(""), true},
		{"clock rejects fractional seconds", `"12:34:56.1"`, func() any { return new(FlexClock) }, FlexClock(""), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.new()
			err := json.Unmarshal([]byte(test.data), got)
			if test.fail {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if clock, ok := got.(*FlexClock); ok {
				if *clock != test.want {
					t.Fatalf("got %q, want %q", *clock, test.want)
				}
				return
			}
			value := flexTimeValue(got)
			if !value.Equal(test.want.(time.Time)) {
				t.Fatalf("got %v, want %v", value, test.want)
			}
		})
	}
}

func flexTimeValue(value any) time.Time {
	switch value := value.(type) {
	case *FlexDateTime:
		return time.Time(*value)
	case *FlexDate:
		return time.Time(*value)
	default:
		panic("unexpected flex time type")
	}
}

func TestFlexUnixTime(t *testing.T) {
	local := time.FixedZone("test-local", 8*60*60)
	original := time.Local
	time.Local = local
	t.Cleanup(func() { time.Local = original })

	localTime := time.Date(2026, 7, 25, 12, 34, 56, 0, local)
	tests := []struct {
		name string
		data string
		want FlexUnixTime
		fail bool
	}{
		{"number", `123`, 123, false},
		{"number string", `"456"`, 456, false},
		{"local datetime", `"2026-07-25 12:34:56"`, FlexUnixTime(localTime.Unix()), false},
		{"RFC3339", `"2026-07-25T12:34:56+08:00"`, FlexUnixTime(localTime.Unix()), false},
		{"null", `null`, 0, false},
		{"empty", `""`, 0, false},
		{"bool rejected", `true`, 0, true},
		{"other format rejected", `"2026/07/25"`, 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got FlexUnixTime
			err := json.Unmarshal([]byte(test.data), &got)
			if test.fail {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %d, want %d", got, test.want)
			}
		})
	}
}

type FlexValueInput struct {
	Tags     CommaJoined   `json:"tags"`
	Options  KeyValueArray `json:"options"`
	When     FlexDateTime  `json:"when"`
	Date     FlexDate      `json:"date"`
	Clock    FlexClock     `json:"clock"`
	UnixTime FlexUnixTime  `json:"unix_time"`
}

type FlexValueModel struct {
	Tags     string
	Options  string
	When     time.Time
	Date     time.Time
	Clock    string
	UnixTime int64
}

func TestFlexValueCopierRoundTrip(t *testing.T) {
	local := time.FixedZone("test-local", 8*60*60)
	original := time.Local
	time.Local = local
	t.Cleanup(func() { time.Local = original })

	var input FlexValueInput
	if err := json.Unmarshal([]byte(`{"tags":["a",2],"options":[{"key":"k","value":"v"}],"when":"2026-07-25 12:34:56","date":"2026-07-25","clock":"12:34:56","unix_time":"123"}`), &input); err != nil {
		t.Fatal(err)
	}
	wantWhen := time.Date(2026, 7, 25, 12, 34, 56, 0, local)
	wantDate := time.Date(2026, 7, 25, 0, 0, 0, 0, local)
	wantClock := "12:34:56"

	model := FlexValueModel{}
	if err := copier.Copy(&model, &input); err != nil {
		t.Fatal(err)
	}
	if model.Tags != "a,2" || model.Options != `[{"key":"k","value":"v"}]` || model.UnixTime != 123 {
		t.Fatalf("unexpected copied scalar values: %+v", model)
	}
	assertFlexTime(t, "model.When", model.When, wantWhen)
	assertFlexTime(t, "model.Date", model.Date, wantDate)
	if model.Clock != wantClock {
		t.Fatalf("model.Clock = %q, want %q", model.Clock, wantClock)
	}

	edit := struct {
		ID int64
		FlexValueInput
	}{}
	edit.ID = 7
	edit.FlexValueInput = input
	updated := struct {
		ID       int64
		Tags     string
		Options  string
		When     time.Time
		Date     time.Time
		Clock    string
		UnixTime int64
	}{}
	if err := copier.Copy(&updated, &edit); err != nil {
		t.Fatal(err)
	}
	if updated.ID != 7 || updated.Tags != "a,2" || updated.Options != `[{"key":"k","value":"v"}]` || updated.UnixTime != 123 {
		t.Fatalf("unexpected edit copy: %+v", updated)
	}
	assertFlexTime(t, "updated.When", updated.When, wantWhen)
	assertFlexTime(t, "updated.Date", updated.Date, wantDate)
	if updated.Clock != wantClock {
		t.Fatalf("updated.Clock = %q, want %q", updated.Clock, wantClock)
	}
}

func assertFlexTime(t *testing.T, name string, got, want time.Time) {
	t.Helper()
	if !got.Equal(want) {
		t.Errorf("%s instant = %v, want %v", name, got, want)
	}
	if got.Location() != want.Location() {
		t.Errorf("%s location = %v, want %v", name, got.Location(), want.Location())
	}
	_, gotOffset := got.Zone()
	_, wantOffset := want.Zone()
	if gotOffset != wantOffset {
		t.Errorf("%s offset = %d, want %d", name, gotOffset, wantOffset)
	}
}
