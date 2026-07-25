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

func TestFlexJSONOutput(t *testing.T) {
	local := time.FixedZone("test-local", 8*60*60)
	original := time.Local
	time.Local = local
	t.Cleanup(func() { time.Local = original })

	when := FlexDateTime(time.Date(2026, 7, 25, 12, 34, 56, 0, local))
	date := FlexDate(time.Date(2026, 7, 25, 0, 0, 0, 0, local))
	unix := FlexUnixTime(time.Time(when).Unix())
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"comma empty", CommaJoined(""), `[]`},
		{"comma values", CommaJoined("a,b"), `["a","b"]`},
		{"key value empty", KeyValueArray(""), `[]`},
		{"key value values", KeyValueArray(`[{"key":"k","value":"v"}]`), `[{"key":"k","value":"v"}]`},
		{"datetime zero", FlexDateTime{}, `null`},
		{"datetime value", when, `"2026-07-25 12:34:56"`},
		{"date zero", FlexDate{}, `null`},
		{"date value", date, `"2026-07-25"`},
		{"clock empty", FlexClock(""), `""`},
		{"clock value", FlexClock("12:34:56"), `"12:34:56"`},
		{"unix zero", FlexUnixTime(0), `null`},
		{"unix value", unix, `"2026-07-25 12:34:56"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
	if _, err := json.Marshal(KeyValueArray(`{"invalid":true}`)); err == nil {
		t.Fatal("malformed non-array key/value storage must fail to marshal")
	}
	if _, err := json.Marshal(KeyValueArray("null")); err == nil {
		t.Fatal("null key/value storage must fail to marshal")
	}
}

func TestFlexSQLScanAndValue(t *testing.T) {
	local := time.FixedZone("test-local", 8*60*60)
	original := time.Local
	time.Local = local
	t.Cleanup(func() { time.Local = original })

	when := time.Date(2026, 7, 25, 12, 34, 56, 0, local)
	var dateTime FlexDateTime
	for _, input := range []any{when, "2026-07-25 12:34:56", []byte("2026-07-25 12:34:56"), nil} {
		if err := dateTime.Scan(input); err != nil {
			t.Fatalf("FlexDateTime.Scan(%T): %v", input, err)
		}
	}
	if !time.Time(dateTime).IsZero() {
		t.Fatal("nil FlexDateTime scan must reset to zero")
	}
	if err := dateTime.Scan("2026-07-25 12:34:56"); err != nil || !time.Time(dateTime).Equal(when) {
		t.Fatalf("FlexDateTime scan = %v, err %v", dateTime, err)
	}

	var date FlexDate
	if err := date.Scan([]byte("2026-07-25")); err != nil || time.Time(date).Format("2006-01-02") != "2026-07-25" {
		t.Fatalf("FlexDate scan = %v, err %v", date, err)
	}
	var clock FlexClock
	if err := clock.Scan([]byte("12:34:56")); err != nil || clock != "12:34:56" {
		t.Fatalf("FlexClock scan = %q, err %v", clock, err)
	}
	var comma CommaJoined
	if err := comma.Scan([]byte("a,b")); err != nil || comma != "a,b" {
		t.Fatalf("CommaJoined scan = %q, err %v", comma, err)
	}
	var options KeyValueArray
	if err := options.Scan(`[ {"key":"k","value":"v"} ]`); err != nil {
		t.Fatal(err)
	}
	if got, _ := json.Marshal(options); string(got) != `[{"key":"k","value":"v"}]` {
		t.Fatalf("KeyValueArray scan output = %s", got)
	}
	var unix FlexUnixTime
	if err := unix.Scan(when); err != nil || unix != FlexUnixTime(when.Unix()) {
		t.Fatalf("FlexUnixTime time scan = %d, err %v", unix, err)
	}
	if err := unix.Scan([]byte("2026-07-25 12:34:56")); err != nil || unix != FlexUnixTime(when.Unix()) {
		t.Fatalf("FlexUnixTime string scan = %d, err %v", unix, err)
	}
	if _, err := dateTime.Value(); err != nil {
		t.Fatal(err)
	}
	if value, err := date.Value(); err != nil || value == nil {
		t.Fatalf("FlexDate.Value() = %v, err %v", value, err)
	}
	if value, err := comma.Value(); err != nil || value != "a,b" {
		t.Fatalf("CommaJoined.Value() = %v, err %v", value, err)
	}
	if value, err := options.Value(); err != nil || value != `[ {"key":"k","value":"v"} ]` {
		t.Fatalf("KeyValueArray.Value() = %v, err %v", value, err)
	}
	if _, err := clock.Value(); err != nil {
		t.Fatal(err)
	}
	if value, err := (FlexClock("")).Value(); err != nil || value != "" {
		t.Fatalf("empty FlexClock.Value() = %v, err %v", value, err)
	}
	if value, err := unix.Value(); err != nil || value != int64(when.Unix()) {
		t.Fatalf("FlexUnixTime.Value() = %v, err %v", value, err)
	}
	if value, err := (FlexUnixTime(0)).Value(); err != nil || value != int64(0) {
		t.Fatalf("zero FlexUnixTime.Value() = %v, err %v", value, err)
	}
	var zero FlexDateTime
	if value, err := zero.Value(); err != nil || value != nil {
		t.Fatalf("zero FlexDateTime.Value() = %v, err %v", value, err)
	}
	if err := clock.Scan(true); err == nil {
		t.Fatal("unsupported clock scan type must fail")
	}
}

type FlexValueInput struct {
	Tags     CommaJoined   `json:"tags"`
	Options  KeyValueArray `json:"options"`
	When     FlexDateTime  `json:"when"`
	Date     FlexDate      `json:"date"`
	Clock    FlexClock     `json:"clock"`
	UnixTime FlexUnixTime  `json:"unix_time"`
	Year     FlexYear      `json:"year"`
}

type FlexValueModel struct {
	Tags     CommaJoined
	Options  KeyValueArray
	When     FlexDateTime
	Date     FlexDate
	Clock    FlexClock
	UnixTime FlexUnixTime
	Year     FlexYear
}

func TestFlexValueCopierRoundTrip(t *testing.T) {
	local := time.FixedZone("test-local", 8*60*60)
	original := time.Local
	time.Local = local
	t.Cleanup(func() { time.Local = original })

	var input FlexValueInput
	if err := json.Unmarshal([]byte(`{"tags":["a",2],"options":[{"key":"k","value":"v"}],"when":"2026-07-25 12:34:56","date":"2026-07-25","clock":"12:34:56","unix_time":"123","year":"2026"}`), &input); err != nil {
		t.Fatal(err)
	}
	wantWhen := time.Date(2026, 7, 25, 12, 34, 56, 0, local)
	wantDate := time.Date(2026, 7, 25, 0, 0, 0, 0, local)
	wantClock := FlexClock("12:34:56")

	model := FlexValueModel{}
	if err := copier.Copy(&model, &input); err != nil {
		t.Fatal(err)
	}
	if model.Tags != "a,2" || model.Options != `[{"key":"k","value":"v"}]` || model.UnixTime != 123 || model.Year != "2026" {
		t.Fatalf("unexpected copied scalar values: %+v", model)
	}
	assertFlexTime(t, "model.When", time.Time(model.When), wantWhen)
	assertFlexTime(t, "model.Date", time.Time(model.Date), wantDate)
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
		Tags     CommaJoined
		Options  KeyValueArray
		When     FlexDateTime
		Date     FlexDate
		Clock    FlexClock
		UnixTime FlexUnixTime
		Year     FlexYear
	}{}
	if err := copier.Copy(&updated, &edit); err != nil {
		t.Fatal(err)
	}
	if updated.ID != 7 || updated.Tags != "a,2" || updated.Options != `[{"key":"k","value":"v"}]` || updated.UnixTime != 123 || updated.Year != "2026" {
		t.Fatalf("unexpected edit copy: %+v", updated)
	}
	assertFlexTime(t, "updated.When", time.Time(updated.When), wantWhen)
	assertFlexTime(t, "updated.Date", time.Time(updated.Date), wantDate)
	if updated.Clock != wantClock {
		t.Fatalf("updated.Clock = %q, want %q", updated.Clock, wantClock)
	}
}

func TestFlexYearCopierPreservesAbsentAndExplicitZero(t *testing.T) {
	type input struct{ Year FlexYear }
	type output struct{ Year FlexYear }
	for _, test := range []struct {
		name string
		in   FlexYear
	}{
		{"absent", FlexYear("")},
		{"explicit zero", FlexYear("0")},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got output
			if err := copier.Copy(&got, &input{Year: test.in}); err != nil {
				t.Fatal(err)
			}
			if got.Year != test.in {
				t.Fatalf("copied year = %q, want %q", got.Year, test.in)
			}
		})
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
